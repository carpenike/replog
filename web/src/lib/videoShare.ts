// Form-check share links: the expiry choices and the small formatting
// helpers the share sheet and the public viewer use. Kept free of React so
// they are unit-tested.
import type { VideoShare } from '@/api/types'

export interface ShareExpiryOption {
  hours: number
  label: string
}

/** The lifetimes the server accepts (it refuses anything over 30 days). */
export const SHARE_EXPIRY_OPTIONS: readonly ShareExpiryOption[] = [
  { hours: 24, label: '24 hours' },
  { hours: 168, label: '7 days' },
  { hours: 720, label: '30 days' },
]

export const DEFAULT_SHARE_EXPIRY_HOURS = 168

/** Matches the server's label cap. */
export const SHARE_LABEL_MAX = 100

/** Whether hours is one of the offered expiry choices. */
export function isShareExpiry(hours: number): boolean {
  return SHARE_EXPIRY_OPTIONS.some(o => o.hours === hours)
}

/** The absolute link to hand out, from the server's relative /s/{token} path. */
export function absoluteShareURL(path: string, origin: string): string {
  return new URL(path, origin).toString()
}

/** The token in a /s/{token} pathname, or null. */
export function shareTokenFromPath(pathname: string): string | null {
  const m = /^\/s\/([^/]+)\/?$/.exec(pathname)
  if (!m) return null
  try {
    return decodeURIComponent(m[1])
  } catch {
    return null
  }
}

/**
 * A friendly expiry: "today at 3:00 PM", "tomorrow at 9:15 AM", or
 * "Dec 1 at 3:00 PM" (with the year when it differs). Times are local.
 */
export function formatShareExpiry(iso: string, now: Date = new Date(), locale?: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  const time = d.toLocaleTimeString(locale, { hour: 'numeric', minute: '2-digit' })
  const startOfDay = (x: Date) => new Date(x.getFullYear(), x.getMonth(), x.getDate()).getTime()
  const days = Math.round((startOfDay(d) - startOfDay(now)) / 86_400_000)
  if (days === 0) return `today at ${time}`
  if (days === 1) return `tomorrow at ${time}`
  const date = d.toLocaleDateString(locale, {
    month: 'short',
    day: 'numeric',
    ...(d.getFullYear() !== now.getFullYear() ? { year: 'numeric' } : {}),
  })
  return `${date} at ${time}`
}

/** "Expires …", "Expired …" or "Revoked" for a link in the owner's list. */
export function shareStatusText(share: VideoShare, now: Date = new Date(), locale?: string): string {
  if (share.status === 'revoked') return 'Revoked'
  if (share.status === 'expired') return `Expired ${formatShareExpiry(share.expires_at, now, locale)}`
  return `Expires ${formatShareExpiry(share.expires_at, now, locale)}`
}

/** "No views yet", "1 view", "3 views". */
export function shareViewsText(count: number): string {
  if (count <= 0) return 'No views yet'
  return count === 1 ? '1 view' : `${count} views`
}
