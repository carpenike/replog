import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'
import { Spinner } from '@/components/ui'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Alert } from '@/components/ui/alert'
import { Card, CardContent } from '@/components/ui/card'
export function TMSetup() {
  const { id } = useParams<{ id: string }>()
  const athleteId = Number(id)
  const queryClient = useQueryClient()
  const [templateId] = useState('')
  const [weights, setWeights] = useState<Record<number, string>>({})
  const [saved, setSaved] = useState(false)
  const { data: athlete } = useQuery({
    queryKey: ['athlete', athleteId],
    queryFn: () => api.getAthlete(athleteId),
    enabled: !isNaN(athleteId),
  })
  const { data: programs } = useQuery({
    queryKey: ['athlete-programs', athleteId],
    queryFn: () => api.listAthletePrograms(athleteId),
    enabled: !isNaN(athleteId),
  })
  const activeProgram = programs?.find(p => p.active)
  const resolvedTemplateId = templateId ? parseInt(templateId) : activeProgram?.template_id
  const { data: missing, isLoading } = useQuery({
    queryKey: ['missing-tms', athleteId, resolvedTemplateId],
    queryFn: () => api.listMissingTMs(athleteId, resolvedTemplateId!),
    enabled: !isNaN(athleteId) && !!resolvedTemplateId,
  })
  const saveMutation = useMutation({
    mutationFn: () => {
      const maxes = Object.entries(weights)
        .filter(([, w]) => w && parseFloat(w) > 0)
        .map(([exId, w]) => ({ exercise_id: parseInt(exId), weight: parseFloat(w) }))
      return api.batchSetTMs(athleteId, maxes)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['missing-tms', athleteId] })
      queryClient.invalidateQueries({ queryKey: ['training-maxes', athleteId] })
      setSaved(true)
      setTimeout(() => setSaved(false), 3000)
    },
  })
  return (
    <div>
      <p className="text-sm text-muted-foreground mb-1">
        <Link to={`/athletes/${athleteId}`} className="hover:text-foreground">{athlete?.name ?? 'Athlete'}</Link>
        {' / TM Setup'}
      </p>
      <h1 className="text-2xl font-bold mb-4">Training Max Setup</h1>
      <p className="text-sm text-muted-foreground mb-6">
        Set training maxes for exercises that need them before starting a program.
      </p>
      {saved && (
        <Alert variant="success" className="mb-4">
          Training maxes saved!
        </Alert>
      )}
      {isLoading ? <Spinner /> : missing && missing.length === 0 ? (
        <Card className="text-center">
          <CardContent>
          <span className="text-3xl block mb-2">✅</span>
          <p className="text-muted-foreground">All training maxes are set! No missing TMs.</p>
          </CardContent>
        </Card>
      ) : missing && missing.length > 0 ? (
        <div>
          <div className="space-y-3 mb-6">
            {missing.map(m => (
              <Card size="sm" className="flex items-center gap-4">
                <CardContent>
                <p className="text-sm font-medium flex-1">{m.exercise_name}</p>
                <div className="w-32">
                  <Input type="number" step="0.5" min={0} value={weights[m.exercise_id] ?? ''} onChange={e => setWeights(prev => ({ ...prev, [m.exercise_id]: e.target.value }))} placeholder="Weight" />
                </div>
                </CardContent>
              </Card>
            ))}
          </div>
          <Button variant="ghost" onClick={() => saveMutation.mutate()}
            disabled={saveMutation.isPending || Object.values(weights).filter(w => w && parseFloat(w) > 0).length === 0}
            >
            {saveMutation.isPending ? 'Saving...' : 'Save Training Maxes'}
          </Button>
        </div>
      ) : (
        <p className="text-muted-foreground">Select a program to check for missing training maxes.</p>
      )}
    </div>
  )
}