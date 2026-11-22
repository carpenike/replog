import { describe, expect, it } from 'vitest'
import { bellLabel, toasterPlacement } from '@/lib/chrome'

describe('toasterPlacement', () => {
  it('puts phone toasts at the top, clear of the rest-timer bar', () => {
    const p = toasterPlacement(false)
    expect(p.position).toBe('top-center')
    expect(p.mobileOffset).toMatchObject({ top: 'calc(var(--app-header-offset) + 0.5rem)' })
  })

  it('keeps desktop toasts bottom-right, lifted by the rest-timer offset', () => {
    const p = toasterPlacement(true)
    expect(p.position).toBe('bottom-right')
    expect(p.offset).toMatchObject({ bottom: 'var(--toast-offset-bottom)' })
  })
})

describe('bellLabel', () => {
  it('includes the unread count only when there is one', () => {
    expect(bellLabel(0)).toBe('Notifications')
    expect(bellLabel(3)).toBe('Notifications, 3 unread')
  })
})
