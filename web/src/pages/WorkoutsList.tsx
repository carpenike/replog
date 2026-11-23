import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { ChevronLeft, ChevronRight, Plus } from 'lucide-react'
import { api } from '@/api/client'
import { PageHeader } from '@/components/PageHeader'
import { QueryState } from '@/components/QueryState'
import { CardList, CardListItem, DesktopOnly, RowLink } from '@/components/CardList'
import { useFriendlyDate } from '@/lib/dates'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

function ReviewBadge({ status }: { status?: string | null }) {
  if (!status) return <span className="text-muted-foreground">—</span>
  return <Badge variant={status === 'approved' ? 'secondary' : 'destructive'}>{status}</Badge>
}

export function WorkoutsList() {
  const { id } = useParams<{ id: string }>()
  const athleteId = Number(id)
  const [offset, setOffset] = useState(0)
  const friendlyDate = useFriendlyDate()

  const { data: athlete } = useQuery({
    queryKey: ['athlete', athleteId],
    queryFn: () => api.getAthlete(athleteId),
    enabled: !isNaN(athleteId),
  })

  const query = useQuery({
    queryKey: ['workouts', athleteId, offset],
    queryFn: () => api.listWorkouts(athleteId, offset),
    enabled: !isNaN(athleteId),
  })
  const newTo = `/athletes/${athleteId}/workouts/new`

  return (
    <div>
      <PageHeader
        title="Workouts"
        back={{ to: `/athletes/${athleteId}`, label: athlete?.name ?? 'Athlete' }}
        actions={<Button render={<Link to={newTo} />}><Plus aria-hidden="true" /> New Workout</Button>}
      />

      <QueryState
        query={query}
        resource="workouts"
        isEmpty={p => p.workouts.length === 0 && offset === 0}
        empty={{
          icon: '🏋️',
          title: 'No workouts logged yet',
          description: 'Start logging to build training history.',
          action: 'New Workout',
          actionTo: newTo,
        }}
      >
        {page => (
          <>
            <CardList label="Workouts">
              {page.workouts.map(w => (
                <CardListItem
                  key={w.id}
                  to={`/athletes/${athleteId}/workouts/${w.id}`}
                  title={friendlyDate(w.date)}
                  subtitle={`${w.program_name ?? 'Unprogrammed'} · ${w.set_count} set${w.set_count === 1 ? '' : 's'}`}
                  trailing={w.review_status ? <ReviewBadge status={w.review_status} /> : undefined}
                />
              ))}
            </CardList>
            <DesktopOnly>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Date</TableHead>
                    <TableHead>Program</TableHead>
                    <TableHead>Sets</TableHead>
                    <TableHead>Review</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {page.workouts.map(workout => (
                    <TableRow key={workout.id} className="relative">
                      <TableCell>
                        <RowLink to={`/athletes/${athleteId}/workouts/${workout.id}`}>{friendlyDate(workout.date)}</RowLink>
                      </TableCell>
                      <TableCell className="text-muted-foreground">{workout.program_name ?? '—'}</TableCell>
                      <TableCell>{workout.set_count}</TableCell>
                      <TableCell><ReviewBadge status={workout.review_status} /></TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </DesktopOnly>

            {(offset > 0 || page.has_more) && (
              <div className="flex items-center justify-between pt-4">
                <Button variant="ghost" onClick={() => setOffset(Math.max(0, offset - 20))} disabled={offset === 0}>
                  <ChevronLeft aria-hidden="true" /> Previous
                </Button>
                {page.has_more && (
                  <Button variant="ghost" onClick={() => setOffset(offset + 20)}>
                    Next <ChevronRight aria-hidden="true" />
                  </Button>
                )}
              </div>
            )}
          </>
        )}
      </QueryState>
    </div>
  )
}
