import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link, useNavigate } from 'react-router-dom'
import { api } from '@/api/client'
import { Spinner } from '@/components/ui'
import type { User } from '@/api/types'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
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
  const navigate = useNavigate()
  const [search, setSearch] = useState('')
  const { data: athletes, isLoading, error } = useQuery({
    queryKey: ['athletes'],
    queryFn: () => api.listAthletes(),
  })
  if (isLoading) return <Spinner />
  if (error) return <p className="text-destructive">Failed to load athletes.</p>
  const filtered = athletes?.filter(a =>
    a.name.toLowerCase().includes(search.toLowerCase())
  ) ?? []
  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <h1 className="text-2xl font-bold">Athletes</h1>
        {(user.is_coach || user.is_admin) && (
          <Button onClick={() => navigate('/athletes/new')} >
            + New Athlete
          </Button>
        )}
      </div>
      <Input type="text" value={search} onChange={e => setSearch(e.target.value)} placeholder="Search athletes..." />
      {filtered.length === 0 ? (
        <p className="text-muted-foreground">No athletes found.</p>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {filtered.map(athlete => (
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