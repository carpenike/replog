import { useParams, Link, useNavigate } from 'react-router-dom'
import { useQuery, useMutation } from '@tanstack/react-query'
import { toast } from 'sonner'
import { api, ApiError } from '@/api/client'
import { EmptyState, QueryError } from '@/components/ui'
import { usePageTitle } from '@/lib/usePageTitle'
import { formatDate, formatWeight, localDateISO } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

export function PrescriptionPage() {
  const navigate = useNavigate()
  const { id } = useParams<{ id: string }>()
  const athleteId = Number(id)

  usePageTitle("Today's Workout")

  const { data: athlete } = useQuery({
    queryKey: ['athlete', athleteId],
    queryFn: () => api.getAthlete(athleteId),
    enabled: !isNaN(athleteId),
  })

  const { data: prescription, isLoading, error, refetch } = useQuery({
    queryKey: ['prescription', athleteId],
    queryFn: () => api.getPrescription(athleteId),
    enabled: !isNaN(athleteId),
    retry: false,
  })

  const startLogging = useMutation({
    // fromPrescription=false: the session screen logs sets as the athlete
    // completes them — pre-creating the full prescription would mark every
    // row done (and record volume that was never performed). Local date, not
    // UTC: near midnight toISOString() lands the workout on the wrong day.
    mutationFn: () => api.createWorkout(athleteId, localDateISO(), '', false),
    meta: { skipGlobalError: true },
    onSuccess: (workout) => {
      navigate(`/athletes/${athleteId}/workouts/${workout.id}/session`)
    },
    onError: async (err) => {
      // A workout already exists for today — resume its live session. The 409
      // was raised against the same date string createWorkout sent, so match
      // on it to find the colliding workout.
      if (err instanceof ApiError && err.code === 409) {
        toast.info('A workout already exists for today — resuming it.')
        const today = localDateISO()
        try {
          const page = await api.listWorkouts(athleteId)
          const existing = page.workouts.find(w => formatDate(w.date) === today)
          navigate(existing
            ? `/athletes/${athleteId}/workouts/${existing.id}/session`
            : `/athletes/${athleteId}/workouts`)
        } catch {
          navigate(`/athletes/${athleteId}/workouts`)
        }
        return
      }
      toast.error('Failed to start logging.')
    },
  })

  const breadcrumb = (
    <p className="text-sm text-muted-foreground mb-1">
      <Link to={`/athletes/${athleteId}`} className="hover:text-foreground">{athlete?.name ?? 'Athlete'}</Link>
      {' / Prescription'}
    </p>
  )

  if (isLoading) {
    return (
      <div>
        {breadcrumb}
        <Skeleton className="h-8 w-48 mb-4" />
        <Skeleton className="h-32 w-full mb-4" />
        <Skeleton className="h-32 w-full" />
      </div>
    )
  }

  // A 404 means no program is assigned for today — an expected empty state.
  // Any other error is a real failure and should be retryable, not disguised.
  const noProgram = error instanceof ApiError && error.code === 404

  return (
    <div>
      {breadcrumb}

      {error && !noProgram ? (
        <div>
          <h1 className="text-2xl font-bold mb-4">Today's Workout</h1>
          <QueryError error={error} onRetry={refetch} resource="prescription" />
        </div>
      ) : noProgram || !prescription ? (
        <div>
          <h1 className="text-2xl font-bold mb-4">Today's Workout</h1>
          <Card className="text-center">
            <CardContent>
            <EmptyState icon="📋" title="No program assigned for today." description="Ask your coach to assign a program." />
            </CardContent>
          </Card>
        </div>
      ) : (
        <div>
          <div className="flex items-center justify-between mb-4">
            <div>
              <h1 className="text-2xl font-bold">Today's Workout</h1>
              <p className="text-sm text-muted-foreground">
                {prescription.program_name} — Week {prescription.current_week}, Day {prescription.current_day}
                {prescription.cycle_number > 1 && ` (Cycle ${prescription.cycle_number})`}
              </p>
            </div>
            <Button size="touch" onClick={() => startLogging.mutate()} disabled={startLogging.isPending}>
              {startLogging.isPending ? 'Starting…' : 'Start Logging'}
            </Button>
          </div>

          {prescription.lines.length === 0 ? (
            <EmptyState icon="😌" title="Rest day" description="No exercises prescribed." />
          ) : (
            <div className="space-y-4">
              {prescription.lines.map((line, i) => (
                <Card key={`${line.exercise_id}-${i}`} size="sm">
                  <CardHeader>
                    <CardTitle>{line.exercise_name}</CardTitle>
                    {line.training_max && (
                      <CardDescription>TM: {formatWeight(line.training_max)}</CardDescription>
                    )}
                  </CardHeader>
                  <CardContent>
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead className="w-16">Set</TableHead>
                          <TableHead>Reps</TableHead>
                          <TableHead>Weight</TableHead>
                          <TableHead>%</TableHead>
                          <TableHead className="whitespace-normal">Notes</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {line.sets.map((set, j) => (
                          <TableRow key={j}>
                            <TableCell className="text-muted-foreground">{set.set_number}</TableCell>
                            <TableCell className="font-medium">
                              {set.reps ? set.reps : 'AMRAP'}
                              {set.reps && set.rep_type === 'amrap' && '+'}
                            </TableCell>
                            <TableCell className="font-medium text-primary">
                              {set.target_weight ? formatWeight(set.target_weight)
                                : set.absolute_weight ? formatWeight(set.absolute_weight)
                                : 'BW'}
                            </TableCell>
                            <TableCell className="text-muted-foreground">
                              {set.percentage ? `${set.percentage}%` : ''}
                            </TableCell>
                            <TableCell className="text-muted-foreground text-xs whitespace-normal wrap-break-word">{set.notes ?? ''}</TableCell>
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
      )}
    </div>
  )
}
