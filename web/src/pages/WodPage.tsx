import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api, ApiError } from '@/api/client'
import { Spinner } from '@/components/ui'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Card, CardContent } from '@/components/ui/card'
import { Alert } from '@/components/ui/alert'

// Backend GenerationResponse / preview shapes (see
// internal/api/handlers_generate.go). Kept inline so this page runs without
// waiting on `just openapi` to regenerate the typed client — same pattern
// as GeneratePage.
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

interface GenerationPreview {
  programs: ProgramPreview[]
}

interface Generation {
  id: number
  athlete_id: number
  status: 'pending' | 'running' | 'succeeded' | 'failed' | 'cancelled'
  kind?: string
  reasoning?: string
  model?: string
  tokens_used?: number
  duration?: string
  truncated?: boolean
  error?: string
  executed?: boolean
  preview?: GenerationPreview
  created_at: string
}

interface WODLogResult {
  workout_id: number
  sets_created: number
  replaced: boolean
}

const TERMINAL_STATUSES: Generation['status'][] = ['succeeded', 'failed', 'cancelled']

const today = () => new Date().toISOString().slice(0, 10)

export function WodPage() {
  const { id } = useParams<{ id: string }>()
  const athleteId = Number(id)
  const queryClient = useQueryClient()
  // 'form' = entry; 'generating' = polling; 'preview' = succeeded, awaiting
  // the log-or-discard decision; 'result' = logged.
  const [step, setStep] = useState<'form' | 'generating' | 'preview' | 'result'>('form')
  const [coachDirections, setCoachDirections] = useState('')
  const [focusAreas, setFocusAreas] = useState('')
  const [logDate, setLogDate] = useState(today())
  const [generationId, setGenerationId] = useState<number | null>(null)
  const [generation, setGeneration] = useState<Generation | null>(null)
  const [logResult, setLogResult] = useState<WODLogResult | null>(null)
  const [collision, setCollision] = useState(false)
  const [error, setError] = useState('')

  const { data: athlete } = useQuery({
    queryKey: ['athlete', athleteId],
    queryFn: () => api.getAthlete(athleteId),
    enabled: !isNaN(athleteId),
  })

  const isYouth = athlete != null && athlete.tier != null && athlete.tier !== ''

  // Poll while the WOD is in flight (reuses the generation status endpoint).
  useQuery({
    queryKey: ['wod-generation', athleteId, generationId],
    queryFn: async () => {
      if (generationId == null) throw new Error('no generation id')
      const data = await api.pollGeneration(athleteId, generationId) as Generation
      setGeneration(data)
      if (data.status === 'succeeded') {
        setStep('preview')
      } else if (data.status === 'failed') {
        setError(data.error ?? 'WOD generation failed')
        setStep('form')
      } else if (data.status === 'cancelled') {
        setError('WOD generation was cancelled')
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
      const body: Record<string, unknown> = {
        coach_directions: coachDirections,
        focus_areas: focusAreas.split(',').map(s => s.trim()).filter(Boolean),
      }
      return api.startWOD(athleteId, body) as Promise<{ generation_id: number; status: Generation['status'] }>
    },
    onSuccess: (data) => {
      setGenerationId(data.generation_id)
      setGeneration({ id: data.generation_id, athlete_id: athleteId, status: data.status, created_at: new Date().toISOString() })
      setStep('generating')
    },
    onError: (err) => {
      setError(err instanceof ApiError ? err.message : 'WOD generation failed')
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

  const logMutation = useMutation({
    mutationFn: async (replace: boolean) => {
      if (generationId == null) throw new Error('no generation id')
      setError('')
      return api.logWOD(athleteId, generationId, { date: logDate, replace }) as Promise<WODLogResult>
    },
    onSuccess: (data) => {
      setLogResult(data)
      setCollision(false)
      setStep('result')
      queryClient.invalidateQueries({ queryKey: ['workouts', athleteId] })
    },
    onError: (err) => {
      // A 409 is the same-day collision — prompt replace-or-cancel rather
      // than surfacing a raw error (HOF-015).
      if (err instanceof ApiError && err.code === 409) {
        setCollision(true)
        return
      }
      setError(err instanceof ApiError ? err.message : 'Failed to log WOD')
    },
  })

  function discard() {
    // "Discard" writes nothing — just return to the form. The generation
    // row stays unexecuted (it still feeds nothing into workout history).
    setStep('form')
    setGenerationId(null)
    setGeneration(null)
    setCollision(false)
    setError('')
  }

  return (
    <div className="max-w-2xl">
      <p className="text-sm text-muted-foreground mb-1">
        <Link to={`/athletes/${athleteId}`} className="hover:text-foreground">{athlete?.name ?? 'Athlete'}</Link>
        {' / WOD'}
      </p>
      <h1 className="text-2xl font-bold mb-6">Ad-hoc WOD</h1>

      {error && (
        <Alert variant="destructive" className="mb-4">
          {error}
          <Button variant="link" size="xs" onClick={() => setError('')}>dismiss</Button>
        </Alert>
      )}

      {isYouth && (
        <Card className="text-center">
          <CardContent>
            <span className="text-3xl block mb-2">🏋️</span>
            <p className="font-medium mb-1">Adult athletes only</p>
            <p className="text-sm text-muted-foreground">
              The ad-hoc WOD generator uses the adult Sarge-circuit style. Youth athletes follow their tier methodology.
            </p>
          </CardContent>
        </Card>
      )}

      {/* Step 1: Form */}
      {!isYouth && step === 'form' && (
        <form onSubmit={(e) => { e.preventDefault(); generateMutation.mutate() }} className="space-y-4">
          <Alert>
            Generates a single Sarge-circuit session scoped to {athlete?.name ?? 'the athlete'}'s configured equipment.
            Review it, then <strong>log it</strong> as an ad-hoc workout or <strong>discard</strong> it. Nothing is saved until you log it.
          </Alert>
          <div>
            <Label>Coach Directions</Label>
            <Textarea value={coachDirections} onChange={e => setCoachDirections(e.target.value)}
              placeholder="Optional — e.g. heavy carries, short on time, emphasize posterior chain..." />
          </div>
          <div>
            <Label>Focus Areas</Label>
            <Input type="text" value={focusAreas} onChange={e => setFocusAreas(e.target.value)}
              placeholder="e.g. conditioning, grip, core (comma-separated)" />
          </div>
          <Button size="lg" type="submit" disabled={generateMutation.isPending}>
            Generate WOD
          </Button>
        </form>
      )}

      {/* Step 2: Generating */}
      {step === 'generating' && (
        <div className="text-center py-12">
          <Spinner />
          <p className="text-muted-foreground mt-4">
            {generation?.status === 'running' ? 'Building your WOD...' : 'Queued — starting shortly...'}
          </p>
          <p className="text-xs text-muted-foreground mt-1">
            Safe to close this tab — generation continues and a notification arrives when it's ready.
          </p>
          <div className="mt-6">
            <Button variant="outline" size="sm" onClick={() => cancelMutation.mutate()} disabled={cancelMutation.isPending}>
              Cancel
            </Button>
          </div>
        </div>
      )}

      {/* Step 3: Preview + log-or-discard */}
      {step === 'preview' && generation && (
        <div>
          <Alert className="mb-4">
            <strong>This is a proposal.</strong> Review the session below, then log it as an ad-hoc workout
            you'll confirm set-by-set, or discard it. Nothing reaches the log until you click Log it.
          </Alert>

          {generation.reasoning && (
            <Card className="mb-6">
              <CardContent>
                <h2 className="font-semibold mb-2">Coach Reasoning</h2>
                <p className="text-sm text-muted-foreground whitespace-pre-wrap">{generation.reasoning}</p>
                <div className="flex gap-4 mt-3 text-xs text-muted-foreground">
                  {generation.model && <span>Model: {generation.model}</span>}
                  {generation.tokens_used != null && <span>{generation.tokens_used} tokens</span>}
                  {generation.duration && <span>{generation.duration}</span>}
                  {generation.truncated && <span className="text-warning">⚠ Output was truncated</span>}
                </div>
              </CardContent>
            </Card>
          )}

          {generation.preview && generation.preview.programs.length > 0 ? (
            generation.preview.programs.map((prog, pi) => (
              <Card key={pi} className="mb-6">
                <CardContent>
                  <h2 className="font-semibold mb-3">{prog.name}</h2>
                  {prog.weeks.map((wk) => (
                    wk.days.map((day) => (
                      <div key={`${wk.week}-${day.day}`} className="mb-2">
                        <ul className="text-sm space-y-0.5">
                          {day.sets.map((s, si) => (
                            <li key={si}>
                              <span className="font-medium">{s.exercise}</span>
                              {' — '}
                              <span className="text-muted-foreground">
                                set {s.set_number}
                                {s.reps != null ? `, ${s.reps} ${s.rep_type || 'reps'}` : ''}
                                {s.absolute_weight != null ? ` @ ${s.absolute_weight} lb` : ''}
                                {s.notes ? ` (${s.notes})` : ''}
                              </span>
                            </li>
                          ))}
                        </ul>
                      </div>
                    ))
                  ))}
                </CardContent>
              </Card>
            ))
          ) : (
            <p className="text-sm text-muted-foreground mb-6">No sets were generated. Try regenerating.</p>
          )}

          <Card className="mb-4">
            <CardContent className="space-y-3">
              <div>
                <Label htmlFor="wod-date">Log date</Label>
                <Input id="wod-date" type="date" value={logDate} onChange={e => setLogDate(e.target.value)} className="w-48" />
              </div>

              {collision ? (
                <Alert variant="destructive">
                  A resistance workout already exists for {logDate}. Replace it or cancel.
                  <div className="flex gap-2 mt-3">
                    <Button variant="destructive" size="sm" onClick={() => logMutation.mutate(true)} disabled={logMutation.isPending}>
                      Replace it
                    </Button>
                    <Button variant="outline" size="sm" onClick={() => setCollision(false)} disabled={logMutation.isPending}>
                      Cancel
                    </Button>
                  </div>
                </Alert>
              ) : (
                <div className="flex gap-3">
                  <Button onClick={() => logMutation.mutate(false)} disabled={logMutation.isPending}>
                    {logMutation.isPending ? 'Logging...' : 'Log it'}
                  </Button>
                  <Button variant="ghost" onClick={discard} disabled={logMutation.isPending}>
                    Discard
                  </Button>
                </div>
              )}
            </CardContent>
          </Card>
        </div>
      )}

      {/* Step 4: Result */}
      {step === 'result' && logResult && (
        <Card className="text-center">
          <CardContent>
            <span className="text-4xl block mb-3">✅</span>
            <h2 className="text-lg font-semibold mb-2">WOD logged</h2>
            <p className="text-sm text-muted-foreground mb-4">
              {logResult.sets_created} set{logResult.sets_created !== 1 ? 's' : ''} seeded
              {logResult.replaced ? ' (replaced the existing session)' : ''}. Confirm or edit them on the workout.
            </p>
            <div className="flex gap-3 justify-center">
              <Link to={`/athletes/${athleteId}/workouts/${logResult.workout_id}`}
                className="text-sm text-primary hover:text-primary/80">
                Open workout →
              </Link>
              <button className="text-sm text-muted-foreground hover:text-foreground" onClick={discard}>
                Generate another
              </button>
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  )
}
