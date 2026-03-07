import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'

export function WorkoutsList() {
  const { id } = useParams<{ id: string }>()
  const athleteId = Number(id)

  const { data: athlete } = useQuery({
    queryKey: ['athlete', athleteId],
    queryFn: () => api.getAthlete(athleteId),
    enabled: !isNaN(athleteId),
  })

  const { data: page, isLoading, error } = useQuery({
    queryKey: ['workouts', athleteId],
    queryFn: () => api.listWorkouts(athleteId),
    enabled: !isNaN(athleteId),
  })

  if (isLoading) return <p className="text-muted-foreground">Loading workouts...</p>
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
        <Link
          to={`/athletes/${athleteId}/workouts/new`}
          className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90"
        >
          + New Workout
        </Link>
      </div>

      {page && page.workouts.length === 0 ? (
        <p className="text-muted-foreground">No workouts logged yet.</p>
      ) : (
        <div className="space-y-2">
          {page?.workouts.map(workout => (
            <Link
              key={workout.id}
              to={`/athletes/${athleteId}/workouts/${workout.id}`}
              className="flex items-center justify-between rounded-lg border border-border bg-card p-4 hover:border-primary/50 transition-colors"
            >
              <div>
                <p className="font-medium">{workout.date}</p>
                {workout.program_name && (
                  <p className="text-sm text-muted-foreground">{workout.program_name}</p>
                )}
              </div>
              <div className="text-right">
                <p className="text-sm">{workout.set_count} set{workout.set_count !== 1 ? 's' : ''}</p>
                {workout.review_status && (
                  <span className={`text-xs px-2 py-0.5 rounded-full ${
                    workout.review_status === 'approved' ? 'bg-success/10 text-success' :
                    workout.review_status === 'needs_work' ? 'bg-warning/10 text-warning' :
                    'bg-muted text-muted-foreground'
                  }`}>
                    {workout.review_status}
                  </span>
                )}
              </div>
            </Link>
          ))}
        </div>
      )}
    </div>
  )
}
