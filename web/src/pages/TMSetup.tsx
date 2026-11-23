import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { api } from '@/api/client'
import { PageHeader } from '@/components/PageHeader'
import { QueryState } from '@/components/QueryState'
import { EmptyState } from '@/components/ui'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent } from '@/components/ui/card'

export function TMSetup() {
  const { id } = useParams<{ id: string }>()
  const athleteId = Number(id)
  const queryClient = useQueryClient()
  const [weights, setWeights] = useState<Record<number, string>>({})
  const { data: athlete } = useQuery({
    queryKey: ['athlete', athleteId],
    queryFn: () => api.getAthlete(athleteId),
    enabled: !isNaN(athleteId),
  })
  const programsQuery = useQuery({
    queryKey: ['athlete-programs', athleteId],
    queryFn: () => api.listAthletePrograms(athleteId),
    enabled: !isNaN(athleteId),
  })
  const activeProgram = programsQuery.data?.find(p => p.active)
  const resolvedTemplateId = activeProgram?.template_id
  const missingQuery = useQuery({
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
      setWeights({})
      toast.success('Training maxes saved')
    },
  })
  const filled = Object.values(weights).filter(w => w && parseFloat(w) > 0).length

  return (
    <div>
      <PageHeader
        title="Training Max Setup"
        back={{ to: `/athletes/${athleteId}`, label: athlete?.name ?? 'Athlete' }}
        description="Set training maxes for exercises that need them before starting a program."
      />
      {programsQuery.data && !activeProgram ? (
        <EmptyState
          icon="📊"
          title="No active program"
          description="Assign a program first, then set the training maxes it needs."
          action="Manage programs"
          actionTo={`/athletes/${athleteId}/assignments`}
        />
      ) : (
        <QueryState
          query={{
            data: missingQuery.data,
            error: programsQuery.error ?? missingQuery.error,
            refetch: programsQuery.error ? programsQuery.refetch : missingQuery.refetch,
          }}
          resource="training maxes"
          empty={{ icon: '✅', title: 'All training maxes are set', description: 'Nothing missing for the active program.', action: 'View training maxes', actionTo: `/athletes/${athleteId}/training-maxes` }}
        >
          {missing => (
            <form onSubmit={e => { e.preventDefault(); saveMutation.mutate() }}>
              <div className="space-y-3 mb-6">
                {missing.map(m => (
                  <Card key={m.exercise_id} size="sm">
                    <CardContent className="flex items-center gap-4">
                      <Label htmlFor={`tm-${m.exercise_id}`} className="flex-1">{m.exercise_name}</Label>
                      <Input
                        id={`tm-${m.exercise_id}`}
                        className="w-32"
                        type="number"
                        inputMode="decimal"
                        step="0.5"
                        min={0}
                        value={weights[m.exercise_id] ?? ''}
                        onChange={e => setWeights(prev => ({ ...prev, [m.exercise_id]: e.target.value }))}
                        placeholder="Weight"
                      />
                    </CardContent>
                  </Card>
                ))}
              </div>
              <Button type="submit" disabled={saveMutation.isPending || filled === 0}>
                {saveMutation.isPending ? 'Saving...' : 'Save Training Maxes'}
              </Button>
            </form>
          )}
        </QueryState>
      )}
    </div>
  )
}
