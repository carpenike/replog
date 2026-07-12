import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

/** Strip time portion from ISO date strings (e.g. "2026-03-10T00:00:00Z" → "2026-03-10") */
export function formatDate(d: string | null | undefined): string {
  if (!d) return '—'
  return d.split('T')[0]
}

/**
 * Format a weight value, dropping the trailing zero for whole numbers
 * (e.g. 100 → "100", 102.5 → "102.5"). Single source of truth — previously
 * redefined ad hoc across several pages.
 */
export function formatWeight(w: number | null | undefined): string {
  if (w == null) return '—'
  return w === Math.floor(w) ? w.toString() : w.toFixed(1)
}

/**
 * Local calendar date as YYYY-MM-DD. toISOString() would shift the date near
 * UTC midnight — this is the "is it today for the athlete?" comparison.
 */
export function localDateISO(d = new Date()): string {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}
