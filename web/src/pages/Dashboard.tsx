import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { api } from '@/api/client'
import { EmptyState, QueryError } from '@/components/ui'
import { fetchWeekStreak } from '@/lib/streak'
import { usePageTitle } from '@/lib/usePageTitle'
import { cn, formatDate } from '@/lib/utils'
import { Card, CardContent } from '@/components/ui/card'
import { Alert } from '@/components/ui/alert'
import { Skeleton } from '@/components/ui/skeleton'
import type { User } from '@/api/types'

interface DashboardProps {
  user: User
}

function formatVolume(v: number): string {
  if (v < 1000) return v.toString()
  return v.toLocaleString('en-US', { maximumFractionDigits: 0 })
}

export function Dashboard({ user }: DashboardProps) {
  usePageTitle('Dashboard')
  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['dashboard'],
    queryFn: () => api.dashboard(),
  })
  // The dashboard endpoint only returns streaks for coach-owned athlete
  // cards, so athlete users compute their own from recent workouts.
  const athleteId = !user.is_coach && !user.is_admin ? user.athlete_id : null
  const { data: streak } = useQuery({
    queryKey: ['week-streak', athleteId],
    queryFn: () => fetchWeekStreak(athleteId!),
    enabled: athleteId != null,
    staleTime: 5 * 60_000,
  })

  if (isLoading) {
    return (
      <div>
        <Skeleton className="h-8 w-56 mb-6" />
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-8">
          {Array.from({ length: 4 }).map((_, i) => <Skeleton key={i} className="h-24 w-full" />)}
        </div>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
          {Array.from({ length: 6 }).map((_, i) => <Skeleton key={i} className="h-16 w-full" />)}
        </div>
      </div>
    )
  }
  if (error) {
    return (
      <div>
        <h1 className="text-2xl font-bold mb-6">Welcome, {user.name ?? user.username}</h1>
        <QueryError error={error} onRetry={refetch} resource="dashboard" />
      </div>
    )
  }

  // Athlete-only user: redirect-like quick actions for their own profile
  if (!user.is_coach && !user.is_admin && user.athlete_id) {
    return (
      <div>
        <h1 className="text-2xl font-bold mb-6">
          Welcome, {user.name ?? user.username}
        </h1>
        {streak != null && (
          <div
            className={cn(
              'mb-4 rounded-lg border px-4 py-3',
              streak > 0 ? 'border-warning/30 bg-warning/5' : 'border-border bg-card',
            )}
          >
            {streak > 0 ? (
              <p className="text-sm font-medium">
                <span aria-hidden="true">🔥</span> {streak}-week streak
                <span className="font-normal text-muted-foreground"> — keep it going!</span>
              </p>
            ) : (
              <p className="text-sm text-muted-foreground">
                Log a workout this week to start a streak <span aria-hidden="true">💪</span>
              </p>
            )}
          </div>
        )}
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <Link to={`/athletes/${user.athlete_id}/prescription`}
            className="rounded-lg border border-primary/30 bg-primary/5 p-6 hover:border-primary/50 transition-colors text-center">
            <span className="text-3xl mb-2 block">📋</span>
            <p className="font-semibold">Today's Workout</p>
            <p className="text-sm text-muted-foreground mt-1">View your prescribed training</p>
          </Link>
          <Link to={`/athletes/${user.athlete_id}/workouts/new`}
            className="rounded-lg border border-border bg-card p-6 hover:border-primary/50 transition-colors text-center">
            <span className="text-3xl mb-2 block">🏋️</span>
            <p className="font-semibold">Log Workout</p>
            <p className="text-sm text-muted-foreground mt-1">Start a new workout session</p>
          </Link>
          <Link to={`/athletes/${user.athlete_id}/workouts`}
            className="rounded-lg border border-border bg-card p-6 hover:border-primary/50 transition-colors text-center">
            <span className="text-3xl mb-2 block">📝</span>
            <p className="font-semibold">Workout History</p>
            <p className="text-sm text-muted-foreground mt-1">View past workouts</p>
          </Link>
          <Link to={`/athletes/${user.athlete_id}/body-weights`}
            className="rounded-lg border border-border bg-card p-6 hover:border-primary/50 transition-colors text-center">
            <span className="text-3xl mb-2 block">⚖️</span>
            <p className="font-semibold">Body Weight</p>
            <p className="text-sm text-muted-foreground mt-1">Log or view body weight</p>
          </Link>
          <Link to={`/athletes/${user.athlete_id}`}
            className="rounded-lg border border-border bg-card p-6 hover:border-primary/50 transition-colors text-center">
            <span className="text-3xl mb-2 block">👤</span>
            <p className="font-semibold">My Profile</p>
            <p className="text-sm text-muted-foreground mt-1">View training maxes, programs, journal</p>
          </Link>
        </div>
      </div>
    )
  }

  // Non-coach user without a linked athlete
  if (!user.is_coach && !user.is_admin) {
    return (
      <div>
        <h1 className="text-2xl font-bold mb-6">
          Welcome, {user.name ?? user.username}
        </h1>
        <Card className="text-center">
          <CardContent>
          <span className="text-4xl block mb-3">👋</span>
          <p className="font-semibold text-lg mb-2">You're all set!</p>
          <p className="text-muted-foreground max-w-md mx-auto">
            Your account hasn't been linked to an athlete profile yet.
            Ask your coach to connect your account so you can start logging workouts.
          </p>
          </CardContent>
        </Card>
      </div>
    )
  }

  return (
    <div>
      <h1 className="text-2xl font-bold mb-6">
        Welcome, {user.name ?? user.username}
      </h1>

      {/* Stats cards */}
      {data?.stats && (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-8">
          <Card size="sm">
            <CardContent>
              <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">This Week</p>
              <p className="text-2xl font-bold mt-1">{data.stats.week_sessions}</p>
              <p className="text-xs text-muted-foreground">sessions</p>
            </CardContent>
          </Card>
          <Card size="sm">
            <CardContent>
              <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">Volume</p>
              <p className="text-2xl font-bold mt-1">{formatVolume(data.stats.week_volume)}</p>
              <p className="text-xs text-muted-foreground">lbs this week</p>
            </CardContent>
          </Card>
          <Card size="sm">
            <CardContent>
              <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">Athletes</p>
              <p className="text-2xl font-bold mt-1">{data.stats.trained_this_week}/{data.stats.total_athletes}</p>
              <p className="text-xs text-muted-foreground">trained this week</p>
            </CardContent>
          </Card>
          <Card size="sm">
            <CardContent>
              <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">Streak</p>
              <p className="text-2xl font-bold mt-1">{data.stats.consecutive_weeks}</p>
              <p className="text-xs text-muted-foreground">consecutive weeks</p>
            </CardContent>
          </Card>
        </div>
      )}

      {/* Review stats for coaches */}
      {data?.review_stats && data.review_stats.pending_count > 0 && (
        <Alert variant="warning" className="mb-6">
          <div className="flex items-center justify-between">
            <div>
              <p className="font-medium">Pending Reviews</p>
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
        </Alert>
      )}

      {/* Athletes grid */}
      {data?.athletes && data.athletes.length === 0 && (
        <EmptyState
          icon="🧑‍🤝‍🧑"
          title="No athletes yet"
          description="Add your first athlete to get started."
          action="Add athlete"
          actionTo="/athletes/new"
        />
      )}
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
                <div className="flex items-center gap-3">
                  {a.avatar_url ? (
                    <img src={a.avatar_url} alt="" className="h-8 w-8 rounded-full object-cover" />
                  ) : (
                    <div className="h-8 w-8 rounded-full bg-muted flex items-center justify-center text-sm font-bold text-muted-foreground">
                      {a.name.charAt(0).toUpperCase()}
                    </div>
                  )}
                  <div>
                    <p className="font-medium">{a.name}</p>
                    <p className="text-xs text-muted-foreground">
                      {a.last_workout_date ? `Last: ${formatDate(a.last_workout_date)}` : 'No workouts'}
                    </p>
                  </div>
                </div>
                <div className="text-right">
                  {a.week_streak > 0 && (
                    <span className="text-sm"><span aria-hidden="true">🔥</span> {a.week_streak}w</span>
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
