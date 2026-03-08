import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { AthleteProgram } from '@/api/types'

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

export function AthleteDetail() {
  const { id } = useParams<{ id: string }>()
  const athleteId = Number(id)

  const { data: athlete, isLoading, error } = useQuery({
    queryKey: ['athlete', athleteId],
    queryFn: () => api.getAthlete(athleteId),
    enabled: !isNaN(athleteId),
  })

  const { data: workouts } = useQuery({
    queryKey: ['workouts', athleteId, 'recent'],
    queryFn: () => api.listWorkouts(athleteId, 0),
    enabled: !isNaN(athleteId),
  })

  const { data: trainingMaxes } = useQuery({
    queryKey: ['training-maxes', athleteId],
    queryFn: () => api.listTrainingMaxes(athleteId),
    enabled: !isNaN(athleteId),
  })

  const { data: programs } = useQuery({
    queryKey: ['athlete-programs', athleteId],
    queryFn: () => api.listAthletePrograms(athleteId) as Promise<AthleteProgram[]>,
    enabled: !isNaN(athleteId),
  })

  if (isLoading) return <p className="text-muted-foreground">Loading athlete...</p>
  if (error) return <p className="text-destructive">Failed to load athlete.</p>
  if (!athlete) return <p className="text-muted-foreground">Athlete not found.</p>

  const recentWorkouts = workouts?.workouts.slice(0, 5) ?? []

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold">{athlete.name}</h1>
          {athlete.tier && (
            <span className={`text-xs px-2 py-0.5 rounded-full mt-1 inline-block ${tierColors[athlete.tier] ?? 'bg-muted text-muted-foreground'}`}>
              {tierLabel(athlete.tier)}
            </span>
          )}
        </div>
        <Link to={`/athletes/${athleteId}/edit`}
          className="rounded-md border border-border px-3 py-1.5 text-sm hover:bg-accent transition-colors">
          ✏️ Edit
        </Link>
      </div>

      {/* Quick nav */}
      <div className="flex flex-wrap gap-2 mb-6">
        <Link to={`/athletes/${athleteId}/workouts`} className="rounded-md border border-border bg-card px-3 py-1.5 text-sm hover:border-primary/50 transition-colors">
          📝 Workouts
        </Link>
        <Link to={`/athletes/${athleteId}/body-weights`} className="rounded-md border border-border bg-card px-3 py-1.5 text-sm hover:border-primary/50 transition-colors">
          ⚖️ Body Weight
        </Link>
        <Link to={`/athletes/${athleteId}/training-maxes`} className="rounded-md border border-border bg-card px-3 py-1.5 text-sm hover:border-primary/50 transition-colors">
          💪 Training Maxes
        </Link>
        <Link to={`/athletes/${athleteId}/journal`} className="rounded-md border border-border bg-card px-3 py-1.5 text-sm hover:border-primary/50 transition-colors">
          📖 Journal
        </Link>
        <Link to={`/athletes/${athleteId}/accessories`} className="rounded-md border border-border bg-card px-3 py-1.5 text-sm hover:border-primary/50 transition-colors">
          🔧 Accessories
        </Link>
      </div>

      {/* Info cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-6">
        {athlete.goal && (
          <div className="rounded-lg border border-border bg-card p-4">
            <h2 className="text-sm font-medium text-muted-foreground mb-1">Goal</h2>
            <p className="text-foreground">{athlete.goal}</p>
          </div>
        )}
        {athlete.notes && (
          <div className="rounded-lg border border-border bg-card p-4">
            <h2 className="text-sm font-medium text-muted-foreground mb-1">Notes</h2>
            <p className="text-foreground">{athlete.notes}</p>
          </div>
        )}
        {athlete.date_of_birth && (
          <div className="rounded-lg border border-border bg-card p-4">
            <h2 className="text-sm font-medium text-muted-foreground mb-1">Date of Birth</h2>
            <p className="text-foreground">{athlete.date_of_birth}</p>
          </div>
        )}
      </div>

      {/* Active Programs */}
      {programs && programs.length > 0 && (
        <div className="mb-6">
          <h2 className="text-lg font-semibold mb-2">Active Programs</h2>
          <div className="space-y-2">
            {programs.filter(p => p.active).map(p => (
              <Link key={p.id} to={`/programs/${p.template_id}`}
                className="flex items-center justify-between rounded-lg border border-border bg-card p-3 hover:border-primary/50 transition-colors">
                <div>
                  <p className="font-medium">{p.template_name}</p>
                  <p className="text-xs text-muted-foreground">
                    Started {p.start_date} • {p.role}
                    {p.num_weeks ? ` • ${p.num_weeks}w` : ''}
                  </p>
                </div>
                {p.is_loop && <span className="text-xs text-primary">Loop</span>}
              </Link>
            ))}
          </div>
        </div>
      )}

      {/* Training Maxes */}
      {trainingMaxes && trainingMaxes.length > 0 && (
        <div className="mb-6">
          <div className="flex items-center justify-between mb-2">
            <h2 className="text-lg font-semibold">Current Training Maxes</h2>
            <Link to={`/athletes/${athleteId}/training-maxes`} className="text-sm text-primary hover:text-primary/80">
              View all &rarr;
            </Link>
          </div>
          <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-2">
            {trainingMaxes.map(tm => (
              <div key={tm.id} className="rounded-lg border border-border bg-card p-3">
                <p className="text-sm text-muted-foreground">{tm.exercise_name}</p>
                <p className="text-lg font-bold">{tm.weight}</p>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Recent Workouts */}
      <div>
        <div className="flex items-center justify-between mb-2">
          <h2 className="text-lg font-semibold">Recent Workouts</h2>
          <Link to={`/athletes/${athleteId}/workouts`} className="text-sm text-primary hover:text-primary/80">
            View all &rarr;
          </Link>
        </div>
        {recentWorkouts.length === 0 ? (
          <p className="text-sm text-muted-foreground">No workouts logged yet.</p>
        ) : (
          <div className="space-y-2">
            {recentWorkouts.map(w => (
              <Link
                key={w.id}
                to={`/athletes/${athleteId}/workouts/${w.id}`}
                className="flex items-center justify-between rounded-lg border border-border bg-card p-3 hover:border-primary/50 transition-colors"
              >
                <div>
                  <p className="text-sm font-medium">{w.date}</p>
                  {w.program_name && <p className="text-xs text-muted-foreground">{w.program_name}</p>}
                </div>
                <span className="text-xs text-muted-foreground">{w.set_count} sets</span>
              </Link>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
