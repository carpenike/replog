import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { api } from '@/api/client'
import { PageHeader } from '@/components/PageHeader'
import { QueryState } from '@/components/QueryState'
import { useFriendlyDate } from '@/lib/dates'

export function PendingReviews() {
  const friendlyDate = useFriendlyDate()
  const query = useQuery({
    queryKey: ['pending-reviews'],
    queryFn: () => api.listPendingReviews(),
  })

  return (
    <div>
      <PageHeader title="Pending Reviews" />
      <QueryState
        query={query}
        resource="pending reviews"
        empty={{ icon: '✅', title: 'All workouts reviewed', description: 'Nice work, coach. New sessions show up here when athletes log them.' }}
      >
        {workouts => (
          <ul className="space-y-2">
            {workouts.map(w => (
              <li key={w.workout_id}>
                <Link
                  to={`/athletes/${w.athlete_id}/workouts/${w.workout_id}`}
                  className="flex items-center justify-between gap-3 rounded-lg border border-warning/30 bg-card p-4 hover:border-primary/50 transition-colors focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
                >
                  <div className="min-w-0">
                    <p className="font-medium">{w.athlete_name}</p>
                    <p className="text-sm text-muted-foreground">{friendlyDate(w.date)}</p>
                    {w.notes && (
                      <p className="text-xs text-muted-foreground mt-1 line-clamp-1">{w.notes}</p>
                    )}
                  </div>
                  <div className="text-right shrink-0">
                    <p className="text-sm">{w.set_count} set{w.set_count !== 1 ? 's' : ''}</p>
                    <div className="mt-0.5 flex flex-wrap justify-end gap-1">
                      {w.low_readiness && (
                        <span className="text-xs px-2 py-0.5 rounded-full bg-destructive/10 text-destructive">
                          Low readiness
                        </span>
                      )}
                      {(w.swap_count ?? 0) > 0 && (
                        <span className="text-xs px-2 py-0.5 rounded-full bg-muted text-muted-foreground">
                          {w.swap_count} swap{w.swap_count === 1 ? '' : 's'}
                        </span>
                      )}
                      <span className="text-xs px-2 py-0.5 rounded-full bg-warning/10 text-warning-strong">
                        Needs review
                      </span>
                    </div>
                  </div>
                </Link>
              </li>
            ))}
          </ul>
        )}
      </QueryState>
    </div>
  )
}
