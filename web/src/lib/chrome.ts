import type { ToasterProps } from 'sonner'

/**
 * Where toasts go. Phones get top-center, under the fixed header, so a toast
 * never covers the rest-timer bar or the set inputs at the bottom. Desktop
 * keeps bottom-right, lifted by --toast-offset-bottom while the rest timer
 * bar is showing (see RestTimerBar and index.css).
 */
export function toasterPlacement(isDesktop: boolean): Pick<ToasterProps, 'position' | 'offset' | 'mobileOffset'> {
  const below = 'calc(var(--app-header-offset) + 0.5rem)'
  const bottom = 'var(--toast-offset-bottom)'
  return {
    position: isDesktop ? 'bottom-right' : 'top-center',
    offset: { top: below, bottom },
    mobileOffset: { top: below, bottom },
  }
}

/** Accessible name for the header notifications bell, with the unread count. */
export function bellLabel(unreadCount: number): string {
  if (unreadCount <= 0) return 'Notifications'
  return `Notifications, ${unreadCount} unread`
}
