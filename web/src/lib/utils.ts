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
