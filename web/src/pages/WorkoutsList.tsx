import { useState } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'
import { Spinner } from '@/components/ui'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

export function WorkoutsList() {
  const navigate = useNavigate()
  const { id } = useParams<{ id: string }>()
  const athleteId = Number(id)
  const [offset, setOffset] = useState(0)

  const { data: athlete } = useQuery({
    queryKey: ['athlete', athleteId],
    queryFn: () => api.getAthlete(athleteId),
    enabled: !isNaN(athleteId),
  })

  const { data: page, isLoading, error } = useQuery({
    queryKey: ['workouts', athleteId, offset],
    queryFn: () => api.listWorkouts(athleteId, offset),
    enabled: !isNaN(athleteId),
  })

  if (isLoading) return <Spinner />
  if (error) return <p className="text-destructive">Failed to load workouts.</p>

  return (
    <div>
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

      {page && page.workouts.length === 0 ? (
        <p className="text-muted-foreground">No workouts logged yet.</p>
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
              {page?.workouts.map(workout => (
                <TableRow key={workout.id} className="cursor-pointer" onClick={() => navigate(`/athletes/${athleteId}/workouts/${workout.id}`)}>
                  <TableCell className="font-medium">{workout.date}</TableCell>
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
              ))}
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