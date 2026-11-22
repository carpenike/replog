// Date helpers for the calendar month view and the compliance week grid.
// All math runs on YYYY-MM-DD strings in UTC so a browser timezone never
// shifts a day; the server already resolved "today" in the user's zone.
import type { CalendarDay, CalendarPhase, ComplianceAthlete, ComplianceStatus } from '@/api/types'

function parse(date: string): Date {
  const [y, m, d] = date.split('-').map(Number)
  return new Date(Date.UTC(y, m - 1, d ?? 1))
}

function fmt(d: Date): string {
  return d.toISOString().slice(0, 10)
}

/** Add days to a YYYY-MM-DD date. */
export function addDays(date: string, days: number): string {
  const d = parse(date)
  d.setUTCDate(d.getUTCDate() + days)
  return fmt(d)
}

/** The Monday on or before date. */
export function mondayOf(date: string): string {
  const d = parse(date)
  const offset = (d.getUTCDay() + 6) % 7
  return addDays(date, -offset)
}

/** YYYY-MM shifted by delta months. */
export function shiftMonth(month: string, delta: number): string {
  const [y, m] = month.split('-').map(Number)
  const d = new Date(Date.UTC(y, m - 1 + delta, 1))
  return fmt(d).slice(0, 7)
}

/** A valid YYYY-MM, or null. */
export function parseMonth(raw: string | null | undefined): string | null {
  return raw && /^\d{4}-(0[1-9]|1[0-2])$/.test(raw) ? raw : null
}

/** A valid Monday YYYY-MM-DD, or null. */
export function parseWeekStart(raw: string | null | undefined): string | null {
  if (!raw || !/^\d{4}-\d{2}-\d{2}$/.test(raw)) return null
  const d = parse(raw)
  if (Number.isNaN(d.getTime()) || fmt(d) !== raw) return null
  return d.getUTCDay() === 1 ? raw : null
}

/**
 * The month as weeks of seven dates, Monday first. Days outside the month
 * are null so the grid keeps its columns.
 */
export function monthGrid(month: string): (string | null)[][] {
  const first = `${month}-01`
  const last = addDays(`${shiftMonth(month, 1)}-01`, -1)
  const weeks: (string | null)[][] = []
  let cursor = mondayOf(first)
  while (cursor <= last) {
    const week: (string | null)[] = []
    for (let i = 0; i < 7; i++) {
      const date = addDays(cursor, i)
      week.push(date.startsWith(month) ? date : null)
    }
    weeks.push(week)
    cursor = addDays(cursor, 7)
  }
  return weeks
}

/** Season phases covering date (an open-ended phase runs on). */
export function phasesOn(date: string, phases: CalendarPhase[]): CalendarPhase[] {
  return phases.filter(p => p.start_date <= date && (p.end_date === null || p.end_date >= date))
}

export type DayTone = 'done' | 'partial' | 'missed' | 'upcoming' | 'logged' | 'none'

/**
 * The one-word summary for a day cell: a scheduled day uses its plan
 * status; an unscheduled day with any session is "logged".
 */
export function dayTone(day: CalendarDay): DayTone {
  if (day.scheduled && day.status) return day.status
  return day.sessions.length > 0 ? 'logged' : 'none'
}

export const PHASE_LABEL: Record<string, string> = { off: 'Off-season', pre: 'Pre-season', in: 'In-season' }

export const DISCIPLINE_LABEL: Record<string, string> = {
  resistance: 'Lifting',
  throwing: 'Throwing',
  conditioning: 'Conditioning',
  skill: 'Skill',
  recovery: 'Recovery check-in',
}

/** "Bullpen", "Long toss" from a stored snake_case label. */
export function humanize(label: string | null): string {
  if (!label) return ''
  const s = label.replace(/_/g, ' ')
  return s.charAt(0).toUpperCase() + s.slice(1)
}

export const COMPLIANCE_LABEL: Record<ComplianceStatus, string> = {
  rest: 'Rest',
  prescribed: 'Prescribed',
  done: 'Done',
  partial: 'Partial',
  skipped: 'Skipped',
  extra: 'Extra',
}

/** Team-wide totals for the compliance header. Planned excludes extra days. */
export function complianceTotals(athletes: ComplianceAthlete[]) {
  return athletes.reduce(
    (t, a) => ({
      planned: t.planned + a.planned_count,
      done: t.done + a.done_count,
      partial: t.partial + a.partial_count,
      skipped: t.skipped + a.skipped_count,
      extra: t.extra + a.extra_count,
    }),
    { planned: 0, done: 0, partial: 0, skipped: 0, extra: 0 },
  )
}
