import { useQuery } from '@tanstack/react-query'
import { useCallback } from 'react'
import { api } from '@/api/client'

/**
 * Date-format preference values. These are Go time layouts stored in
 * user_preferences.date_format; the server formats with them as-is and the
 * SPA maps them onto the same output here.
 */
export type DateLayout = 'Jan 2, 2006' | '2006-01-02' | '01/02/2006' | '02/01/2006'

export const DEFAULT_DATE_LAYOUT: DateLayout = 'Jan 2, 2006'

const WEEKDAYS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']
const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']

interface CalendarDate { y: number; m: number; d: number }

/**
 * Parse the calendar part of an API date ("2026-11-21" or
 * "2026-11-21T00:00:00Z"). The time portion is ignored on purpose: workout and
 * log dates are calendar days, and converting them through Date would shift
 * them across midnight for users west of UTC.
 */
export function parseCalendarDate(iso: string | null | undefined): CalendarDate | null {
  if (!iso) return null
  const m = /^(\d{4})-(\d{2})-(\d{2})/.exec(iso)
  if (!m) return null
  const y = Number(m[1]); const mo = Number(m[2]); const d = Number(m[3])
  if (mo < 1 || mo > 12 || d < 1 || d > 31) return null
  return { y, m: mo, d }
}

/** Days since the epoch for a calendar date (UTC arithmetic, no DST drift). */
function dayNumber(c: CalendarDate): number {
  return Math.round(Date.UTC(c.y, c.m - 1, c.d) / 86_400_000)
}

function weekday(c: CalendarDate): string {
  return WEEKDAYS[new Date(Date.UTC(c.y, c.m - 1, c.d)).getUTCDay()]
}

const pad = (n: number) => String(n).padStart(2, '0')

/** Today's calendar date, in the given IANA time zone when one is known. */
export function todayCalendar(now: Date = new Date(), timeZone?: string): CalendarDate {
  if (timeZone) {
    try {
      const parts = new Intl.DateTimeFormat('en-CA', {
        timeZone, year: 'numeric', month: '2-digit', day: '2-digit',
      }).formatToParts(now)
      const get = (t: string) => Number(parts.find(p => p.type === t)?.value)
      const c = { y: get('year'), m: get('month'), d: get('day') }
      if (c.y && c.m && c.d) return c
    } catch {
      // Unknown zone: fall through to the browser's local date.
    }
  }
  return { y: now.getFullYear(), m: now.getMonth() + 1, d: now.getDate() }
}

/** Format a calendar date with one of the preference layouts, e.g. "Nov 21, 2026". */
export function formatWithLayout(c: CalendarDate, layout: string, opts: { omitYear?: boolean } = {}): string {
  switch (layout) {
    case '2006-01-02':
      return `${c.y}-${pad(c.m)}-${pad(c.d)}`
    case '01/02/2006':
      return opts.omitYear ? `${pad(c.m)}/${pad(c.d)}` : `${pad(c.m)}/${pad(c.d)}/${c.y}`
    case '02/01/2006':
      return opts.omitYear ? `${pad(c.d)}/${pad(c.m)}` : `${pad(c.d)}/${pad(c.m)}/${c.y}`
    default:
      return opts.omitYear ? `${MONTHS[c.m - 1]} ${c.d}` : `${MONTHS[c.m - 1]} ${c.d}, ${c.y}`
  }
}

export interface FriendlyDateOptions {
  /** Go layout from the user's date-format preference. */
  layout?: string
  /** IANA zone used to decide what "today" is. */
  timeZone?: string
  /** Injectable clock for tests. */
  now?: Date
  /** Skip "Today"/"Yesterday"/"Tomorrow" and always print the date. */
  absolute?: boolean
  /**
   * The value is an instant (created_at, sent_at), not a calendar date:
   * convert it to the user's local day before formatting.
   */
  timestamp?: boolean
}

/**
 * Friendly calendar date for headings and lists: "Today", "Yesterday",
 * "Tomorrow", otherwise "Sat, Nov 21" this year or "Sat, Nov 21, 2025"
 * for other years. Numeric preference layouts keep their order
 * ("Sat, 11/21", "Sat, 2026-11-21"). Returns "—" for a missing date and
 * the raw string when it is not a date.
 */
export function formatFriendlyDate(iso: string | null | undefined, opts: FriendlyDateOptions = {}): string {
  if (!iso) return '—'
  let c = parseCalendarDate(iso)
  if (opts.timestamp) {
    const t = new Date(iso)
    if (!isNaN(t.getTime())) c = todayCalendar(t, opts.timeZone)
  }
  if (!c) return iso
  const today = todayCalendar(opts.now, opts.timeZone)
  if (!opts.absolute) {
    const diff = dayNumber(c) - dayNumber(today)
    if (diff === 0) return 'Today'
    if (diff === -1) return 'Yesterday'
    if (diff === 1) return 'Tomorrow'
  }
  const layout = opts.layout || DEFAULT_DATE_LAYOUT
  // ISO layout always carries the year so it stays sortable and unambiguous.
  const omitYear = c.y === today.y && layout !== '2006-01-02'
  return `${weekday(c)}, ${formatWithLayout(c, layout, { omitYear })}`
}

/**
 * Hook returning a formatter bound to the signed-in user's date-format and
 * time-zone preferences. Falls back to defaults until (or if) preferences load.
 */
export function useFriendlyDate(): (iso: string | null | undefined, opts?: Omit<FriendlyDateOptions, 'layout' | 'timeZone'>) => string {
  const { data: prefs } = useQuery({
    queryKey: ['preferences'],
    queryFn: () => api.getPreferences(),
    staleTime: 5 * 60_000,
    retry: false,
  })
  const layout = prefs?.date_format
  const timeZone = prefs?.timezone
  return useCallback(
    (iso, opts) => formatFriendlyDate(iso, { ...opts, layout, timeZone }),
    [layout, timeZone],
  )
}
