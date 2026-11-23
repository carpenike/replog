import { useSyncExternalStore } from 'react'

/** Tailwind's `md` breakpoint, the point where the desktop sidebar appears. */
export const DESKTOP_QUERY = '(min-width: 768px)'

function matches(query: string): boolean {
  return typeof window !== 'undefined' && typeof window.matchMedia === 'function'
    ? window.matchMedia(query).matches
    : false
}

/** Subscribe to a CSS media query. False where matchMedia is unavailable. */
export function useMediaQuery(query: string): boolean {
  return useSyncExternalStore(
    (onChange) => {
      if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return () => {}
      const mq = window.matchMedia(query)
      mq.addEventListener('change', onChange)
      return () => mq.removeEventListener('change', onChange)
    },
    () => matches(query),
    () => false,
  )
}
