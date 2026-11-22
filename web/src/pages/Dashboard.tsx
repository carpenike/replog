import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import { api } from '@/api/client'
import { EmptyState, QueryError } from '@/components/ui'
import { Button } from '@/components/ui/button'
import { fetchWeekStreak } from '@/lib/streak'
import { openTodaySession, sessionPath, workoutOnDate } from '@/lib/todaySession'
import { usePageTitle } from '@/lib/usePageTitle'
import { cn, formatDate, localDateISO } from '@/lib/utils'
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
  // Linked non-coach: the home screen is today's log, one tap away.
  // Coaches and admins keep the staff dashboard; when they also train
  // (athlete_id set) it opens with their own session button and streak.
  if (!user.is_coach && !user.is_admin && user.athlete_id != null) {
    return <AthleteHome user={user} athleteId={user.athlete_id} />
  }
  return <StaffHome user={user} />
}

/**
 * Today's session button and the week streak. The button opens today's
 * resistance session — resume the day's workout, or create an empty one.
 * The screen records the log. It does not choose the work.
 */
function TodaySessionCard({ athleteId }: { athleteId: number }) {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const today = localDateISO()
  const { data: streak } = useQuery({
    queryKey: ['week-streak', athleteId],
    queryFn: () => fetchWeekStreak(athleteId),
    staleTime: 5 * 60_000,
  })
  const workouts = useQuery({
    queryKey: ['workouts', athleteId, 0],
    queryFn: () => api.listWorkouts(athleteId),
  })
  const todayWorkout = workouts.data ? workoutOnDate(workouts.data.workouts, today) : undefined

  const openSession = useMutation({
    mutationFn: () => openTodaySession(api, athleteId, today),
    meta: { skipGlobalError: true },
    onSuccess: (path) => {
      queryClient.invalidateQueries({ queryKey: ['workouts', athleteId] })
      navigate(path)
    },
    onError: () => {
      toast.error("Couldn't open today's session.")
    },
  })

  const sessionLabel = !workouts.isSuccess
    ? "Today's session"
    : todayWorkout
      ? "Continue today's session"
      : "Start today's session"
  const sessionClass = 'h-14 w-full text-base font-semibold'

  return (
    <>
      {todayWorkout ? (
        <Button
          size="touch"
          className={sessionClass}
          render={<Link to={sessionPath(athleteId, todayWorkout.id)} />}
        >
          {sessionLabel}
        </Button>
      ) : (
        <Button
          size="touch"
          className={sessionClass}
          onClick={() => openSession.mutate()}
          disabled={openSession.isPending}
        >
          {openSession.isPending ? 'Opening…' : sessionLabel}
        </Button>
      )}
      <p className="mt-2 text-sm text-muted-foreground">Log today's sets.</p>

      {streak != null && (
        <div
          className={cn(
            'mt-4 rounded-lg border px-4 py-3',
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
    </>
  )
}

/** Kid home: today's session one tap away. */
function AthleteHome({ user, athleteId }: { user: User; athleteId: number }) {
  usePageTitle('Today')

  return (
    <div>
      <h1 className="text-2xl font-bold mb-4">
        Welcome, {user.name ?? user.username}
      </h1>

      <TodaySessionCard athleteId={athleteId} />

      <nav aria-label="More" className="mt-6 flex flex-wrap gap-x-4 gap-y-2 text-sm">
        <Link to={`/athletes/${athleteId}/prescription`} className="text-muted-foreground hover:text-foreground">
          Today's plan
        </Link>
        <Link to={`/athletes/${athleteId}/workouts`} className="text-muted-foreground hover:text-foreground">
          History
        </Link>
        <Link to={`/athletes/${athleteId}/body-weights`} className="text-muted-foreground hover:text-foreground">
          Body weight
        </Link>
        <Link to={`/athletes/${athleteId}`} className="text-muted-foreground hover:text-foreground">
          Profile
        </Link>
      </nav>
    </div>
  )
}

function StaffHome({ user }: DashboardProps) {
  usePageTitle('Dashboard')
  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['dashboard'],
    queryFn: () => api.dashboard(),
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

      {/* Staff who also train get their own session first. */}
      {user.athlete_id != null && (
        <section aria-label="My training" className="mb-8">
          <TodaySessionCard athleteId={user.athlete_id} />
        </section>
      )}

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
