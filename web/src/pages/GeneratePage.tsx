import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api, ApiError } from '@/api/client'
import { Spinner } from '@/components/ui'

interface GenerateFormData {
  configured: boolean
  reference_programs: { id: number; name: string }[]
  default_days: number
  default_weeks: number
}

interface GenerateResult {
  reasoning: string
  model: string
  tokens_used: number
  duration: string
  truncated: boolean
  programs: number
  exercises: number
}

interface ExecuteResult {
  programs_created: number
  exercises_created: number
  prescribed_sets: number
  progression_rules: number
}

export function GeneratePage() {
  const { id } = useParams<{ id: string }>()
  const athleteId = Number(id)
  const queryClient = useQueryClient()

  const [step, setStep] = useState<'form' | 'generating' | 'preview' | 'result'>('form')
  const [programName, setProgramName] = useState('')
  const [numDays, setNumDays] = useState('3')
  const [numWeeks, setNumWeeks] = useState('4')
  const [isLoop, setIsLoop] = useState(false)
  const [coachDirections, setCoachDirections] = useState('')
  const [focusAreas, setFocusAreas] = useState('')
  const [referenceIds, setReferenceIds] = useState<number[]>([])
  const [genResult, setGenResult] = useState<GenerateResult | null>(null)
  const [execResult, setExecResult] = useState<ExecuteResult | null>(null)
  const [error, setError] = useState('')

  const { data: athlete } = useQuery({
    queryKey: ['athlete', athleteId],
    queryFn: () => api.getAthlete(athleteId),
    enabled: !isNaN(athleteId),
  })

  const { data: formData } = useQuery({
    queryKey: ['generate-form', athleteId],
    queryFn: () => fetch(`/api/athletes/${athleteId}/generate`, { credentials: 'include', headers: { Accept: 'application/json' } }).then(r => r.json()) as Promise<GenerateFormData>,
    enabled: !isNaN(athleteId) && step === 'form',
  })

  // Set defaults from form data
  if (formData && !programName) {
    setNumDays(formData.default_days.toString())
    setNumWeeks(formData.default_weeks.toString())
  }

  const generateMutation = useMutation({
    mutationFn: async () => {
      setStep('generating')
      const res = await fetch(`/api/athletes/${athleteId}/generate`, {
        method: 'POST',
        credentials: 'include',
        headers: { 'Accept': 'application/json', 'Content-Type': 'application/json' },
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
      return res.json() as Promise<GenerateResult>
    },
    onSuccess: (data) => {
      setGenResult(data)
      setStep('preview')
    },
    onError: (err) => {
      setError(err instanceof ApiError ? err.message : 'Generation failed')
      setStep('form')
    },
  })

  const executeMutation = useMutation({
    mutationFn: async () => {
      const res = await fetch(`/api/athletes/${athleteId}/generate/execute`, {
        method: 'POST',
        credentials: 'include',
        headers: { 'Accept': 'application/json', 'Content-Type': 'application/json' },
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
        <div className="rounded-lg border border-border bg-card p-6 text-center">
          <span className="text-3xl block mb-2">🤖</span>
          <p className="font-medium mb-1">AI Coach Not Configured</p>
          <p className="text-sm text-muted-foreground">Go to Admin Settings to configure an LLM provider.</p>
        </div>
      )}

      {error && (
        <div className="rounded-md bg-destructive/10 border border-destructive/30 p-3 text-sm text-destructive mb-4">
          {error}
          <button onClick={() => setError('')} className="ml-2 text-xs underline">dismiss</button>
        </div>
      )}

      {/* Step 1: Form */}
      {step === 'form' && formData?.configured && (
        <form onSubmit={(e) => { e.preventDefault(); generateMutation.mutate() }} className="space-y-4">
          <div>
            <label className="block text-sm font-medium mb-1">Program Name *</label>
            <input type="text" value={programName} onChange={e => setProgramName(e.target.value)} required
              placeholder="e.g. Sport Performance Block 5"
              className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm" />
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-sm font-medium mb-1">Days/Week</label>
              <input type="number" min={1} max={7} value={numDays} onChange={e => setNumDays(e.target.value)}
                className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm" />
            </div>
            <div>
              <label className="block text-sm font-medium mb-1">Weeks</label>
              <input type="number" min={1} max={52} value={numWeeks} onChange={e => setNumWeeks(e.target.value)}
                className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm" />
            </div>
          </div>

          <div className="flex items-center gap-2">
            <input type="checkbox" checked={isLoop} onChange={e => setIsLoop(e.target.checked)} className="rounded border-border" />
            <label className="text-sm">Loop (repeat week sequence)</label>
          </div>

          <div>
            <label className="block text-sm font-medium mb-1">Coach Directions</label>
            <textarea value={coachDirections} onChange={e => setCoachDirections(e.target.value)} rows={3}
              placeholder="Specific instructions for the AI coach..."
              className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm" />
          </div>

          <div>
            <label className="block text-sm font-medium mb-1">Focus Areas</label>
            <input type="text" value={focusAreas} onChange={e => setFocusAreas(e.target.value)}
              placeholder="e.g. power, conditioning, hypertrophy (comma-separated)"
              className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm" />
          </div>

          {formData.reference_programs.length > 0 && (
            <div>
              <label className="block text-sm font-medium mb-1">Reference Programs</label>
              <div className="space-y-1 max-h-40 overflow-y-auto rounded-md border border-border p-2">
                {formData.reference_programs.map(p => (
                  <label key={p.id} className="flex items-center gap-2 text-sm">
                    <input type="checkbox"
                      checked={referenceIds.includes(p.id)}
                      onChange={e => setReferenceIds(prev =>
                        e.target.checked ? [...prev, p.id] : prev.filter(id => id !== p.id)
                      )}
                      className="rounded border-border" />
                    {p.name}
                  </label>
                ))}
              </div>
            </div>
          )}

          <button type="submit" disabled={generateMutation.isPending}
            className="rounded-md bg-primary px-6 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50">
            Generate Program
          </button>
        </form>
      )}

      {/* Step 2: Generating */}
      {step === 'generating' && (
        <div className="text-center py-12">
          <Spinner />
          <p className="text-muted-foreground mt-4">AI Coach is generating your program...</p>
          <p className="text-xs text-muted-foreground mt-1">This may take up to a minute.</p>
        </div>
      )}

      {/* Step 3: Preview */}
      {step === 'preview' && genResult && (
        <div>
          <div className="rounded-lg border border-border bg-card p-4 mb-6">
            <h2 className="font-semibold mb-2">AI Coach Reasoning</h2>
            <p className="text-sm text-muted-foreground whitespace-pre-wrap">{genResult.reasoning}</p>
            <div className="flex gap-4 mt-3 text-xs text-muted-foreground">
              <span>Model: {genResult.model}</span>
              <span>{genResult.tokens_used} tokens</span>
              <span>{genResult.duration}</span>
              {genResult.truncated && <span className="text-warning">⚠ Output was truncated</span>}
            </div>
          </div>

          <div className="rounded-lg border border-border bg-card p-4 mb-6">
            <h2 className="font-semibold mb-2">Generated Content</h2>
            <div className="grid grid-cols-2 gap-4 text-sm">
              <div>
                <p className="text-muted-foreground">Programs</p>
                <p className="text-lg font-bold">{genResult.programs}</p>
              </div>
              <div>
                <p className="text-muted-foreground">Exercises</p>
                <p className="text-lg font-bold">{genResult.exercises}</p>
              </div>
            </div>
          </div>

          <div className="flex gap-3">
            <button onClick={() => executeMutation.mutate()}
              disabled={executeMutation.isPending}
              className="rounded-md bg-primary px-6 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50">
              {executeMutation.isPending ? 'Saving...' : 'Save Program'}
            </button>
            <button onClick={() => { setStep('form'); setGenResult(null); setError('') }}
              className="rounded-md border border-border px-4 py-2 text-sm hover:bg-accent transition-colors">
              Start Over
            </button>
          </div>
        </div>
      )}

      {/* Step 4: Result */}
      {step === 'result' && execResult && (
        <div className="rounded-lg border border-border bg-card p-6 text-center">
          <span className="text-4xl block mb-3">✅</span>
          <h2 className="text-lg font-semibold mb-2">Program Created!</h2>
          <div className="text-sm text-muted-foreground space-y-1">
            <p>{execResult.programs_created} program template{execResult.programs_created !== 1 ? 's' : ''}</p>
            <p>{execResult.prescribed_sets} prescribed sets</p>
            {execResult.exercises_created > 0 && (
              <p>{execResult.exercises_created} new exercise{execResult.exercises_created !== 1 ? 's' : ''}</p>
            )}
            {execResult.progression_rules > 0 && (
              <p>{execResult.progression_rules} progression rule{execResult.progression_rules !== 1 ? 's' : ''}</p>
            )}
          </div>
          <div className="mt-4 flex gap-3 justify-center">
            <Link to={`/athletes/${athleteId}`}
              className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90">
              View Athlete
            </Link>
            <Link to="/programs"
              className="rounded-md border border-border px-4 py-2 text-sm hover:bg-accent transition-colors">
              View Programs
            </Link>
          </div>
        </div>
      )}
    </div>
  )
}
