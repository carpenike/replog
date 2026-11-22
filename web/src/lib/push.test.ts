import { describe, expect, it, vi } from 'vitest'
import { detectPushSupport, isIOS, sameKey, subscriptionBody, urlBase64ToUint8Array, withRestAlerts, type PushEnv } from './push'
import type { RestTimer } from './useRestTimer'

const desktopChrome: PushEnv = {
  hasServiceWorker: true,
  hasPushManager: true,
  hasNotification: true,
  isSecureContext: true,
  userAgent: 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 Chrome/130 Safari/537.36',
  maxTouchPoints: 0,
  standalone: false,
}
const iphoneUA = 'Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 Version/17.4 Mobile/15E148 Safari/604.1'
const ipadUA = 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 Version/17.4 Safari/605.1.15'

describe('detectPushSupport', () => {
  it('supports a modern desktop browser', () => {
    expect(detectPushSupport(desktopChrome)).toBe('supported')
  })

  it('asks iPhone Safari tabs to install to the Home Screen', () => {
    expect(detectPushSupport({ ...desktopChrome, userAgent: iphoneUA, hasPushManager: false })).toBe('ios-install')
  })

  it('treats iPadOS (a touch Mac) like iOS', () => {
    expect(isIOS({ userAgent: ipadUA, maxTouchPoints: 5 })).toBe(true)
    expect(isIOS({ userAgent: ipadUA, maxTouchPoints: 0 })).toBe(false)
    expect(detectPushSupport({ ...desktopChrome, userAgent: ipadUA, maxTouchPoints: 5 })).toBe('ios-install')
  })

  it('supports an installed iOS 16.4+ app and flags older iOS', () => {
    expect(detectPushSupport({ ...desktopChrome, userAgent: iphoneUA, standalone: true })).toBe('supported')
    expect(detectPushSupport({ ...desktopChrome, userAgent: iphoneUA, standalone: true, hasPushManager: false })).toBe('ios-update')
  })

  it('needs a secure context and the Push API', () => {
    expect(detectPushSupport({ ...desktopChrome, isSecureContext: false })).toBe('insecure')
    expect(detectPushSupport({ ...desktopChrome, hasPushManager: false })).toBe('unsupported')
    expect(detectPushSupport({ ...desktopChrome, hasServiceWorker: false })).toBe('unsupported')
  })
})

describe('urlBase64ToUint8Array', () => {
  it('decodes unpadded base64url', () => {
    // 0xfb 0xff 0xfe → "-__-" in base64url
    expect(Array.from(urlBase64ToUint8Array('-__-'))).toEqual([0xfb, 0xff, 0xfe])
    expect(Array.from(urlBase64ToUint8Array('AQ'))).toEqual([1])
  })
})

describe('sameKey', () => {
  const key = new Uint8Array([4, 1, 2, 3])
  it('matches identical keys and trusts a browser that reports none', () => {
    expect(sameKey(new Uint8Array([4, 1, 2, 3]).buffer, key)).toBe(true)
    expect(sameKey(null, key)).toBe(true)
  })
  it('flags a subscription made with a different VAPID key', () => {
    expect(sameKey(new Uint8Array([4, 1, 2, 9]).buffer, key)).toBe(false)
    expect(sameKey(new Uint8Array([4, 1, 2]).buffer, key)).toBe(false)
  })
})

describe('subscriptionBody', () => {
  it('keeps the endpoint and keys and rejects incomplete JSON', () => {
    expect(subscriptionBody({ endpoint: 'https://fcm.googleapis.com/x', keys: { p256dh: 'p', auth: 'a' } }))
      .toEqual({ endpoint: 'https://fcm.googleapis.com/x', keys: { p256dh: 'p', auth: 'a' } })
    expect(subscriptionBody({ endpoint: 'https://fcm.googleapis.com/x', keys: { p256dh: 'p' } })).toBeNull()
    expect(subscriptionBody({})).toBeNull()
  })
})

function fakeTimer(over: Partial<RestTimer> = {}): RestTimer {
  return { running: true, remaining: 40, total: 90, start: vi.fn(), extend: vi.fn(), skip: vi.fn(), ...over }
}

describe('withRestAlerts', () => {
  it('returns the timer untouched without an alert sink', () => {
    const t = fakeTimer()
    expect(withRestAlerts(t, null)).toBe(t)
  })

  it('schedules on start, reschedules on +30s, cancels on skip', () => {
    const t = fakeTimer()
    const sink = { schedule: vi.fn(), cancel: vi.fn() }
    const w = withRestAlerts(t, sink)

    w.start(90)
    expect(t.start).toHaveBeenCalledWith(90)
    expect(sink.schedule).toHaveBeenLastCalledWith(90)

    w.extend(30)
    expect(t.extend).toHaveBeenCalledWith(30)
    expect(sink.schedule).toHaveBeenLastCalledWith(70) // 40 left + 30

    w.skip()
    expect(t.skip).toHaveBeenCalled()
    expect(sink.cancel).toHaveBeenCalledTimes(1)
  })

  it('does not schedule an extension for an idle timer', () => {
    const sink = { schedule: vi.fn(), cancel: vi.fn() }
    withRestAlerts(fakeTimer({ running: false, remaining: 0 }), sink).extend(30)
    expect(sink.schedule).not.toHaveBeenCalled()
  })
})
