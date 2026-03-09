import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { api } from '@/api/client'
import { Spinner } from '@/components/ui'

export function PendingReviews() {
  const { data: workouts, isLoading, error } = useQuery({
    queryKey: ['pending-reviews'],
    queryFn: () => api.listPendingReviews(),
  })

  if (isLoading) return <Spinner />
  if (error) return <p className="text-destructive">Failed to load pending reviews.</p>

  return (
    <div>
      <h1 className="text-2xl font-bold mb-6">Pending Reviews</h1>

      {workouts && workouts.length === 0 ? (
        <div className="rounded-lg border border-border bg-card p-6 text-center">
          <p className="text-2xl mb-2">✅</p>
          <p className="text-muted-foreground">All workouts reviewed! Nice work, coach.</p>
        </div>
      ) : (
        <div className="space-y-2">
          {workouts?.map(w => (
            <Link
              key={w.workout_id}
              to={`/athletes/${w.athlete_id}/workouts/${w.workout_id}`}
              className="flex items-center justify-between rounded-lg border border-warning/30 bg-card p-4 hover:border-primary/50 transition-colors"
            >
              <div>
                <p className="font-medium">{w.athlete_name}</p>
                <p className="text-sm text-muted-foreground">{w.date}</p>
                {w.notes && (
                  <p className="text-xs text-muted-foreground mt-1 line-clamp-1">{w.notes}</p>
                )}
              </div>
              <div className="text-right">
                <p className="text-sm">{w.set_count} set{w.set_count !== 1 ? 's' : ''}</p>
                <span className="text-xs px-2 py-0.5 rounded-full bg-warning/10 text-warning">
                  Needs review
                </span>
              </div>
            </Link>
          ))}
        </div>
      )}
    </div>
  )
}
