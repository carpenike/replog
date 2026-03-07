import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'

function formatWeight(w: number): string {
  return w === Math.floor(w) ? w.toString() : w.toFixed(1)
}

export function TrainingMaxesList() {
  const { id } = useParams<{ id: string }>()
  const athleteId = Number(id)

  const { data: maxes, isLoading, error } = useQuery({
    queryKey: ['training-maxes', athleteId],
    queryFn: () => api.listTrainingMaxes(athleteId),
    enabled: !isNaN(athleteId),
  })

  if (isLoading) return <p className="text-muted-foreground">Loading training maxes...</p>
  if (error) return <p className="text-destructive">Failed to load training maxes.</p>

  return (
    <div>
      <p className="text-sm text-muted-foreground mb-1">
        <Link to={`/athletes/${athleteId}`} className="hover:text-foreground">Athlete</Link>
        {' / Training Maxes'}
      </p>
      <h1 className="text-2xl font-bold mb-6">Training Maxes</h1>

      {maxes && maxes.length === 0 ? (
        <p className="text-muted-foreground">No training maxes recorded.</p>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {maxes?.map(tm => (
            <div key={tm.id} className="rounded-lg border border-border bg-card p-4">
              <p className="text-sm text-muted-foreground">{tm.exercise_name}</p>
              <p className="text-2xl font-bold mt-1">{formatWeight(tm.weight)}</p>
              <p className="text-xs text-muted-foreground mt-1">
                Effective: {tm.effective_date}
              </p>
              {tm.notes && (
                <p className="text-xs text-muted-foreground mt-1">{tm.notes}</p>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
