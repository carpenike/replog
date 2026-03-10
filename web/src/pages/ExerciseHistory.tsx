import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'
import { Spinner } from '@/components/ui'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Card, CardContent, CardHeader, CardTitle, CardAction } from '@/components/ui/card'

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
            <Card key={day.workout_id} size="sm">
              <CardHeader>
                <CardTitle>
                  <Link to={`/athletes/${athleteId}/workouts/${day.workout_id}`}
                    className="hover:text-primary">
                    {day.workout_date}
                  </Link>
                </CardTitle>
                <CardAction>
                  <span className="text-xs text-muted-foreground">{day.sets.length} set{day.sets.length !== 1 ? 's' : ''}</span>
                </CardAction>
              </CardHeader>
              <CardContent>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead className="w-12">Set</TableHead>
                      <TableHead>Reps</TableHead>
                      <TableHead>Weight</TableHead>
                      <TableHead>RPE</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {day.sets.map(set => (
                      <TableRow key={`${set.workout_id}-${set.set_number}`}>
                        <TableCell className="text-muted-foreground">{set.set_number}</TableCell>
                        <TableCell>{set.reps}</TableCell>
                        <TableCell>{set.weight ? formatWeight(set.weight) : '—'}</TableCell>
                        <TableCell>{set.rpe ?? '—'}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  )
}
