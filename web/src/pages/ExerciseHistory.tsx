import { useMemo, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'
import { PageHeader } from '@/components/PageHeader'
import { QueryState } from '@/components/QueryState'
import { useFriendlyDate } from '@/lib/dates'
import { fullHistoryQuery } from '@/lib/fullHistory'
import { computeBests, type ExerciseBests } from '@/lib/records'
import { trendChange, trendSeries, type TrendMetric } from '@/lib/trend'
import { cn, formatWeight, localDateISO } from '@/lib/utils'
import { TrendChart } from '@/components/TrendChart'
import { Skeleton } from '@/components/ui/skeleton'
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

type TrendRange = '3m' | '6m' | '1y' | 'all'

const RANGES: { value: TrendRange; label: string; months?: number }[] = [
  { value: '3m', label: '3M', months: 3 },
  { value: '6m', label: '6M', months: 6 },
  { value: '1y', label: '1Y', months: 12 },
  { value: 'all', label: 'All' },
]

const METRICS: { value: TrendMetric; label: string }[] = [
  { value: 'e1rm', label: 'Estimated 1RM' },
  { value: 'top', label: 'Top set' },
]

function rangeStart(range: TrendRange): string {
  const months = RANGES.find(r => r.value === range)?.months
  if (!months) return ''
  const d = new Date()
  d.setMonth(d.getMonth() - months)
  return localDateISO(d)
}

/** Segmented single-choice control (aria-pressed buttons in a labelled group). */
function Segmented<T extends string>({ label, value, options, onChange }: {
  label: string
  value: T
  options: { value: T; label: string }[]
  onChange: (v: T) => void
}) {
  return (
    <div role="group" aria-label={label} className="inline-flex rounded-md border border-border p-0.5">
      {options.map(o => (
        <button
          key={o.value}
          type="button"
          aria-pressed={value === o.value}
          onClick={() => onChange(o.value)}
          className={cn(
            'min-h-11 rounded px-2.5 text-xs font-medium transition-colors md:min-h-7',
            value === o.value ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:text-foreground',
          )}
        >
          {o.label}
        </button>
      ))}
    </div>
  )
}

/** e1RM / top-set trend for one exercise, from the read-only progress endpoint. */
function TrendCard({ athleteId, exerciseId }: { athleteId: number; exerciseId: number }) {
  const [metric, setMetric] = useState<TrendMetric>('e1rm')
  const [range, setRange] = useState<TrendRange>('all')
  const { data, isLoading } = useQuery({
    queryKey: ['progress', athleteId, exerciseId],
    queryFn: () => api.getProgress(athleteId, { exerciseId }),
    enabled: !isNaN(athleteId) && !isNaN(exerciseId),
    staleTime: 5 * 60_000,
  })
  const exercise = data?.exercises.find(e => e.exercise_id === exerciseId)
  const start = rangeStart(range)
  const points = trendSeries(exercise, metric).filter(p => !start || p.date >= start)
  const change = trendChange(points)
  const metricLabel = METRICS.find(m => m.value === metric)!.label

  if (isLoading) return <Skeleton className="mb-4 h-64 w-full" />
  // Bodyweight-only or brand-new exercises have nothing to plot.
  if (!exercise || trendSeries(exercise, 'top').length + trendSeries(exercise, 'e1rm').length === 0) return null

  return (
    <Card size="sm" className="mb-4">
      <CardHeader>
        <CardTitle>{metricLabel} trend</CardTitle>
        {change != null && (
          <p className="text-xs text-muted-foreground tabular-nums">
            {change >= 0 ? '+' : '−'}{formatWeight(Math.round(Math.abs(change)))} lb over {points.length} sessions
          </p>
        )}
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="flex flex-wrap gap-2">
          <Segmented label="Measure" value={metric} options={METRICS} onChange={setMetric} />
          <Segmented label="Time range" value={range} options={RANGES} onChange={setRange} />
        </div>
        {points.length >= 2 ? (
          <TrendChart points={points} label={metricLabel} />
        ) : (
          <p className="py-8 text-center text-sm text-muted-foreground">
            {points.length === 1 ? 'One session in this range. Log another to see a trend.' : 'No sessions in this range.'}
          </p>
        )}
        <p className="text-xs text-muted-foreground">
          Estimated 1RM uses the Epley formula on each session's best set. Warm-ups are left out. Ringed points are PRs.
        </p>
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
        actions={
          <div className="flex flex-wrap gap-3 text-sm">
            <Link to={`/athletes/${athleteId}/records`} className="text-primary hover:underline">All records</Link>
            {exercise && <Link to={`/exercises/${exId}`} className="text-primary hover:underline">About this exercise</Link>}
          </div>
        }
      />

      {bests && <BestsCard bests={bests} />}

      <TrendCard athleteId={athleteId} exerciseId={exId} />

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
