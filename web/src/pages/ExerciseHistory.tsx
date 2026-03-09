import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'
import { Spinner } from '@/components/ui'

function formatWeight(w: number): string {
  return w === Math.floor(w) ? w.toString() : w.toFixed(1)
}

export function ExerciseHistory() {
  const { id, exerciseId } = useParams<{ id: string; exerciseId: string }>()
  const athleteId = Number(id)
  const exId = Number(exerciseId)

  const { data: exercise } = useQuery({
    queryKey: ['exercise', exId],
    queryFn: () => api.getExercise(exId),
    enabled: !isNaN(exId),
  })

  const { data: history, isLoading, error } = useQuery({
    queryKey: ['exercise-history', athleteId, exId],
    queryFn: () => api.listExerciseHistory(athleteId, exId),
    enabled: !isNaN(athleteId) && !isNaN(exId),
  })

  if (isLoading) return <Spinner />
  if (error) return <p className="text-destructive">Failed to load history.</p>

  return (
    <div>
      <p className="text-sm text-muted-foreground mb-1">
        <Link to={`/athletes/${athleteId}`} className="hover:text-foreground">Athlete</Link>
        {' / '}
        <Link to={`/exercises/${exId}`} className="hover:text-foreground">{exercise?.name ?? 'Exercise'}</Link>
        {' / History'}
      </p>
      <h1 className="text-2xl font-bold mb-6">{exercise?.name ?? 'Exercise'} History</h1>

      {history && history.days.length === 0 ? (
        <p className="text-muted-foreground">No history for this exercise.</p>
      ) : (
        <div className="space-y-4">
          {history?.days.map(day => (
            <div key={day.workout_id} className="rounded-lg border border-border overflow-hidden table-scroll">
              <div className="bg-muted/50 px-4 py-2 border-b border-border flex items-center justify-between">
                <Link to={`/athletes/${athleteId}/workouts/${day.workout_id}`}
                  className="text-sm font-medium hover:text-primary">
                  {day.workout_date}
                </Link>
                <span className="text-xs text-muted-foreground">{day.sets.length} set{day.sets.length !== 1 ? 's' : ''}</span>
              </div>
              <table className="w-full">
                <thead>
                  <tr className="text-xs text-muted-foreground border-b border-border">
                    <th className="text-left px-4 py-1.5 w-12">Set</th>
                    <th className="text-left px-4 py-1.5">Reps</th>
                    <th className="text-left px-4 py-1.5">Weight</th>
                    <th className="text-left px-4 py-1.5">RPE</th>
                  </tr>
                </thead>
                <tbody>
                  {day.sets.map(set => (
                    <tr key={`${set.workout_id}-${set.set_number}`} className="border-b border-border last:border-0 text-sm">
                      <td className="px-4 py-1.5 text-muted-foreground">{set.set_number}</td>
                      <td className="px-4 py-1.5">{set.reps}</td>
                      <td className="px-4 py-1.5">{set.weight ? formatWeight(set.weight) : '—'}</td>
                      <td className="px-4 py-1.5">{set.rpe ?? '—'}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
