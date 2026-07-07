import { useState } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'
import { EmptyState, QueryError } from '@/components/ui'
import { usePageTitle } from '@/lib/usePageTitle'
import { formatDate } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

export function WorkoutsList() {
  const navigate = useNavigate()
  const { id } = useParams<{ id: string }>()
  const athleteId = Number(id)
  const [offset, setOffset] = useState(0)

  usePageTitle('Workouts')

  const { data: athlete } = useQuery({
    queryKey: ['athlete', athleteId],
    queryFn: () => api.getAthlete(athleteId),
    enabled: !isNaN(athleteId),
  })

  const { data: page, isLoading, error, refetch } = useQuery({
    queryKey: ['workouts', athleteId, offset],
    queryFn: () => api.listWorkouts(athleteId, offset),
    enabled: !isNaN(athleteId),
  })

  const header = (
    <div className="flex items-center justify-between mb-6">
      <div>
        <p className="text-sm text-muted-foreground">
          <Link to={`/athletes/${athleteId}`} className="hover:text-foreground">
            {athlete?.name ?? 'Athlete'}
          </Link>
          {' / '}
          Workouts
        </p>
        <h1 className="text-2xl font-bold">Workouts</h1>
      </div>
      <Button onClick={() => navigate(`/athletes/${athleteId}/workouts/new`)}>
        + New Workout
      </Button>
    </div>
  )

  if (isLoading) {
    return (
      <div>
        {header}
        <div className="space-y-2">
          {Array.from({ length: 6 }).map((_, i) => <Skeleton key={i} className="h-10 w-full" />)}
        </div>
      </div>
    )
  }
  if (error) return <div>{header}<QueryError error={error} onRetry={refetch} resource="workouts" /></div>

  return (
    <div>
      {header}

      {page && page.workouts.length === 0 ? (
        <EmptyState
          icon="🏋️"
          title="No workouts logged yet"
          description="Start logging to build training history."
          action="+ New Workout"
          actionTo={`/athletes/${athleteId}/workouts/new`}
        />
      ) : (
        <>
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
              {page?.workouts.map(workout => {
                const go = () => navigate(`/athletes/${athleteId}/workouts/${workout.id}`)
                return (
                <TableRow
                  key={workout.id}
                  role="button"
                  tabIndex={0}
                  aria-label={`Open workout ${formatDate(workout.date)}`}
                  className="cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/50"
                  onClick={go}
                  onKeyDown={e => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); go() } }}
                >
                  <TableCell className="font-medium">{formatDate(workout.date)}</TableCell>
                  <TableCell className="text-muted-foreground">{workout.program_name ?? '—'}</TableCell>
                  <TableCell>{workout.set_count}</TableCell>
                  <TableCell>
                    {workout.review_status ? (
                      <Badge variant={workout.review_status === 'approved' ? 'secondary' : 'destructive'}>
                        {workout.review_status}
                      </Badge>
                    ) : '—'}
                  </TableCell>
                </TableRow>
                )
              })}
            </TableBody>
          </Table>

          <div className="flex items-center justify-between pt-4">
            <Button variant="ghost" onClick={() => setOffset(Math.max(0, offset - 20))}
              disabled={offset === 0}>
              ← Previous
            </Button>
            {page?.has_more && (
              <Button variant="ghost" onClick={() => setOffset(offset + 20)}>
                Next →
              </Button>
            )}
          </div>
        </>
      )}
    </div>
  )
}