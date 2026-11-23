import { useState } from 'react'
import { Link, useParams, useSearchParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { ChevronLeft, ChevronRight } from 'lucide-react'
import { api } from '@/api/client'
import type { CalendarData, CalendarDay, CalendarPhase } from '@/api/types'
import { PageHeader } from '@/components/PageHeader'
import { QueryState } from '@/components/QueryState'
import { Button } from '@/components/ui/button'
import {
  DISCIPLINE_LABEL,
  PHASE_LABEL,
  dayTone,
  humanize,
  monthGrid,
  parseMonth,
  phasesOn,
  shiftMonth,
  type DayTone,
} from '@/lib/calendar'
import { useFriendlyDate } from '@/lib/dates'
import { cn } from '@/lib/utils'

const WEEKDAYS = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun']

const TONE: Record<DayTone, { label: string; dot: string }> = {
  done: { label: 'Done', dot: 'bg-success' },
  partial: { label: 'Partial', dot: 'bg-warning' },
  missed: { label: 'Missed', dot: 'bg-destructive' },
  upcoming: { label: 'Scheduled', dot: 'border border-primary bg-transparent' },
  logged: { label: 'Logged', dot: 'bg-primary' },
  none: { label: '', dot: '' },
}

const PHASE_BAR: Record<string, string> = {
  off: 'bg-muted-foreground/40',
  pre: 'bg-warning',
  in: 'bg-success',
}

const SESSION_PATH: Record<string, string> = {
  throwing: 'throwing-sessions',
  conditioning: 'conditioning-sessions',
  skill: 'skill-sessions',
  recovery: 'recovery-checkins',
}

function sessionHref(athleteId: number, discipline: string, workoutId: number): string {
  if (discipline === 'resistance') return `/athletes/${athleteId}/workouts/${workoutId}`
  return `/athletes/${athleteId}/${SESSION_PATH[discipline] ?? 'workouts'}`
}

function phaseName(p: CalendarPhase): string {
  const label = PHASE_LABEL[p.phase] ?? humanize(p.phase)
  return p.sport ? `${humanize(p.sport)} ${label.toLowerCase()}` : label
}

function DayCell({ date, day, phases, isToday, selected, onSelect, label }: {
  date: string
  day: CalendarDay | undefined
  phases: CalendarPhase[]
  isToday: boolean
  selected: boolean
  onSelect: () => void
  label: string
}) {
  const tone = day ? dayTone(day) : 'none'
  const sessionCount = day?.sessions.length ?? 0
  const summary = [TONE[tone].label, sessionCount > 0 ? `${sessionCount} session${sessionCount === 1 ? '' : 's'}` : '', ...phases.map(phaseName)]
    .filter(Boolean).join(', ')
  return (
    <button
      type="button"
      onClick={onSelect}
      aria-pressed={selected}
      aria-label={`${label}${summary ? `: ${summary}` : ''}`}
      className={cn(
        'relative flex min-h-14 w-full flex-col items-start overflow-hidden rounded-md border p-1 text-left text-xs transition-colors md:min-h-20 md:p-1.5',
        'focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-ring/50',
        selected ? 'border-primary bg-primary/5' : 'border-border bg-card hover:border-primary/50',
      )}
    >
      {phases.length > 0 && (
        <span className={cn('absolute inset-x-0 top-0 h-1', PHASE_BAR[phases[0].phase] ?? 'bg-muted')} aria-hidden="true" />
      )}
      <span className={cn('mt-0.5 font-medium tabular-nums', isToday && 'rounded-full bg-primary px-1.5 text-primary-foreground')}>
        {Number(date.slice(8))}
      </span>
      {tone !== 'none' && (
        <span className="mt-1 flex items-center gap-1" aria-hidden="true">
          <span className={cn('inline-block size-2 rounded-full', TONE[tone].dot)} />
          <span className="hidden truncate text-muted-foreground md:inline">{TONE[tone].label}</span>
        </span>
      )}
      {sessionCount > 0 && (
        <span className="mt-auto hidden w-full truncate text-muted-foreground md:block" aria-hidden="true">
          {day!.sessions.map(s => DISCIPLINE_LABEL[s.discipline] ?? humanize(s.discipline)).join(' · ')}
        </span>
      )}
    </button>
  )
}

function DayDetail({ athleteId, day, phases, title }: { athleteId: number; day: CalendarDay | undefined; phases: CalendarPhase[]; title: string }) {
  return (
    <section aria-live="polite" className="mt-4 rounded-lg border border-border bg-card p-4">
      <h2 className="mb-2 font-semibold">{title}</h2>
      <ul className="space-y-2 text-sm">
        {phases.map(p => (
          <li key={`phase-${p.id}`} className="text-muted-foreground">{phaseName(p)}</li>
        ))}
        {day?.scheduled && (
          <li>
            Scheduled: <span className="font-medium">{day.program_name ?? 'Program day'}</span>
            {day.status && <span className="text-muted-foreground"> · {TONE[dayTone(day)].label}</span>}
          </li>
        )}
        {day?.sessions.map(s => (
          <li key={s.workout_id}>
            <Link to={sessionHref(athleteId, s.discipline, s.workout_id)} className="font-medium text-primary hover:underline">
              {DISCIPLINE_LABEL[s.discipline] ?? humanize(s.discipline)}
            </Link>
            {s.label && <span className="text-muted-foreground"> · {humanize(s.label)}</span>}
            {s.discipline === 'resistance' && <span className="text-muted-foreground"> · {s.set_count} set{s.set_count === 1 ? '' : 's'}</span>}
          </li>
        ))}
        {!day?.scheduled && (day?.sessions.length ?? 0) === 0 && phases.length === 0 && (
          <li className="text-muted-foreground">Nothing scheduled or logged.</li>
        )}
      </ul>
    </section>
  )
}

function MonthView({ athleteId, data, label }: { athleteId: number; data: CalendarData; label: (iso: string) => string }) {
  const byDate = new Map(data.days.map(d => [d.date, d]))
  const [picked, setPicked] = useState<string | null>(null)
  const selected = picked && picked.startsWith(data.month) ? picked : (data.today.startsWith(data.month) ? data.today : data.start)
  const weeks = monthGrid(data.month)
  return (
    <>
      <div aria-label="Month" className="grid grid-cols-7 gap-1">
        {WEEKDAYS.map(d => (
          <div key={d} className="pb-1 text-center text-xs font-medium text-muted-foreground">
            <span className="md:hidden">{d.charAt(0)}</span>
            <span className="hidden md:inline">{d}</span>
          </div>
        ))}
        {weeks.flat().map((date, i) =>
          date ? (
            <div key={date}>
              <DayCell
                date={date}
                day={byDate.get(date)}
                phases={phasesOn(date, data.phases)}
                isToday={date === data.today}
                selected={date === selected}
                onSelect={() => setPicked(date)}
                label={label(date)}
              />
            </div>
          ) : (
            <div key={`pad-${i}`} aria-hidden="true" />
          ),
        )}
      </div>

      <ul className="mt-3 flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground" aria-label="Legend">
        {(['done', 'partial', 'missed', 'upcoming', 'logged'] as DayTone[]).map(t => (
          <li key={t} className="flex items-center gap-1.5">
            <span className={cn('inline-block size-2 rounded-full', TONE[t].dot)} aria-hidden="true" />
            {t === 'logged' ? 'Other session' : TONE[t].label}
          </li>
        ))}
        {Object.entries(PHASE_BAR).map(([phase, cls]) => (
          <li key={phase} className="flex items-center gap-1.5">
            <span className={cn('inline-block h-1 w-4 rounded', cls)} aria-hidden="true" />
            {PHASE_LABEL[phase]}
          </li>
        ))}
      </ul>

      <DayDetail athleteId={athleteId} day={byDate.get(selected)} phases={phasesOn(selected, data.phases)} title={label(selected)} />
    </>
  )
}

/**
 * Month view of an athlete's scheduled program days, completed sessions,
 * other session types and season phases. Read only.
 */
export function CalendarPage() {
  const { id } = useParams<{ id: string }>()
  const athleteId = Number(id)
  const [params, setParams] = useSearchParams()
  const month = parseMonth(params.get('month'))
  const friendlyDate = useFriendlyDate()
  const { data: athlete } = useQuery({
    queryKey: ['athlete', athleteId],
    queryFn: () => api.getAthlete(athleteId),
    enabled: !isNaN(athleteId),
  })
  const query = useQuery({
    queryKey: ['athlete-calendar', athleteId, month ?? 'current'],
    queryFn: () => api.getAthleteCalendar(athleteId, month ?? undefined),
    enabled: !isNaN(athleteId),
  })
  const shown = query.data?.month ?? month
  const goTo = (m: string | null) => {
    const next = new URLSearchParams(params)
    if (m) next.set('month', m)
    else next.delete('month')
    setParams(next, { replace: true })
  }
  const monthTitle = shown
    ? new Date(`${shown}-01T12:00:00`).toLocaleDateString(undefined, { month: 'long', year: 'numeric' })
    : 'Calendar'

  return (
    <div className="max-w-4xl">
      <PageHeader
        title={monthTitle}
        pageTitle={athlete ? `Calendar · ${athlete.name}` : 'Calendar'}
        back={{ to: `/athletes/${athleteId}`, label: athlete?.name ?? 'Athlete' }}
        description="Scheduled program days, logged sessions and season phases."
        actions={
          <>
            <Button variant="outline" size="icon" aria-label="Previous month" disabled={!shown} onClick={() => shown && goTo(shiftMonth(shown, -1))}>
              <ChevronLeft aria-hidden="true" />
            </Button>
            <Button variant="outline" onClick={() => goTo(null)} disabled={!month}>Today</Button>
            <Button variant="outline" size="icon" aria-label="Next month" disabled={!shown} onClick={() => shown && goTo(shiftMonth(shown, 1))}>
              <ChevronRight aria-hidden="true" />
            </Button>
          </>
        }
      />
      <QueryState query={query} resource="the calendar" skeleton="cards">
        {data => (
          <MonthView
            key={data.month}
            athleteId={athleteId}
            data={data}
            label={iso => friendlyDate(iso, { absolute: true })}
          />
        )}
      </QueryState>
    </div>
  )
}
