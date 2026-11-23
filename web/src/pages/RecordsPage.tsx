import { useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { ExerciseRecordsData, RecordSetData } from '@/api/types'
import { PageHeader } from '@/components/PageHeader'
import { QueryState } from '@/components/QueryState'
import { CardList, CardListDetail, CardListItem, DesktopOnly } from '@/components/CardList'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { useFriendlyDate } from '@/lib/dates'
import { formatWeight } from '@/lib/utils'

/** Rep-max columns shown in the desktop table; phones list every rep max. */
const TABLE_TARGETS = [1, 3, 5, 8, 10]

function repMax(ex: ExerciseRecordsData, target: number): RecordSetData | undefined {
  return ex.rep_maxes.find(r => r.target === target)
}

function setLabel(s: RecordSetData): string {
  return s.weight != null && s.weight > 0 ? `${formatWeight(s.weight)} × ${s.reps}` : `${s.reps} reps`
}

function RecordCell({ set, value, date }: { set?: RecordSetData | null; value?: string; date: (iso: string) => string }) {
  if (!set) return <span className="text-muted-foreground">—</span>
  return (
    <div>
      <div className="font-semibold tabular-nums">{value ?? setLabel(set)}</div>
      <div className="text-xs text-muted-foreground">
        {value ? `${setLabel(set)} · ` : ''}{date(set.date)}
      </div>
    </div>
  )
}

/**
 * Current personal records by lift and rep range, from the athlete's own
 * logged sets. No comparison with anyone else.
 */
export function RecordsPage() {
  const { id } = useParams<{ id: string }>()
  const athleteId = Number(id)
  const friendlyDate = useFriendlyDate()
  const date = (iso: string) => friendlyDate(iso, { absolute: true })
  const [filter, setFilter] = useState('')
  const { data: athlete } = useQuery({
    queryKey: ['athlete', athleteId],
    queryFn: () => api.getAthlete(athleteId),
    enabled: !isNaN(athleteId),
  })
  const query = useQuery({
    queryKey: ['records', athleteId],
    queryFn: () => api.getAthleteRecords(athleteId),
    enabled: !isNaN(athleteId),
  })

  return (
    <div>
      <PageHeader
        title="Personal Records"
        back={{ to: `/athletes/${athleteId}`, label: athlete?.name ?? 'Athlete' }}
        description="Best sets from your own logged history. A rep max is the heaviest load lifted for at least that many reps. Warm-ups don't count."
      />
      <QueryState
        query={query}
        resource="records"
        isEmpty={r => r.exercises.length === 0}
        empty={{ icon: '🏆', title: 'No records yet', description: 'Log some working sets and your bests show up here.', action: "Today's workout", actionTo: `/athletes/${athleteId}/prescription` }}
      >
        {records => {
          const q = filter.trim().toLowerCase()
          const rows = q ? records.exercises.filter(e => e.name.toLowerCase().includes(q)) : records.exercises
          const historyPath = (ex: ExerciseRecordsData) => `/athletes/${athleteId}/exercises/${ex.exercise_id}/history`
          return (
            <>
              {records.exercises.length > 6 && (
                <div className="mb-4 max-w-xs">
                  <Label htmlFor="records-filter">Find a lift</Label>
                  <Input id="records-filter" type="search" value={filter} onChange={e => setFilter(e.target.value)} placeholder="Squat, bench…" />
                </div>
              )}
              {rows.length === 0 && <p className="text-sm text-muted-foreground">No lifts match “{filter}”.</p>}

              <CardList label="Personal records">
                {rows.map(ex => (
                  <CardListItem
                    key={ex.exercise_id}
                    to={historyPath(ex)}
                    title={ex.name}
                    subtitle={`${ex.session_count} session${ex.session_count !== 1 ? 's' : ''} · last ${friendlyDate(ex.last_date)}`}
                    trailing={ex.best_e1rm ? (
                      <div>
                        <div className="font-semibold tabular-nums">{Math.round(ex.best_e1rm.e1rm)} lb</div>
                        <div className="text-xs text-muted-foreground">est. 1RM</div>
                      </div>
                    ) : undefined}
                  >
                    <div className="space-y-1">
                      {ex.rep_maxes.map(rm => (
                        <CardListDetail key={rm.target} label={`${rm.target}RM`}>
                          <span className="tabular-nums">{formatWeight(rm.weight)} lb</span>
                          <span className="text-xs text-muted-foreground"> · {date(rm.date)}</span>
                        </CardListDetail>
                      ))}
                      {ex.most_reps && (
                        <CardListDetail label="Most reps">
                          <span className="tabular-nums">{setLabel(ex.most_reps)}</span>
                          <span className="text-xs text-muted-foreground"> · {date(ex.most_reps.date)}</span>
                        </CardListDetail>
                      )}
                    </div>
                  </CardListItem>
                ))}
              </CardList>

              <DesktopOnly>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Lift</TableHead>
                      <TableHead>Est. 1RM</TableHead>
                      {TABLE_TARGETS.map(t => <TableHead key={t}>{t}RM</TableHead>)}
                      <TableHead>Most reps</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {rows.map(ex => (
                      <TableRow key={ex.exercise_id}>
                        <TableCell>
                          <Link to={historyPath(ex)} className="font-medium hover:text-primary">{ex.name}</Link>
                          <div className="text-xs text-muted-foreground">{ex.session_count} session{ex.session_count !== 1 ? 's' : ''}</div>
                        </TableCell>
                        <TableCell>
                          <RecordCell set={ex.best_e1rm} value={ex.best_e1rm ? `${Math.round(ex.best_e1rm.e1rm)} lb` : undefined} date={date} />
                        </TableCell>
                        {TABLE_TARGETS.map(t => {
                          const rm = repMax(ex, t)
                          return (
                            <TableCell key={t}>
                              <RecordCell set={rm} value={rm ? `${formatWeight(rm.weight)} lb` : undefined} date={date} />
                            </TableCell>
                          )
                        })}
                        <TableCell><RecordCell set={ex.most_reps} date={date} /></TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </DesktopOnly>
              <p className="mt-4 text-xs text-muted-foreground">
                Estimated 1RM uses the Epley formula on sets of {records.e1rm_max_reps} reps or fewer. Loads in lb.
              </p>
            </>
          )
        }}
      </QueryState>
    </div>
  )
}
