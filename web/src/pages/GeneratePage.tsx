import { useState } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api, ApiError } from '@/api/client'
import { Spinner } from '@/components/ui'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Card, CardContent } from '@/components/ui/card'
import { Alert } from '@/components/ui/alert'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import type { Generation } from '@/api/generation'

// Page-local form/result shapes (the shared generation + preview shapes now
// live in @/api/generation).
interface GenerateFormData {
  configured: boolean
  reference_programs: { id: number; name: string }[]
  default_days: number
  default_weeks: number
  latest_generation?: Generation
  available_methodologies?: MethodologyOption[]
  default_methodology_id?: number | null
  suggested_program_name?: string
}

interface MethodologyOption {
  id: number
  key: string
  name: string
  audience?: string
  applicable_tiers?: string
  philosophy?: string
}

interface ExecuteResult {
  programs_created: number
  exercises_created: number
  prescribed_sets: number
  progression_rules: number
  created_template_ids: number[]
}

const TERMINAL_STATUSES: Generation['status'][] = ['succeeded', 'failed', 'cancelled']

export function GeneratePage() {
  const navigate = useNavigate()
  const { id } = useParams<{ id: string }>()
  const athleteId = Number(id)
  const queryClient = useQueryClient()
  // 'form' = entry; 'generating' = polling status; 'preview' = succeeded,
  // awaiting coach approval; 'result' = executed/imported.
  const [step, setStep] = useState<'form' | 'generating' | 'preview' | 'result'>('form')
  const [programName, setProgramName] = useState('')
  const [numDays, setNumDays] = useState('3')
  const [numWeeks, setNumWeeks] = useState('4')
  // Schedule shape (ADR 016 HOF-007): 'fixed' = a multi-week block with an
  // end date; 'loop' = one week's pattern that repeats indefinitely. The
  // two are mutually exclusive — a radio matches that semantics, and the
  // Weeks input HIDES on 'loop' (semantically inapplicable, not just
  // disabled). On submit, looping always sends num_weeks: 1 — the
  // backend also normalizes the same invariant for non-SPA clients.
  const [schedule, setSchedule] = useState<'fixed' | 'loop'>('fixed')
  const [coachDirections, setCoachDirections] = useState('')
  const [focusAreas, setFocusAreas] = useState('')
  const [referenceIds, setReferenceIds] = useState<number[]>([])
  // Methodology selection (ADR 016 Phase 3). null = unselected (adult
  // path; the backend's generic-block fallback covers the unset path).
  // Youth always lands here with a default from the form data, set by
  // the defaults block below.
  const [methodologyId, setMethodologyId] = useState<number | null>(null)
  const [showAdvancedRefs, setShowAdvancedRefs] = useState(false)
  const [generationId, setGenerationId] = useState<number | null>(null)
  const [generation, setGeneration] = useState<Generation | null>(null)
  const [execResult, setExecResult] = useState<ExecuteResult | null>(null)
  const [error, setError] = useState('')

  const { data: athlete } = useQuery({
    queryKey: ['athlete', athleteId],
    queryFn: () => api.getAthlete(athleteId),
    enabled: !isNaN(athleteId),
  })

  const { data: formData } = useQuery({
    queryKey: ['generate-form', athleteId],
    queryFn: () => api.getGenerateForm(athleteId) as Promise<GenerateFormData>,
    enabled: !isNaN(athleteId) && step === 'form',
  })

  // Apply defaults from form data once.
  const [defaultsApplied, setDefaultsApplied] = useState(false)
  if (formData && !defaultsApplied) {
    setDefaultsApplied(true)
    setNumDays(formData.default_days.toString())
    setNumWeeks(formData.default_weeks.toString())
    // Youth: pre-select the tier-mapped methodology. Adults: leave null
    // (the selector renders unselected and methodology_id is omitted
    // from the submit body, hitting the backend's generic-block fallback).
    if (formData.default_methodology_id != null) {
      setMethodologyId(formData.default_methodology_id)
    }
    // Suggested program name (HOF-007). Sticky: only set if the field is
    // currently empty (the coach hasn't typed). Same defaultsApplied
    // guard prevents re-suggesting on later renders.
    if (formData.suggested_program_name && programName === '') {
      setProgramName(formData.suggested_program_name)
    }
  }

  // Resume an in-flight or recent generation if the SPA loaded fresh.
  // Skip rows already imported — the coach is done with those. Uses the
  // same render-phase setState + useState-flag pattern as the defaults
  // block above (AGENTS.md: "no setState in useEffect for derivable values"
  // — React's official guidance, not arbitrary).
  const [resumeChecked, setResumeChecked] = useState(false)
  if (formData && !resumeChecked) {
    setResumeChecked(true)
    const latest = formData.latest_generation
    if (latest && !latest.executed) {
      setGenerationId(latest.id)
      setGeneration(latest)
      if (latest.status === 'pending' || latest.status === 'running') {
        setStep('generating')
      } else if (latest.status === 'succeeded') {
        setStep('preview')
      } else if (latest.status === 'failed') {
        setError(latest.error ?? 'Generation failed')
      }
    }
  }

  // Poll the status endpoint while the generation is in flight. TanStack
  // Query halts further polling when refetchInterval returns false.
  useQuery({
    queryKey: ['generation', athleteId, generationId],
    queryFn: async () => {
      if (generationId == null) throw new Error('no generation id')
      const data = await api.pollGeneration(athleteId, generationId) as Generation
      setGeneration(data)
      if (data.status === 'succeeded') {
        setStep('preview')
      } else if (data.status === 'failed') {
        setError(data.error ?? 'Generation failed')
        setStep('form')
      } else if (data.status === 'cancelled') {
        setError('Generation was cancelled')
        setStep('form')
      }
      return data
    },
    enabled: generationId != null && step === 'generating',
    refetchInterval: (query) => {
      const data = query.state.data
      if (!data || TERMINAL_STATUSES.includes(data.status)) return false
      return 2000
    },
  })

  const generateMutation = useMutation({
    mutationFn: async () => {
      setError('')
      // Build the request body. methodology_id is omitted when blank so
      // the backend's adult generic-block fallback path stays reachable
      // (ADR 016 D1; the field is *int64 omitempty on the Go side).
      // For looping schedules, num_weeks is always 1 — the LLM prompt
      // only emits num_weeks when !is_loop, so anything else would
      // silently desync. (Backend also normalizes the invariant for
      // non-SPA clients; see HOF-007 D3.)
      const isLoop = schedule === 'loop'
      const requestBody: Record<string, unknown> = {
        program_name: programName,
        num_days: parseInt(numDays),
        num_weeks: isLoop ? 1 : parseInt(numWeeks),
        is_loop: isLoop,
        coach_directions: coachDirections,
        focus_areas: focusAreas.split(',').map(s => s.trim()).filter(Boolean),
        reference_ids: referenceIds,
      }
      if (methodologyId != null) {
        requestBody.methodology_id = methodologyId
      }
      return api.startGeneration(athleteId, requestBody) as Promise<{ generation_id: number; status: Generation['status'] }>
    },
    meta: { skipGlobalError: true },
    onSuccess: (data) => {
      setGenerationId(data.generation_id)
      setGeneration({
        id: data.generation_id,
        athlete_id: athleteId,
        status: data.status,
        created_at: new Date().toISOString(),
      })
      setStep('generating')
      // Drop the cached form-data so resume picks up the fresh row next visit.
      queryClient.invalidateQueries({ queryKey: ['generate-form', athleteId] })
    },
    onError: (err) => {
      setError(err instanceof ApiError ? err.message : 'Generation failed')
      setStep('form')
    },
  })

  const cancelMutation = useMutation({
    mutationFn: async () => {
      if (generationId == null) throw new Error('no generation id')
      return api.cancelGeneration(athleteId, generationId) as Promise<Generation>
    },
    onSuccess: () => {
      setStep('form')
      setGenerationId(null)
      setGeneration(null)
    },
  })

  const executeMutation = useMutation({
    mutationFn: async () => {
      if (generationId == null) throw new Error('no generation id')
      return api.executeGeneration(athleteId, generationId) as Promise<ExecuteResult>
    },
    meta: { skipGlobalError: true },
    onSuccess: (data) => {
      setExecResult(data)
      setStep('result')
      queryClient.invalidateQueries({ queryKey: ['programs'] })
      queryClient.invalidateQueries({ queryKey: ['athlete-programs', athleteId] })
    },
    onError: (err) => {
      setError(err instanceof ApiError ? err.message : 'Failed to save program')
    },
  })

  return (
    <div className="max-w-2xl">
      <p className="text-sm text-muted-foreground mb-1">
        <Link to={`/athletes/${athleteId}`} className="hover:text-foreground">{athlete?.name ?? 'Athlete'}</Link>
        {' / AI Coach'}
      </p>
      <h1 className="text-2xl font-bold mb-6">Generate Program</h1>
      {formData && !formData.configured && (
        <Card className="text-center">
          <CardContent>
          <span className="text-3xl block mb-2">🤖</span>
          <p className="font-medium mb-1">AI Coach Not Configured</p>
          <p className="text-sm text-muted-foreground">Go to Admin Settings to configure an LLM provider.</p>
          </CardContent>
        </Card>
      )}
      {error && (
        <Alert variant="destructive" className="mb-4">
          {error}
          <Button variant="link" size="xs" onClick={() => setError('')}>dismiss</Button>
        </Alert>
      )}
      {/* Step 1: Form */}
      {step === 'form' && formData?.configured && (
        <form onSubmit={(e) => { e.preventDefault(); generateMutation.mutate() }} className="space-y-4">
          {/* Methodology selector (ADR 016 Phase 3). Required for youth
              (defaults to the tier-mapped methodology); optional for
              adults (renders unselected, may submit blank). */}
          {formData.available_methodologies && formData.available_methodologies.length > 0 && (
            <div>
              <Label>
                Methodology
                {formData.default_methodology_id != null && <span className="text-destructive"> *</span>}
              </Label>
              <Select
                value={methodologyId != null ? String(methodologyId) : ''}
                onValueChange={(val) => setMethodologyId(val ? parseInt(val) : null)}
              >
                <SelectTrigger className="w-full">
                  <SelectValue placeholder={formData.default_methodology_id == null ? 'Optional — pick a methodology or leave blank' : 'Select a methodology'}>
                    {(value: string | null) => {
                      if (!value) return formData.default_methodology_id == null ? 'Optional — leave blank for the default block' : 'Select a methodology'
                      const m = formData.available_methodologies?.find(opt => String(opt.id) === value)
                      return m?.name ?? value
                    }}
                  </SelectValue>
                </SelectTrigger>
                <SelectContent>
                  {formData.default_methodology_id == null && (
                    <SelectItem value="">— None (use default adult block) —</SelectItem>
                  )}
                  {formData.available_methodologies.map(m => (
                    <SelectItem key={m.id} value={String(m.id)}>{m.name}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {methodologyId != null && (() => {
                const m = formData.available_methodologies?.find(opt => opt.id === methodologyId)
                if (!m?.philosophy) return null
                return (
                  <p className="text-xs text-muted-foreground mt-1">{m.philosophy}</p>
                )
              })()}
            </div>
          )}
          <div>
            <Label htmlFor="gen-program-name">Program Name *</Label>
            <Input id="gen-program-name" type="text" value={programName} onChange={e => setProgramName(e.target.value)} required placeholder="e.g. Ryan — Block 5" />
          </div>
          <div>
            <Label htmlFor="gen-days">Days/Week</Label>
            <Input id="gen-days" type="number" inputMode="numeric" min={1} max={7} value={numDays} onChange={e => setNumDays(e.target.value)} className="w-24" />
          </div>
          <div>
            <Label>Schedule</Label>
            <RadioGroup
              value={schedule}
              onValueChange={(val: 'fixed' | 'loop' | null) => { if (val) setSchedule(val) }}
              className="mt-2"
            >
              <Label className="flex items-center gap-2 font-normal">
                <RadioGroupItem value="fixed" />
                <span>Fixed block</span>
                {schedule === 'fixed' && (
                  <span className="flex items-center gap-2 ml-2">
                    <Input
                      type="number"
                      inputMode="numeric"
                      min={1}
                      max={52}
                      value={numWeeks}
                      onChange={e => setNumWeeks(e.target.value)}
                      aria-label="Number of weeks"
                      className="w-20"
                    />
                    <span className="text-sm text-muted-foreground">weeks</span>
                  </span>
                )}
              </Label>
              <Label className="flex items-center gap-2 font-normal">
                <RadioGroupItem value="loop" />
                <span>
                  Looping
                  <span className="text-muted-foreground ml-1">— one week&apos;s pattern that repeats indefinitely</span>
                </span>
              </Label>
            </RadioGroup>
          </div>
          <div>
            <Label htmlFor="gen-directions">Coach Directions</Label>
            <Textarea id="gen-directions" value={coachDirections} onChange={e => setCoachDirections(e.target.value)}
              placeholder="Specific instructions for the AI coach..." />
          </div>
          <div>
            <Label htmlFor="gen-focus">Focus Areas</Label>
            <Input id="gen-focus" type="text" value={focusAreas} onChange={e => setFocusAreas(e.target.value)} placeholder="e.g. power, conditioning, hypertrophy (comma-separated)" />
          </div>
          {/* Advanced: override the methodology's default reference programs.
              When the coach selects entries here, they REPLACE (not add to)
              the methodology's seeded exemplars — see ADR 016 D4 (the
              backend's reference_ids overrides the methodology's exemplars). */}
          {formData.reference_programs.length > 0 && (
            <div>
              <button
                type="button"
                className="text-sm text-muted-foreground underline hover:text-foreground"
                onClick={() => setShowAdvancedRefs(v => !v)}
              >
                {showAdvancedRefs ? '▾' : '▸'} Advanced — override reference programs
              </button>
              {showAdvancedRefs && (
                <div className="mt-2">
                  <p className="text-xs text-muted-foreground mb-2">
                    Replaces the methodology's default exemplar programs for this generation.
                    Leave empty to use the methodology's exemplars.
                  </p>
                  <div className="space-y-1 max-h-40 overflow-y-auto rounded-md border border-input p-2">
                    {formData.reference_programs.map(p => (
                      <Label key={p.id}>
                        <Checkbox
                          checked={referenceIds.includes(p.id)}
                          onCheckedChange={(checked) => setReferenceIds(prev =>
                            checked ? [...prev, p.id] : prev.filter(id => id !== p.id)
                          )}
                        />
                        {p.name}
                      </Label>
                    ))}
                  </div>
                </div>
              )}
            </div>
          )}
          <Button size="lg" type="submit" disabled={generateMutation.isPending}
            >
            Generate Program
          </Button>
        </form>
      )}
      {/* Step 2: Generating (polling) */}
      {step === 'generating' && (
        <div className="text-center py-12">
          <Spinner />
          <p className="text-muted-foreground mt-4">
            {generation?.status === 'running' ? 'AI Coach is generating your program...' : 'Queued — starting shortly...'}
          </p>
          <p className="text-xs text-muted-foreground mt-1">
            Safe to close this tab — the draft will keep generating and a notification will arrive when it's ready.
          </p>
          <div className="mt-6">
            <Button variant="outline" size="sm" onClick={() => cancelMutation.mutate()}
              disabled={cancelMutation.isPending}>
              Cancel
            </Button>
          </div>
        </div>
      )}
      {/* Step 3: Preview */}
      {step === 'preview' && generation && (
        <div>
          <Alert className="mb-4">
            <strong>This is a proposal.</strong> Review the prescribed sets below. Approving saves an
            unassigned program template — you'll edit it and assign it to {athlete?.name ?? 'the athlete'}
            {' '}as separate steps. Nothing reaches the athlete until you explicitly assign.
          </Alert>
          <Card className="mb-6">
            <CardContent>
            <h2 className="font-semibold mb-2">AI Coach Reasoning</h2>
            <p className="text-sm text-muted-foreground whitespace-pre-wrap">{generation.reasoning}</p>
            <div className="flex gap-4 mt-3 text-xs text-muted-foreground">
              {generation.model && <span>Model: {generation.model}</span>}
              {generation.tokens_used != null && <span>{generation.tokens_used} tokens</span>}
              {generation.duration && <span>{generation.duration}</span>}
              {generation.truncated && <span className="text-warning">⚠ Output was truncated</span>}
            </div>
            </CardContent>
          </Card>

          {/* Set-level preview — what the coach will get if they approve. */}
          {generation.preview && generation.preview.programs.length > 0 ? (
            generation.preview.programs.map((prog, pi) => (
              <Card key={pi} className="mb-6">
                <CardContent>
                  <h2 className="font-semibold mb-1">{prog.name}</h2>
                  <p className="text-xs text-muted-foreground mb-3">
                    {prog.num_weeks} week{prog.num_weeks !== 1 ? 's' : ''} · {prog.num_days} day{prog.num_days !== 1 ? 's' : ''}/week
                    {prog.is_loop ? ' · loops' : ''}
                  </p>
                  {prog.description && (
                    <p className="text-sm text-muted-foreground mb-3 whitespace-pre-wrap">{prog.description}</p>
                  )}
                  {prog.weeks.map((wk) => (
                    <div key={wk.week} className="mb-4">
                      <h3 className="text-sm font-semibold mb-1">Week {wk.week}</h3>
                      {wk.days.map((day) => (
                        <div key={day.day} className="mb-2 pl-3 border-l-2 border-muted">
                          <p className="text-xs text-muted-foreground mb-1">Day {day.day}</p>
                          <ul className="text-sm space-y-0.5">
                            {day.sets.map((s, si) => (
                              <li key={si}>
                                <span className="font-medium">{s.exercise}</span>
                                {' — '}
                                <span className="text-muted-foreground">
                                  set {s.set_number}
                                  {s.reps != null ? `, ${s.reps} ${s.rep_type || 'reps'}` : ''}
                                  {s.percentage != null ? ` @ ${Math.round(s.percentage * 100)}% TM` : ''}
                                  {s.absolute_weight != null ? ` @ ${s.absolute_weight} lb` : ''}
                                  {s.notes ? ` (${s.notes})` : ''}
                                </span>
                              </li>
                            ))}
                          </ul>
                        </div>
                      ))}
                    </div>
                  ))}
                </CardContent>
              </Card>
            ))
          ) : (
            <Card className="mb-6">
              <CardContent>
                <h2 className="font-semibold mb-2">Generated Content</h2>
                <div className="grid grid-cols-2 gap-4 text-sm">
                  <div>
                    <p className="text-muted-foreground">Programs</p>
                    <p className="text-lg font-bold">{generation.programs ?? 0}</p>
                  </div>
                  <div>
                    <p className="text-muted-foreground">Exercises</p>
                    <p className="text-lg font-bold">{generation.exercises ?? 0}</p>
                  </div>
                </div>
              </CardContent>
            </Card>
          )}

          {generation.preview?.progression_rules && generation.preview.progression_rules.length > 0 && (
            <Card className="mb-6">
              <CardContent>
                <h2 className="font-semibold mb-2">Progression Rules</h2>
                <ul className="text-sm space-y-1">
                  {generation.preview.progression_rules.map((r, ri) => (
                    <li key={ri}>
                      <span className="font-medium">{r.exercise}</span>
                      {' — '}
                      <span className="text-muted-foreground">+{r.increment} lb per cycle</span>
                    </li>
                  ))}
                </ul>
              </CardContent>
            </Card>
          )}

          <div className="flex gap-3">
            <Button variant="ghost" onClick={() => executeMutation.mutate()}
              disabled={executeMutation.isPending}
              >
              {executeMutation.isPending ? 'Saving...' : 'Approve as Draft'}
            </Button>
            <Button variant="ghost" onClick={() => {
                setStep('form')
                setGeneration(null)
                setGenerationId(null)
                setError('')
              }}
              >
              Start Over
            </Button>
          </div>
        </div>
      )}
      {/* Step 4: Result */}
      {step === 'result' && execResult && (
        <Card className="text-center">
          <CardContent>
          <span className="text-4xl block mb-3">✅</span>
          <h2 className="text-lg font-semibold mb-2">Draft Saved</h2>
          <p className="text-sm text-muted-foreground mb-3">
            The program template was created but is <strong>not yet assigned</strong> to {athlete?.name ?? 'the athlete'}.
            Review and edit the template, then assign it from the athlete page.
          </p>
          <div className="text-sm text-muted-foreground space-y-1">
            <p>{execResult.programs_created} program template{execResult.programs_created !== 1 ? 's' : ''} created</p>
            <p>{execResult.prescribed_sets} prescribed sets</p>
            {execResult.exercises_created > 0 && (
              <p>{execResult.exercises_created} new exercise{execResult.exercises_created !== 1 ? 's' : ''}</p>
            )}
            {execResult.progression_rules > 0 && (
              <p>{execResult.progression_rules} progression rule{execResult.progression_rules !== 1 ? 's' : ''}</p>
            )}
          </div>
          <div className="mt-4 flex gap-3 justify-center flex-wrap">
            {execResult.created_template_ids?.length > 0 && (
              <Button onClick={() => navigate(`/programs/${execResult.created_template_ids[0]}/edit`)}>
                Edit Template
              </Button>
            )}
            <Button variant="outline" onClick={() => {
              const tid = execResult.created_template_ids?.[0]
              navigate(tid != null ? `/athletes/${athleteId}?assign=${tid}` : `/athletes/${athleteId}`)
            }}>
              Assign to Athlete
            </Button>
            <Button variant="ghost" onClick={() => navigate('/programs')}>
              All Programs
            </Button>
          </div>
          </CardContent>
        </Card>
      )}
    </div>
  )
}
