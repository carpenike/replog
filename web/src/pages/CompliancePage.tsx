import { Link, useSearchParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { Check, ChevronLeft, ChevronRight, Circle, Minus, Plus, X } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { api } from '@/api/client'
import type { ComplianceAthlete, ComplianceDay, ComplianceStatus } from '@/api/types'
import { PageHeader } from '@/components/PageHeader'
import { QueryState } from '@/components/QueryState'
import { Button } from '@/components/ui/button'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { addDays, COMPLIANCE_LABEL, complianceTotals, parseWeekStart } from '@/lib/calendar'
import { useFriendlyDate } from '@/lib/dates'
import { cn } from '@/lib/utils'

const WEEKDAYS = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun']

const STATUS_STYLE: Record<ComplianceStatus, { icon: LucideIcon | null; className: string }> = {
  done: { icon: Check, className: 'bg-success/15 text-success-strong' },
  partial: { icon: Minus, className: 'bg-warning/20 text-warning-strong' },
  skipped: { icon: X, className: 'bg-destructive/10 text-destructive' },
  prescribed: { icon: Circle, className: 'border border-dashed border-border text-muted-foreground' },
  extra: { icon: Plus, className: 'bg-primary/10 text-primary' },
  rest: { icon: null, className: 'text-muted-foreground' },
}

function dayHref(athleteId: number, day: ComplianceDay): string | null {
  return day.session_id ? `/athletes/${athleteId}/workouts/${day.session_id}` : null
}

function StatusCell({ athlete, day, dayLabel, compact }: { athlete: ComplianceAthlete; day: ComplianceDay; dayLabel: string; compact?: boolean }) {
  const style = STATUS_STYLE[day.status] ?? STATUS_STYLE.rest
  const Icon = style.icon
  const label = COMPLIANCE_LABEL[day.status] ?? day.status
  const detail = day.program_day_label ?? day.program_name ?? ''
  const aria = `${athlete.name}, ${dayLabel}: ${label}${detail ? ` (${detail})` : ''}`
  const inner = (
    <span
      className={cn(
        'flex flex-col items-center justify-center gap-0.5 rounded-md text-xs font-medium',
        compact ? 'h-11 w-full' : 'h-12 min-w-16 px-2',
        style.className,
      )}
    >
      {Icon ? <Icon className="size-4" aria-hidden="true" /> : <span aria-hidden="true">·</span>}
      {!compact && <span>{label}</span>}
    </span>
  )
  const href = dayHref(athlete.athlete_id, day)
  if (href) {
    return (
      <Link to={href} aria-label={aria} title={aria} className="block rounded-md focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-ring/50">
        {inner}
      </Link>
    )
  }
  return <span role="img" aria-label={aria} title={aria} className="block">{inner}</span>
}

function Legend() {
  return (
    <ul className="mb-4 flex flex-wrap gap-x-4 gap-y-2 text-xs text-muted-foreground" aria-label="Legend">
      {(['done', 'partial', 'skipped', 'prescribed', 'extra', 'rest'] as ComplianceStatus[]).map(s => {
        const Icon = STATUS_STYLE[s].icon
        return (
          <li key={s} className="flex items-center gap-1.5">
            <span className={cn('inline-flex size-6 items-center justify-center rounded', STATUS_STYLE[s].className)} aria-hidden="true">
              {Icon ? <Icon className="size-3.5" /> : '·'}
            </span>
            {COMPLIANCE_LABEL[s]}
          </li>
        )
      })}
    </ul>
  )
}

/**
 * Coach view: every coached athlete by day for one week, from the week
 * plan-versus-done read. It reports; the coach decides what to do.
 */
export function CompliancePage() {
  const [params, setParams] = useSearchParams()
  const week = parseWeekStart(params.get('week'))
  const friendlyDate = useFriendlyDate()
  const query = useQuery({
    queryKey: ['coach-compliance', week ?? 'current'],
    queryFn: () => api.getCoachCompliance(week ?? undefined),
  })
  const weekStart = query.data?.week_start ?? week
  const goTo = (start: string | null) => {
    const next = new URLSearchParams(params)
    if (start) next.set('week', start)
    else next.delete('week')
    setParams(next, { replace: true })
  }

  return (
    <div className="max-w-5xl">
      <PageHeader
        title="Compliance"
        description="Planned versus done by day. Read only: nothing here changes a plan."
        actions={
          <>
            <Button variant="outline" size="icon" aria-label="Previous week" disabled={!weekStart} onClick={() => weekStart && goTo(addDays(weekStart, -7))}>
              <ChevronLeft aria-hidden="true" />
            </Button>
            <Button variant="outline" onClick={() => goTo(null)} disabled={!week}>This week</Button>
            <Button variant="outline" size="icon" aria-label="Next week" disabled={!weekStart} onClick={() => weekStart && goTo(addDays(weekStart, 7))}>
              <ChevronRight aria-hidden="true" />
            </Button>
          </>
        }
      />

      <QueryState
        query={query}
        resource="the compliance grid"
        skeleton="list"
        isEmpty={d => d.athletes.length === 0}
        empty={{ title: 'No athletes yet', description: 'Athletes you coach show up here once they are assigned to you.', action: 'Go to athletes', actionTo: '/athletes' }}
      >
        {data => {
          const totals = complianceTotals(data.athletes)
          const dayLabels = Array.from({ length: 7 }, (_, i) => friendlyDate(addDays(data.week_start, i), { absolute: true }))
          return (
            <>
              <p className="mb-3 text-sm">
                <span className="font-medium">
                  {friendlyDate(data.week_start, { absolute: true })} – {friendlyDate(data.week_end, { absolute: true })}
                </span>
                <span className="text-muted-foreground">
                  {' '}· {totals.done} of {totals.planned} planned sessions done
                  {totals.partial > 0 && `, ${totals.partial} partial`}
                  {totals.skipped > 0 && `, ${totals.skipped} skipped`}
                  {totals.extra > 0 && `, ${totals.extra} extra`}
                </span>
              </p>
              <Legend />

              {/* Desktop: athletes by day */}
              <div className="hidden overflow-x-auto md:block">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Athlete</TableHead>
                      {WEEKDAYS.map((d, i) => (
                        <TableHead key={d} className="text-center">
                          <abbr title={dayLabels[i]} className="no-underline">{d}</abbr>
                        </TableHead>
                      ))}
                      <TableHead className="text-right">Done</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {data.athletes.map(a => (
                      <TableRow key={a.athlete_id}>
                        <TableCell>
                          <Link to={`/athletes/${a.athlete_id}`} className="font-medium hover:underline">{a.name}</Link>
                        </TableCell>
                        {a.days.map((d, i) => (
                          <TableCell key={d.date} className="p-1 text-center">
                            <StatusCell athlete={a} day={d} dayLabel={dayLabels[i]} />
                          </TableCell>
                        ))}
                        <TableCell className="text-right tabular-nums">{a.planned_count > 0 ? a.summary : '—'}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>

              {/* Phones: one card per athlete with a seven-day strip */}
              <ul className="space-y-3 md:hidden" aria-label="Athletes">
                {data.athletes.map(a => (
                  <li key={a.athlete_id} className="rounded-lg border border-border bg-card p-3">
                    <div className="mb-2 flex items-center justify-between gap-2">
                      <Link to={`/athletes/${a.athlete_id}`} className="font-medium hover:underline">{a.name}</Link>
                      <span className="text-sm text-muted-foreground tabular-nums">{a.planned_count > 0 ? `${a.summary} done` : 'Nothing planned'}</span>
                    </div>
                    <div className="grid grid-cols-7 gap-1">
                      {a.days.map((d, i) => (
                        <div key={d.date} className="text-center">
                          <div className="mb-1 text-[0.7rem] text-muted-foreground" aria-hidden="true">{WEEKDAYS[i].charAt(0)}</div>
                          <StatusCell athlete={a} day={d} dayLabel={dayLabels[i]} compact />
                        </div>
                      ))}
                    </div>
                  </li>
                ))}
              </ul>
            </>
          )
        }}
      </QueryState>
    </div>
  )
}
