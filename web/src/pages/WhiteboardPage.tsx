import { useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { ArrowLeft, Check, Maximize, Minimize, Users } from 'lucide-react'
import { api } from '@/api/client'
import type { WhiteboardAthlete, WhiteboardLine } from '@/api/types'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { useFriendlyDate } from '@/lib/dates'
import { normalizeUnit } from '@/lib/plates'
import { usePageTitle } from '@/lib/usePageTitle'
import { useWakeLock } from '@/lib/useWakeLock'
import { cn } from '@/lib/utils'
import { groupKind } from '@/lib/groups'
import {
  boardBlocks, lineStatus, loggedChip, parseAthleteIds, prescribedSummary, progressPercent, toggleAthlete,
} from '@/lib/whiteboard'

/** Refresh cadence: a set logged on a phone shows up on the TV within this. */
const REFRESH_MS = 20_000

function useClock(): Date {
  const [now, setNow] = useState(() => new Date())
  useEffect(() => {
    const t = setInterval(() => setNow(new Date()), 15_000)
    return () => clearInterval(t)
  }, [])
  return now
}

function useFullscreen(): [boolean, () => void] {
  const [on, setOn] = useState(() => typeof document !== 'undefined' && !!document.fullscreenElement)
  useEffect(() => {
    const onChange = () => setOn(!!document.fullscreenElement)
    document.addEventListener('fullscreenchange', onChange)
    return () => document.removeEventListener('fullscreenchange', onChange)
  }, [])
  const toggle = () => {
    if (document.fullscreenElement) void document.exitFullscreen?.().catch(() => {})
    else void document.documentElement.requestFullscreen?.().catch(() => {})
  }
  return [on, toggle]
}

const STATUS_STYLES = {
  done: 'border-success/40 bg-success/5',
  active: 'border-primary/50 bg-primary/5',
  todo: 'border-border',
  extra: 'border-dashed border-border',
} as const

function BoardLine({ line, label, unit }: { line: WhiteboardLine; label: string | null; unit: string }) {
  const status = lineStatus(line)
  const { runs, warmups } = prescribedSummary(line.prescribed, unit)
  return (
    <li className={cn('rounded-xl border-2 p-4', STATUS_STYLES[status])}>
      <div className="flex items-start gap-3">
        {label && (
          <span className="shrink-0 rounded-md bg-primary px-2 py-0.5 text-xl font-bold text-primary-foreground">{label}</span>
        )}
        <h3 className="min-w-0 flex-1 text-2xl font-semibold leading-tight lg:text-3xl">{line.exercise_name}</h3>
        {status === 'done' ? (
          <span className="inline-flex shrink-0 items-center gap-1 text-xl font-semibold text-success-strong">
            <Check className="size-7" aria-hidden="true" /> Done
          </span>
        ) : status !== 'extra' && (
          <span className="shrink-0 text-xl font-semibold tabular-nums text-muted-foreground">
            {line.working_done}/{line.working_target}
          </span>
        )}
      </div>
      {runs.length > 0 && (
        <p className="mt-2 text-xl text-foreground/90 lg:text-2xl">
          {runs.join(' · ')}
          {warmups > 0 && <span className="text-muted-foreground"> · +{warmups} warm-up{warmups === 1 ? '' : 's'}</span>}
        </p>
      )}
      {status === 'extra' && <p className="mt-1 text-lg text-muted-foreground">Not in today's program</p>}
      {line.logged.length > 0 && (
        <ul className="mt-3 flex flex-wrap gap-2" aria-label={`${line.exercise_name} sets logged`}>
          {line.logged.map(s => {
            const chip = loggedChip(s)
            return (
              <li key={s.id} className={cn(
                'rounded-lg px-3 py-1 text-xl font-medium tabular-nums lg:text-2xl',
                chip.warmup ? 'bg-muted text-muted-foreground' : 'bg-secondary text-secondary-foreground',
              )}>
                {chip.warmup && <span className="sr-only">Warm-up </span>}
                {chip.text}
              </li>
            )
          })}
        </ul>
      )}
    </li>
  )
}

function AthleteColumn({ athlete, unit }: { athlete: WhiteboardAthlete; unit: string }) {
  const pct = progressPercent(athlete.working_done, athlete.working_target)
  const blocks = boardBlocks(athlete.lines)
  return (
    <section aria-labelledby={`wb-${athlete.athlete_id}`} className="flex min-w-0 flex-col gap-4 rounded-2xl border border-border bg-card p-5">
      <header>
        <h2 id={`wb-${athlete.athlete_id}`} className="text-4xl font-bold tracking-tight lg:text-5xl">{athlete.name}</h2>
        {athlete.program_name && (
          <p className="mt-1 text-xl text-muted-foreground">
            {athlete.program_name}
            {athlete.current_week ? ` · Week ${athlete.current_week}, Day ${athlete.current_day}` : ''}
          </p>
        )}
        {athlete.working_target > 0 && (
          <div className="mt-3">
            <div className="h-3 overflow-hidden rounded-full bg-muted" role="progressbar" aria-valuemin={0} aria-valuemax={100} aria-valuenow={pct} aria-label={`${athlete.name} progress`}>
              <div className="h-full rounded-full bg-primary transition-[width] duration-500" style={{ width: `${pct}%` }} />
            </div>
            <p className="mt-1 text-lg tabular-nums text-muted-foreground">
              {athlete.working_done} of {athlete.working_target} working sets
            </p>
          </div>
        )}
      </header>
      {athlete.lines.length === 0 ? (
        <p className="text-2xl text-muted-foreground">No session on the board today.</p>
      ) : (
        <div className="flex flex-col gap-3">
          {blocks.map((b, i) => b.key ? (
            <div key={`g-${b.key}`} className="rounded-2xl border-2 border-primary/30 p-2">
              <p className="px-2 pb-2 text-lg font-semibold uppercase tracking-wide text-primary">
                {groupKind(b.lines.length)} {b.key} · alternate, rest after each round
              </p>
              <ul className="flex flex-col gap-2">
                {b.lines.map(({ line, label }) => <BoardLine key={line.exercise_id} line={line} label={label} unit={unit} />)}
              </ul>
            </div>
          ) : (
            <ul key={`l-${i}`} className="flex flex-col">
              {b.lines.map(({ line }) => <BoardLine key={line.exercise_id} line={line} label={null} unit={unit} />)}
            </ul>
          ))}
        </div>
      )}
    </section>
  )
}

function columnsClass(n: number): string {
  if (n <= 1) return 'grid-cols-1'
  if (n === 2) return 'grid-cols-1 md:grid-cols-2'
  if (n === 3) return 'grid-cols-1 md:grid-cols-2 xl:grid-cols-3'
  return 'grid-cols-1 md:grid-cols-2 2xl:grid-cols-4'
}

/**
 * Whiteboard / TV mode: today's session for one or more athletes in large
 * type, for a garage TV or tablet. Read-only — logging happens on the
 * athletes' phones and shows up here on the next refresh.
 */
export function WhiteboardPage() {
  usePageTitle('Whiteboard')
  useWakeLock()
  const [params, setParams] = useSearchParams()
  const ids = parseAthleteIds(params.get('athletes'))
  const [picking, setPicking] = useState(false)
  const [fullscreen, toggleFullscreen] = useFullscreen()
  const friendlyDate = useFriendlyDate()
  const now = useClock()
  const { data: prefs } = useQuery({ queryKey: ['preferences'], queryFn: () => api.getPreferences(), staleTime: 10 * 60_000 })
  const unit = normalizeUnit(prefs?.weight_unit) === 'kg' ? 'kg' : 'lb'

  const board = useQuery({
    queryKey: ['whiteboard', ids.join(',')],
    queryFn: () => api.getWhiteboard(ids),
    refetchInterval: REFRESH_MS,
    refetchIntervalInBackground: false,
  })
  const data = board.data
  const shown = new Set((data?.athletes ?? []).map(a => a.athlete_id))

  function setIds(next: number[]) {
    const p = new URLSearchParams(params)
    if (next.length) p.set('athletes', next.join(','))
    else p.delete('athletes')
    setParams(p, { replace: true })
  }

  return (
    <div className="min-h-dvh bg-background p-4 pt-[calc(1rem+env(safe-area-inset-top))] text-foreground lg:p-8">
      <header className="mb-6 flex flex-wrap items-center gap-3">
        <Button variant="ghost" size="icon-touch" render={<Link to="/" />} aria-label="Leave whiteboard">
          <ArrowLeft aria-hidden="true" />
        </Button>
        <div className="min-w-0 flex-1">
          <h1 className="text-3xl font-bold lg:text-4xl">{data ? friendlyDate(data.date) : 'Today'}</h1>
          <p className="text-xl tabular-nums text-muted-foreground">
            {now.toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' })}
            {board.isFetching && !board.isLoading ? ' · updating' : ''}
            {board.isError && data ? ' · offline, showing last update' : ''}
          </p>
        </div>
        {data && data.available.length > 1 && (
          <Button variant="outline" size="touch" onClick={() => setPicking(p => !p)} aria-expanded={picking} aria-controls="wb-picker">
            <Users aria-hidden="true" /> Athletes
          </Button>
        )}
        <Button variant="outline" size="icon-touch" onClick={toggleFullscreen} aria-label={fullscreen ? 'Exit full screen' : 'Full screen'}>
          {fullscreen ? <Minimize aria-hidden="true" /> : <Maximize aria-hidden="true" />}
        </Button>
      </header>

      {picking && data && (
        <fieldset id="wb-picker" className="mb-6 rounded-xl border border-border p-4">
          <legend className="px-1 text-sm font-medium">Show on the board (up to 8)</legend>
          <div className="flex flex-wrap gap-2">
            {data.available.map(a => {
              const on = ids.length ? ids.includes(a.id) : shown.has(a.id)
              return (
                <Button key={a.id} variant={on ? 'default' : 'outline'} size="touch" aria-pressed={on}
                  onClick={() => setIds(toggleAthlete(ids.length ? ids : [...shown], a.id))}>
                  {on && <Check aria-hidden="true" />} {a.name}
                </Button>
              )
            })}
          </div>
        </fieldset>
      )}

      {board.isLoading ? (
        <div className="grid grid-cols-1 gap-6 md:grid-cols-2">
          <Skeleton className="h-96 rounded-2xl" />
          <Skeleton className="h-96 rounded-2xl" />
        </div>
      ) : board.isError && !data ? (
        <div className="rounded-2xl border border-border p-8 text-center">
          <p className="text-2xl">Couldn't load the board.</p>
          <Button size="touch" className="mt-4" onClick={() => void board.refetch()}>Retry</Button>
        </div>
      ) : data && data.athletes.length === 0 ? (
        <p className="text-2xl text-muted-foreground">No athletes on the board. Pick some with Athletes.</p>
      ) : data && (
        <div className={cn('grid gap-6', columnsClass(data.athletes.length))}>
          {data.athletes.map(a => <AthleteColumn key={a.athlete_id} athlete={a} unit={unit} />)}
        </div>
      )}
      {data?.truncated && (
        <p className="mt-4 text-lg text-muted-foreground">Showing the first 8 athletes. Use Athletes to choose.</p>
      )}
    </div>
  )
}
