import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Award, CalendarCheck, Sparkles } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { api } from '@/api/client'
import type { MilestoneData, MilestoneKind } from '@/api/types'
import { Card, CardAction, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { useFriendlyDate } from '@/lib/dates'
import { featuredMilestones, goalPercent, goalRemaining, isNewMilestone } from '@/lib/milestones'
import { cn, localDateISO } from '@/lib/utils'

const KIND_ICON: Record<MilestoneKind, LucideIcon> = {
  tier: Award,
  first: Sparkles,
  consistency: CalendarCheck,
}

const KIND_LABEL: Record<MilestoneKind, string> = {
  tier: 'Tier',
  first: 'First',
  consistency: 'Consistency',
}

const KIND_TINT: Record<MilestoneKind, string> = {
  tier: 'bg-primary/10 text-primary',
  first: 'bg-warning/15 text-warning-strong',
  consistency: 'bg-success/15 text-success-strong',
}

function MilestoneBadge({ m, today, date }: { m: MilestoneData; today: string; date: (iso: string) => string }) {
  const Icon = KIND_ICON[m.kind] ?? Sparkles
  const fresh = isNewMilestone(m, today)
  return (
    <li className="flex items-start gap-3 rounded-lg border border-border bg-card p-3">
      <span className={cn('flex size-9 shrink-0 items-center justify-center rounded-full', KIND_TINT[m.kind])}>
        <Icon className="size-4" aria-hidden="true" />
      </span>
      <div className="min-w-0 flex-1">
        <p className="text-sm font-medium">
          {m.title}
          {fresh && <span className="ml-2 rounded-full bg-primary px-1.5 py-0.5 text-[10px] font-semibold uppercase text-primary-foreground">New</span>}
        </p>
        <p className="text-xs text-muted-foreground">
          <span className="sr-only">{KIND_LABEL[m.kind]} milestone. </span>
          {m.detail} {date(m.achieved_on)}
        </p>
      </div>
    </li>
  )
}

/**
 * Tier badges, firsts and consistency milestones for one athlete, plus the
 * next goals. Everything is measured against the athlete's own history;
 * there is no comparison with other athletes.
 */
export function MilestonesCard({ athleteId, limit = 4, className }: { athleteId: number; limit?: number; className?: string }) {
  const [showAll, setShowAll] = useState(false)
  const friendlyDate = useFriendlyDate()
  const date = (iso: string) => friendlyDate(iso)
  const today = localDateISO()
  const { data, isLoading, isError } = useQuery({
    queryKey: ['milestones', athleteId],
    queryFn: () => api.getAthleteMilestones(athleteId),
    enabled: !isNaN(athleteId),
    staleTime: 5 * 60_000,
  })

  if (isLoading) return <Skeleton className={cn('h-40 w-full', className)} />
  // Milestones are a bonus; a failed read shouldn't take the page down.
  if (isError || !data) return null

  const shown = showAll ? data.achieved : featuredMilestones(data.achieved, limit)
  const hidden = data.achieved.length - shown.length

  return (
    <Card size="sm" className={className}>
      <CardHeader>
        <CardTitle>Milestones</CardTitle>
        <CardAction>
          <span className="text-xs text-muted-foreground">Your own history</span>
        </CardAction>
      </CardHeader>
      <CardContent className="space-y-4">
        <dl className="grid grid-cols-4 gap-2 text-center">
          {[
            ['Sessions', data.total_sessions],
            ['Week streak', data.current_streak],
            ['Best streak', data.best_streak],
            ['PRs', data.pr_count],
          ].map(([label, value]) => (
            <div key={label} className="rounded-md bg-muted/50 px-1 py-2">
              <dt className="text-[11px] text-muted-foreground">{label}</dt>
              <dd className="text-lg font-bold tabular-nums">{value}</dd>
            </div>
          ))}
        </dl>

        {data.next.length > 0 && (
          <div>
            <h3 className="mb-2 text-xs font-medium uppercase tracking-wider text-muted-foreground">Up next</h3>
            <ul className="space-y-2">
              {data.next.map(goal => {
                const pct = goalPercent(goal)
                return (
                  <li key={goal.key}>
                    <div className="flex justify-between text-sm">
                      <span>{goal.title}</span>
                      <span className="text-xs text-muted-foreground tabular-nums">{goal.current}/{goal.target} · {goalRemaining(goal)}</span>
                    </div>
                    <div
                      role="progressbar"
                      aria-label={goal.title}
                      aria-valuemin={0}
                      aria-valuemax={goal.target}
                      aria-valuenow={Math.min(goal.current, goal.target)}
                      className="mt-1 h-1.5 overflow-hidden rounded-full bg-muted"
                    >
                      <div className="h-full rounded-full bg-primary" style={{ width: `${pct}%` }} />
                    </div>
                  </li>
                )
              })}
            </ul>
          </div>
        )}

        {data.achieved.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            Log your first session to earn your first milestone <span aria-hidden="true">💪</span>
          </p>
        ) : (
          <div>
            <h3 className="mb-2 text-xs font-medium uppercase tracking-wider text-muted-foreground">Earned</h3>
            <ul className="grid gap-2 sm:grid-cols-2">
              {shown.map(m => <MilestoneBadge key={m.key} m={m} today={today} date={date} />)}
            </ul>
            {(hidden > 0 || showAll) && (
              <Button variant="ghost" size="sm" className="mt-2" aria-expanded={showAll} onClick={() => setShowAll(!showAll)}>
                {showAll ? 'Show fewer' : `Show all ${data.achieved.length}`}
              </Button>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  )
}
