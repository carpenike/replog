import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { Plus } from 'lucide-react'
import { api } from '@/api/client'
import type { AthleteCard, User } from '@/api/types'
import { PageHeader } from '@/components/PageHeader'
import { QueryState } from '@/components/QueryState'
import { CardList, CardListItem, DesktopOnly, RowLink } from '@/components/CardList'
import { EmptyState } from '@/components/ui'
import { useFriendlyDate } from '@/lib/dates'
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

function AthleteAvatar({ athlete }: { athlete: AthleteCard }) {
  return athlete.avatar_url ? (
    <img src={athlete.avatar_url} alt="" className="h-6 w-6 rounded-full object-cover" />
  ) : (
    <div aria-hidden="true" className="h-6 w-6 rounded-full bg-muted flex items-center justify-center text-xs font-bold text-muted-foreground">
      {athlete.name.charAt(0).toUpperCase()}
    </div>
  )
}

function TierBadge({ tier }: { tier: string | null | undefined }) {
  return tier
    ? <Badge variant={tierVariant[tier] ?? 'secondary'}>{tierLabel(tier)}</Badge>
    : <span className="text-muted-foreground">—</span>
}

function Streak({ weeks }: { weeks: number }) {
  if (weeks <= 0) return <span className="text-muted-foreground">—</span>
  return (
    <span className="inline-flex items-center gap-1">
      <span aria-hidden="true">🔥</span>{weeks}w<span className="sr-only"> streak</span>
    </span>
  )
}

export function AthletesList({ user }: { user: User }) {
  const [search, setSearch] = useState('')
  const friendlyDate = useFriendlyDate()
  const canManage = user.is_coach || user.is_admin

  const query = useQuery({
    queryKey: ['athletes'],
    queryFn: () => api.listAthletes(),
  })

  return (
    <div>
      <PageHeader
        title="Athletes"
        actions={canManage && (
          <Button render={<Link to="/athletes/new" />}>
            <Plus aria-hidden="true" /> New Athlete
          </Button>
        )}
      />

      <QueryState
        query={query}
        resource="athletes"
        empty={{
          icon: '🏋️',
          title: 'No athletes yet',
          description: canManage ? 'Add an athlete to start logging their training.' : undefined,
          action: canManage ? 'New Athlete' : undefined,
          actionTo: canManage ? '/athletes/new' : undefined,
        }}
      >
        {athletes => {
          const filtered = athletes.filter(a => a.name.toLowerCase().includes(search.toLowerCase()))
          return (
            <>
              <Input
                type="search"
                aria-label="Search athletes"
                value={search}
                onChange={e => setSearch(e.target.value)}
                placeholder="Search athletes..."
                className="mb-4"
              />
              {filtered.length === 0 ? (
                <EmptyState icon="🔍" title="No athletes match your search" />
              ) : (
                <>
                  <CardList label="Athletes">
                    {filtered.map(a => (
                      <CardListItem
                        key={a.id}
                        to={`/athletes/${a.id}`}
                        title={<span className="flex items-center gap-2"><AthleteAvatar athlete={a} />{a.name}</span>}
                        subtitle={a.last_workout_date ? `Last workout ${friendlyDate(a.last_workout_date)}` : 'No workouts yet'}
                        trailing={<div className="flex flex-col items-end gap-1"><TierBadge tier={a.tier} /><Streak weeks={a.week_streak} /></div>}
                      />
                    ))}
                  </CardList>
                  <DesktopOnly>
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
                          <TableRow key={athlete.id} className="relative">
                            <TableCell>
                              <div className="flex items-center gap-2">
                                <AthleteAvatar athlete={athlete} />
                                <RowLink to={`/athletes/${athlete.id}`}>{athlete.name}</RowLink>
                              </div>
                            </TableCell>
                            <TableCell><TierBadge tier={athlete.tier} /></TableCell>
                            <TableCell>{athlete.active_assignments}</TableCell>
                            <TableCell className="text-muted-foreground">{friendlyDate(athlete.last_workout_date)}</TableCell>
                            <TableCell><Streak weeks={athlete.week_streak} /></TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </DesktopOnly>
                </>
              )}
            </>
          )
        }}
      </QueryState>
    </div>
  )
}
