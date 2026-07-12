import { useMemo } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'
import { Spinner } from '@/components/ui'
import { fullHistoryQuery } from '@/lib/fullHistory'
import { computeBests, type ExerciseBests } from '@/lib/records'
import { formatWeight } from '@/lib/utils'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Card, CardContent, CardHeader, CardTitle, CardAction } from '@/components/ui/card'

/** Compact record date ("Jun 28", with year only when not the current one). */
function recordDate(isoDate: string): string {
  const d = new Date(`${isoDate.split('T')[0]}T00:00:00`)
  const opts: Intl.DateTimeFormatOptions = { month: 'short', day: 'numeric' }
  if (d.getFullYear() !== new Date().getFullYear()) opts.year = 'numeric'
  return d.toLocaleDateString(undefined, opts)
}

function BestStat({ label, value, detail }: { label: string; value: string; detail?: string }) {
  return (
    <div className="min-w-0">
      <dt className="text-[11px] font-medium uppercase tracking-wider text-muted-foreground">{label}</dt>
      <dd className="text-lg font-bold">{value}</dd>
      {detail && <dd className="truncate text-xs text-muted-foreground">{detail}</dd>}
    </div>
  )
}

function BestsCard({ bests }: { bests: ExerciseBests }) {
  const { heaviest, bestE1rm, mostReps } = bests
  if (!heaviest && !bestE1rm && !mostReps) return null
  return (
    <Card size="sm" className="mb-4">
      <CardHeader>
        <CardTitle>Bests</CardTitle>
      </CardHeader>
      <CardContent>
        <dl className="grid grid-cols-3 gap-2">
          <BestStat
            label="Heaviest"
            value={heaviest ? `${formatWeight(heaviest.weight)} lb` : '—'}
            detail={heaviest ? `×${heaviest.reps} · ${recordDate(heaviest.date)}` : undefined}
          />
          <BestStat
            label="Best e1RM"
            value={bestE1rm ? `${Math.round(bestE1rm.e1rm)} lb` : '—'}
            detail={bestE1rm ? `${formatWeight(bestE1rm.weight)}×${bestE1rm.reps} · ${recordDate(bestE1rm.date)}` : undefined}
          />
          <BestStat
            label="Most reps"
            value={mostReps ? String(mostReps.reps) : '—'}
            detail={mostReps
              ? `${mostReps.weight != null && mostReps.weight > 0 ? `at ${formatWeight(mostReps.weight)} lb` : 'bodyweight'} · ${recordDate(mostReps.date)}`
              : undefined}
          />
        </dl>
      </CardContent>
    </Card>
  )
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

  // All-time bests need the full (all-pages) history, not just page one.
  const { data: fullDays } = useQuery({
    ...fullHistoryQuery(athleteId, exId),
    enabled: !isNaN(athleteId) && !isNaN(exId),
  })
  const bests = useMemo(
    () => fullDays
      ? computeBests(fullDays.flatMap(d => d.sets.map(s => ({ ...s, date: d.workout_date }))))
      : null,
    [fullDays],
  )

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

      {bests && <BestsCard bests={bests} />}

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
