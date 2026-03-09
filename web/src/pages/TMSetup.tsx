import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'
import { Spinner } from '@/components/ui'

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

  const activeProgram = (programs as { template_id: number; active: boolean; template_name: string }[] | undefined)?.find(p => p.active)

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
        <div className="rounded-md bg-success/10 border border-success/30 p-3 text-sm text-success mb-4">
          Training maxes saved!
        </div>
      )}

      {isLoading ? <Spinner /> : missing && missing.length === 0 ? (
        <div className="rounded-lg border border-border bg-card p-6 text-center">
          <span className="text-3xl block mb-2">✅</span>
          <p className="text-muted-foreground">All training maxes are set! No missing TMs.</p>
        </div>
      ) : missing && missing.length > 0 ? (
        <div>
          <div className="space-y-3 mb-6">
            {missing.map(m => (
              <div key={m.exercise_id} className="flex items-center gap-4 rounded-lg border border-border bg-card p-3">
                <p className="text-sm font-medium flex-1">{m.exercise_name}</p>
                <div className="w-32">
                  <input
                    type="number"
                    step="0.5"
                    min={0}
                    value={weights[m.exercise_id] ?? ''}
                    onChange={e => setWeights(prev => ({ ...prev, [m.exercise_id]: e.target.value }))}
                    placeholder="Weight"
                    className="w-full rounded-md border border-border bg-background px-3 py-1.5 text-sm"
                  />
                </div>
              </div>
            ))}
          </div>

          <button onClick={() => saveMutation.mutate()}
            disabled={saveMutation.isPending || Object.values(weights).filter(w => w && parseFloat(w) > 0).length === 0}
            className="rounded-md bg-primary px-6 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50">
            {saveMutation.isPending ? 'Saving...' : 'Save Training Maxes'}
          </button>
        </div>
      ) : (
        <p className="text-muted-foreground">Select a program to check for missing training maxes.</p>
      )}
    </div>
  )
}
