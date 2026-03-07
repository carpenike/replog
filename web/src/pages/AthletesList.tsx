import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { api } from '@/api/client'
import type { User } from '@/api/types'

const tierColors: Record<string, string> = {
  foundational: 'bg-emerald-500/10 text-emerald-400',
  intermediate: 'bg-amber-500/10 text-amber-400',
  sport_performance: 'bg-purple-500/10 text-purple-400',
}

function tierLabel(tier: string): string {
  switch (tier) {
    case 'foundational': return 'Foundational'
    case 'intermediate': return 'Intermediate'
    case 'sport_performance': return 'Sport Performance'
    default: return tier
  }
}

export function AthletesList({ user }: { user: User }) {
  const { data: athletes, isLoading, error } = useQuery({
    queryKey: ['athletes'],
    queryFn: () => api.listAthletes(),
  })

  if (isLoading) return <p className="text-muted-foreground">Loading athletes...</p>
  if (error) return <p className="text-destructive">Failed to load athletes.</p>

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">Athletes</h1>
        {(user.is_coach || user.is_admin) && (
          <Link to="/athletes/new" className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90">
            + New Athlete
          </Link>
        )}
      </div>

      {athletes && athletes.length === 0 ? (
        <p className="text-muted-foreground">No athletes found.</p>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {athletes?.map(athlete => (
            <Link
              key={athlete.id}
              to={`/athletes/${athlete.id}`}
              className="rounded-lg border border-border bg-card p-4 hover:border-primary/50 transition-colors"
            >
              <h3 className="font-semibold text-foreground">{athlete.name}</h3>
              <div className="flex items-center gap-2 mt-2">
                {athlete.tier && (
                  <span className={`text-xs px-2 py-0.5 rounded-full ${tierColors[athlete.tier] ?? 'bg-muted text-muted-foreground'}`}>
                    {tierLabel(athlete.tier)}
                  </span>
                )}
                {athlete.week_streak > 0 && (
                  <span className="text-xs text-muted-foreground">
                    🔥 {athlete.week_streak}w streak
                  </span>
                )}
              </div>
              <div className="mt-3 text-xs text-muted-foreground space-y-0.5">
                <p>{athlete.active_assignments} active assignment{athlete.active_assignments !== 1 ? 's' : ''}</p>
                {athlete.last_workout_date && (
                  <p>Last workout: {athlete.last_workout_date}</p>
                )}
              </div>
            </Link>
          ))}
        </div>
      )}
    </div>
  )
}
