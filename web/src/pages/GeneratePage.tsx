import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
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
          <Card className="mb-6">
            <CardContent>
            <h2 className="font-semibold mb-2">AI Coach Reasoning</h2>
            <p className="text-sm text-muted-foreground whitespace-pre-wrap">{genResult.reasoning}</p>
            <div className="flex gap-4 mt-3 text-xs text-muted-foreground">
              <span>Model: {genResult.model}</span>
              <span>{genResult.tokens_used} tokens</span>
              <span>{genResult.duration}</span>
              {genResult.truncated && <span className="text-warning">⚠ Output was truncated</span>}
            </div>
            </CardContent>
          </Card>
          <Card className="mb-6">
            <CardContent>
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
            </CardContent>
          </Card>
          <div className="flex gap-3">
            <Button variant="ghost" onClick={() => executeMutation.mutate()}
              disabled={executeMutation.isPending}
              >
              {executeMutation.isPending ? 'Saving...' : 'Save Program'}
            </Button>
            <Button variant="ghost" onClick={() => { setStep('form'); setGenResult(null); setError('') }}
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
            <Button onClick={() => window.location.href = `/athletes/${athleteId}`}>
              View Athlete
            </Button>
            <Button variant="outline" onClick={() => window.location.href = '/programs'}>
              View Programs
            </Button>
          </div>
          </CardContent>
        </Card>
      )}
    </div>
  )
}