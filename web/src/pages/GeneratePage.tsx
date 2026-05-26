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

// Backend GenerationResponse shape (see internal/api/handlers_generate.go).
// Kept inline so this page can run without waiting on `just openapi` to
// regenerate the typed client.
interface PrescribedSetPreview {
  exercise: string
  set_number: number
  reps?: number
  rep_type?: string
  percentage?: number
  absolute_weight?: number
  notes?: string
}

interface DayPreview {
  day: number
  sets: PrescribedSetPreview[]
}

interface WeekPreview {
  week: number
  days: DayPreview[]
}

interface ProgramPreview {
  name: string
  description?: string
  num_weeks: number
  num_days: number
  is_loop: boolean
  weeks: WeekPreview[]
}

interface ProgressionRulePreview {
  program: string
  exercise: string
  increment: number
}

interface GenerationPreview {
  programs: ProgramPreview[]
  progression_rules?: ProgressionRulePreview[]
}

interface Generation {
  id: number
  athlete_id: number
  status: 'pending' | 'running' | 'succeeded' | 'failed' | 'cancelled'
  reasoning?: string
  model?: string
  tokens_used?: number
  duration?: string
  truncated?: boolean
  programs?: number
  exercises?: number
  error?: string
  executed?: boolean
  preview?: GenerationPreview
  created_at: string
  started_at?: string
  completed_at?: string
}

interface GenerateFormData {
  configured: boolean
  reference_programs: { id: number; name: string }[]
  default_days: number
  default_weeks: number
  latest_generation?: Generation
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
  const [isLoop, setIsLoop] = useState(false)
  const [coachDirections, setCoachDirections] = useState('')
  const [focusAreas, setFocusAreas] = useState('')
  const [referenceIds, setReferenceIds] = useState<number[]>([])
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
    queryFn: () => fetch(`/api/athletes/${athleteId}/generate`, {
      credentials: 'include',
      headers: { Accept: 'application/json' },
    }).then(r => r.json()) as Promise<GenerateFormData>,
    enabled: !isNaN(athleteId) && step === 'form',
  })

  // Apply defaults from form data once.
  const [defaultsApplied, setDefaultsApplied] = useState(false)
  if (formData && !defaultsApplied) {
    setDefaultsApplied(true)
    setNumDays(formData.default_days.toString())
    setNumWeeks(formData.default_weeks.toString())
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
      const res = await fetch(`/api/athletes/${athleteId}/generations/${generationId}`, {
        credentials: 'include',
        headers: { Accept: 'application/json' },
      })
      if (!res.ok) throw new ApiError('failed to poll', res.status)
      const data = (await res.json()) as Generation
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
      const res = await fetch(`/api/athletes/${athleteId}/generate`, {
        method: 'POST',
        credentials: 'include',
        headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
        body: JSON.stringify({
          program_name: programName,
          num_days: parseInt(numDays),
          num_weeks: parseInt(numWeeks),
          is_loop: isLoop,
          coach_directions: coachDirections,
          focus_areas: focusAreas.split(',').map(s => s.trim()).filter(Boolean),
          reference_ids: referenceIds,
        }),
      })
      if (!res.ok) {
        const err = await res.json()
        throw new ApiError(err.error ?? 'Generation failed', res.status)
      }
      return res.json() as Promise<{ generation_id: number; status: Generation['status'] }>
    },
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
      const res = await fetch(`/api/athletes/${athleteId}/generations/${generationId}/cancel`, {
        method: 'POST',
        credentials: 'include',
        headers: { Accept: 'application/json' },
      })
      if (!res.ok) {
        const err = await res.json()
        throw new ApiError(err.error ?? 'Cancel failed', res.status)
      }
      return res.json() as Promise<Generation>
    },
    onSuccess: () => {
      setStep('form')
      setGenerationId(null)
      setGeneration(null)
    },
  })

  const executeMutation = useMutation({
    mutationFn: async () => {
      const res = await fetch(`/api/athletes/${athleteId}/generations/${generationId}/execute`, {
        method: 'POST',
        credentials: 'include',
        headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
        body: '{}',
      })
      if (!res.ok) {
        const err = await res.json()
        throw new ApiError(err.error ?? 'Execute failed', res.status)
      }
      return res.json() as Promise<ExecuteResult>
    },
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
          <div>
            <Label >Program Name *</Label>
            <Input type="text" value={programName} onChange={e => setProgramName(e.target.value)} required placeholder="e.g. Sport Performance Block 5" />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <Label >Days/Week</Label>
              <Input type="number" min={1} max={7} value={numDays} onChange={e => setNumDays(e.target.value)} />
            </div>
            <div>
              <Label >Weeks</Label>
              <Input type="number" min={1} max={52} value={numWeeks} onChange={e => setNumWeeks(e.target.value)} />
            </div>
          </div>
          <div className="flex items-center gap-2">
            <Checkbox checked={isLoop} onCheckedChange={(checked) => setIsLoop(checked)} />
            <Label>Loop (repeat week sequence)</Label>
          </div>
          <div>
            <Label >Coach Directions</Label>
            <Textarea value={coachDirections} onChange={e => setCoachDirections(e.target.value)}
              placeholder="Specific instructions for the AI coach..." />
          </div>
          <div>
            <Label >Focus Areas</Label>
            <Input type="text" value={focusAreas} onChange={e => setFocusAreas(e.target.value)} placeholder="e.g. power, conditioning, hypertrophy (comma-separated)" />
          </div>
          {formData.reference_programs.length > 0 && (
            <div>
              <Label >Reference Programs</Label>
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
            <Button variant="outline" onClick={() => navigate(`/athletes/${athleteId}`)}>
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
