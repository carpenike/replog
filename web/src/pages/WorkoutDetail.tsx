import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'

function formatWeight(w: number): string {
  return w === Math.floor(w) ? w.toString() : w.toFixed(1)
}

export function WorkoutDetail() {
  const { id, workoutId } = useParams<{ id: string; workoutId: string }>()
  const athleteId = Number(id)
  const wId = Number(workoutId)

  const { data, isLoading, error } = useQuery({
    queryKey: ['workout', athleteId, wId],
    queryFn: () => api.getWorkout(athleteId, wId),
    enabled: !isNaN(athleteId) && !isNaN(wId),
  })

  if (isLoading) return <p className="text-muted-foreground">Loading workout...</p>
  if (error) return <p className="text-destructive">Failed to load workout.</p>
  if (!data) return <p className="text-muted-foreground">Workout not found.</p>

  const { workout, groups } = data

  return (
    <div>
      <p className="text-sm text-muted-foreground mb-1">
        <Link to={`/athletes/${athleteId}`} className="hover:text-foreground">Athlete</Link>
        {' / '}
        <Link to={`/athletes/${athleteId}/workouts`} className="hover:text-foreground">Workouts</Link>
        {' / '}
        {workout.date}
      </p>
      <h1 className="text-2xl font-bold mb-2">{workout.date}</h1>
      {workout.program_name && (
        <p className="text-sm text-muted-foreground mb-4">{workout.program_name}</p>
      )}
      {workout.notes && (
        <div className="rounded-lg border border-border bg-card p-3 mb-4 text-sm">
          {workout.notes}
        </div>
      )}

      {groups.length === 0 ? (
        <p className="text-muted-foreground">No sets logged.</p>
      ) : (
        <div className="space-y-6">
          {groups.map(group => (
            <div key={group.exercise_id} className="rounded-lg border border-border overflow-hidden">
              <div className="bg-muted/50 px-4 py-2 border-b border-border">
                <h3 className="font-semibold">{group.exercise_name}</h3>
              </div>
              <table className="w-full">
                <thead>
                  <tr className="text-xs text-muted-foreground border-b border-border">
                    <th className="text-left px-4 py-2 w-12">Set</th>
                    <th className="text-left px-4 py-2">Reps</th>
                    <th className="text-left px-4 py-2">Weight</th>
                    <th className="text-left px-4 py-2">RPE</th>
                    <th className="text-left px-4 py-2">Notes</th>
                  </tr>
                </thead>
                <tbody>
                  {group.sets.map(set => (
                    <tr key={set.id} className="border-b border-border last:border-0 text-sm">
                      <td className="px-4 py-2 text-muted-foreground">{set.set_number}</td>
                      <td className="px-4 py-2">{set.reps_label ?? set.reps}</td>
                      <td className="px-4 py-2">{set.weight ? formatWeight(set.weight) : '—'}</td>
                      <td className="px-4 py-2">{set.rpe ?? '—'}</td>
                      <td className="px-4 py-2 text-muted-foreground">{set.notes ?? ''}</td>
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
