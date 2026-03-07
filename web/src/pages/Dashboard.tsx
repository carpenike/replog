import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { api } from '@/api/client'
import type { User } from '@/api/types'

interface DashboardProps {
  user: User
}

function formatVolume(v: number): string {
  if (v < 1000) return v.toString()
  return v.toLocaleString('en-US', { maximumFractionDigits: 0 })
}

export function Dashboard({ user }: DashboardProps) {
  const { data } = useQuery({
    queryKey: ['dashboard'],
    queryFn: () => api.dashboard(),
  })

  return (
    <div>
      <h1 className="text-2xl font-bold mb-6">
        Welcome, {user.name ?? user.username}
      </h1>

      {/* Stats cards */}
      {data?.stats && (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-8">
          <div className="rounded-lg border border-border bg-card p-4">
            <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">This Week</p>
            <p className="text-2xl font-bold mt-1">{data.stats.week_sessions}</p>
            <p className="text-xs text-muted-foreground">sessions</p>
          </div>
          <div className="rounded-lg border border-border bg-card p-4">
            <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">Volume</p>
            <p className="text-2xl font-bold mt-1">{formatVolume(data.stats.week_volume)}</p>
            <p className="text-xs text-muted-foreground">lbs this week</p>
          </div>
          <div className="rounded-lg border border-border bg-card p-4">
            <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">Athletes</p>
            <p className="text-2xl font-bold mt-1">{data.stats.trained_this_week}/{data.stats.total_athletes}</p>
            <p className="text-xs text-muted-foreground">trained this week</p>
          </div>
          <div className="rounded-lg border border-border bg-card p-4">
            <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">Streak</p>
            <p className="text-2xl font-bold mt-1">{data.stats.consecutive_weeks}</p>
            <p className="text-xs text-muted-foreground">consecutive weeks</p>
          </div>
        </div>
      )}

      {/* Review stats for coaches */}
      {data?.review_stats && data.review_stats.pending_count > 0 && (
        <div className="rounded-lg border border-warning/30 bg-warning/5 p-4 mb-6">
          <div className="flex items-center justify-between">
            <div>
              <p className="font-medium text-warning">Pending Reviews</p>
              <p className="text-sm text-muted-foreground">
                {data.review_stats.pending_count} workout{data.review_stats.pending_count !== 1 ? 's' : ''} awaiting review
              </p>
            </div>
            <Link
              to="/reviews/pending"
              className="text-sm font-medium text-primary hover:text-primary/80"
            >
              Review now &rarr;
            </Link>
          </div>
        </div>
      )}

      {/* Athletes grid */}
      {data?.athletes && data.athletes.length > 0 && (
        <div>
          <h2 className="text-lg font-semibold mb-3">Athletes</h2>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
            {data.athletes.map(a => (
              <Link
                key={a.id}
                to={`/athletes/${a.id}`}
                className="flex items-center justify-between rounded-lg border border-border bg-card p-3 hover:border-primary/50 transition-colors"
              >
                <div>
                  <p className="font-medium">{a.name}</p>
                  <p className="text-xs text-muted-foreground">
                    {a.last_workout_date ? `Last: ${a.last_workout_date}` : 'No workouts'}
                  </p>
                </div>
                <div className="text-right">
                  {a.week_streak > 0 && (
                    <span className="text-sm">🔥 {a.week_streak}w</span>
                  )}
                </div>
              </Link>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
