import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { api } from '@/api/client'
import { Spinner } from '@/components/ui'
import { usePageTitle } from '@/lib/usePageTitle'
import type { User } from '@/api/types'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

const tierVariant: Record<string, 'default' | 'secondary' | 'outline'> = {
  foundational: 'secondary',
  intermediate: 'default',
  sport_performance: 'outline',
}

function tierLabel(tier: string): string {
  switch (tier) {
    case 'foundational': return 'Foundational'
    case 'intermediate': return 'Intermediate'
    case 'sport_performance': return 'Sport Perf'
    default: return tier
  }
}

export function AthletesList({ user }: { user: User }) {
  usePageTitle('Athletes')
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
          <Button onClick={() => navigate('/athletes/new')}>
            + New Athlete
          </Button>
        )}
      </div>

      <Input type="text" value={search} onChange={e => setSearch(e.target.value)} placeholder="Search athletes..." className="mb-4" />

      {filtered.length === 0 ? (
        <p className="text-muted-foreground">No athletes found.</p>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Tier</TableHead>
              <TableHead>Assignments</TableHead>
              <TableHead>Last Workout</TableHead>
              <TableHead>Streak</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {filtered.map(athlete => (
              <TableRow key={athlete.id} className="cursor-pointer" onClick={() => navigate(`/athletes/${athlete.id}`)}>
                <TableCell className="font-medium">
                  <div className="flex items-center gap-2">
                    {athlete.avatar_url ? (
                      <img src={athlete.avatar_url} alt="" className="h-6 w-6 rounded-full object-cover" />
                    ) : (
                      <div className="h-6 w-6 rounded-full bg-muted flex items-center justify-center text-xs font-bold text-muted-foreground">
                        {athlete.name.charAt(0).toUpperCase()}
                      </div>
                    )}
                    {athlete.name}
                  </div>
                </TableCell>
                <TableCell>
                  {athlete.tier ? (
                    <Badge variant={tierVariant[athlete.tier] ?? 'secondary'}>{tierLabel(athlete.tier)}</Badge>
                  ) : (
                    <span className="text-muted-foreground">—</span>
                  )}
                </TableCell>
                <TableCell>{athlete.active_assignments}</TableCell>
                <TableCell className="text-muted-foreground">{athlete.last_workout_date ?? '—'}</TableCell>
                <TableCell>{athlete.week_streak > 0 ? `🔥 ${athlete.week_streak}w` : '—'}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </div>
  )
}