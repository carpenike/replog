import { useMemo } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'
import { PageHeader } from '@/components/PageHeader'
import { QueryState } from '@/components/QueryState'
import { useFriendlyDate } from '@/lib/dates'
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

  const { data: athlete } = useQuery({
    queryKey: ['athlete', athleteId],
    queryFn: () => api.getAthlete(athleteId),
    enabled: !isNaN(athleteId),
  })
  const friendlyDate = useFriendlyDate()

  const query = useQuery({
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

  return (
    <div>
      <PageHeader
        title={`${exercise?.name ?? 'Exercise'} History`}
        pageTitle={exercise ? `${exercise.name} History` : null}
        back={{ to: `/athletes/${athleteId}`, label: athlete?.name ?? 'Athlete' }}
        actions={exercise && (
          <Link to={`/exercises/${exId}`} className="text-sm text-primary hover:underline">About this exercise</Link>
        )}
      />

      {bests && <BestsCard bests={bests} />}

      <QueryState
        query={query}
        resource="history"
        isEmpty={h => h.days.length === 0}
        empty={{ icon: '📈', title: 'No history for this exercise yet', description: 'Logged sets show up here, newest first.' }}
      >
        {history => (
        <div className="space-y-4">
          {history.days.map(day => (
            <Card key={day.workout_id} size="sm">
              <CardHeader>
                <CardTitle>
                  <Link to={`/athletes/${athleteId}/workouts/${day.workout_id}`}
                    className="hover:text-primary">
                    {friendlyDate(day.workout_date)}
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
      </QueryState>
    </div>
  )
}
