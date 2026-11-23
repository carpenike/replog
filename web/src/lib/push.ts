// Web Push rest alerts (gap 4): feature detection, subscribe/unsubscribe,
// and the wrapper that keeps the server-scheduled push in step with the
// in-page rest timer. Detection and the timer wrapper are pure so they can
// be unit-tested without a browser.
import type { RestTimer } from '@/lib/useRestTimer'

/**
 * - supported: this browser can subscribe.
 * - ios-install: iPhone/iPad in a Safari tab. Push needs the app added to
 *   the Home Screen (iOS 16.4+) and opened from there.
 * - ios-update: installed on iOS, but too old for Web Push (before 16.4).
 * - insecure: push needs HTTPS (or localhost).
 * - unsupported: the browser has no Push API.
 */
export type PushSupport = 'supported' | 'ios-install' | 'ios-update' | 'insecure' | 'unsupported'

export interface PushEnv {
  hasServiceWorker: boolean
  hasPushManager: boolean
  hasNotification: boolean
  isSecureContext: boolean
  userAgent: string
  maxTouchPoints: number
  /** Running as an installed app (display-mode: standalone, or iOS navigator.standalone). */
  standalone: boolean
}

/** iPhone, iPod, or iPad (iPadOS reports itself as a touch Mac). */
export function isIOS(env: Pick<PushEnv, 'userAgent' | 'maxTouchPoints'>): boolean {
  return /iPad|iPhone|iPod/.test(env.userAgent) || (/Macintosh/.test(env.userAgent) && env.maxTouchPoints > 1)
}

export function detectPushSupport(env: PushEnv): PushSupport {
  if (isIOS(env)) {
    if (!env.standalone) return 'ios-install'
    if (!env.hasPushManager || !env.hasNotification || !env.hasServiceWorker) return 'ios-update'
  }
  if (!env.isSecureContext) return 'insecure'
  if (!env.hasServiceWorker || !env.hasPushManager || !env.hasNotification) return 'unsupported'
  return 'supported'
}

/** Read the current browser's PushEnv. Safe to call where APIs are missing. */
export function readPushEnv(): PushEnv {
  const nav = typeof navigator !== 'undefined' ? navigator : undefined
  const win = typeof window !== 'undefined' ? window : undefined
  let standalone = false
  try {
    standalone = Boolean(win?.matchMedia?.('(display-mode: standalone)').matches)
      || (nav as Navigator & { standalone?: boolean } | undefined)?.standalone === true
  } catch {
    standalone = false
  }
  return {
    hasServiceWorker: Boolean(nav && 'serviceWorker' in nav),
    hasPushManager: Boolean(win && 'PushManager' in win),
    hasNotification: Boolean(win && 'Notification' in win),
    isSecureContext: Boolean(win?.isSecureContext),
    userAgent: nav?.userAgent ?? '',
    maxTouchPoints: nav?.maxTouchPoints ?? 0,
    standalone,
  }
}

/** Decode a base64url VAPID key into the bytes PushManager.subscribe wants. */
export function urlBase64ToUint8Array(base64url: string): Uint8Array<ArrayBuffer> {
  const padded = base64url.replace(/-/g, '+').replace(/_/g, '/') + '='.repeat((4 - (base64url.length % 4)) % 4)
  const raw = atob(padded)
  const out = new Uint8Array(new ArrayBuffer(raw.length))
  for (let i = 0; i < raw.length; i++) out[i] = raw.charCodeAt(i)
  return out
}

/**
 * Whether a subscription's applicationServerKey matches the server's key.
 * A browser that doesn't report the key (null) is trusted as a match.
 */
export function sameKey(existing: ArrayBuffer | null, serverKey: Uint8Array): boolean {
  if (!existing) return true
  const a = new Uint8Array(existing)
  if (a.length !== serverKey.length) return false
  for (let i = 0; i < a.length; i++) if (a[i] !== serverKey[i]) return false
  return true
}

export interface SubscriptionBody {
  endpoint: string
  keys: { p256dh: string; auth: string }
}

/** The request body for POST /api/push/subscriptions, or null if incomplete. */
export function subscriptionBody(json: PushSubscriptionJSON): SubscriptionBody | null {
  const p256dh = json.keys?.p256dh
  const auth = json.keys?.auth
  if (!json.endpoint || !p256dh || !auth) return null
  return { endpoint: json.endpoint, keys: { p256dh, auth } }
}

// --- This-device opt-in flag ------------------------------------------------
// Whether this browser subscribed, so the session page only calls the
// rest-alert API on devices that will actually buzz. Storage can be
// unavailable (private mode); then alerts are simply off.

const FLAG_KEY = 'replog.rest-alerts'

export function restAlertsEnabledHere(): boolean {
  try {
    if (typeof Notification !== 'undefined' && Notification.permission !== 'granted') return false
    return localStorage.getItem(FLAG_KEY) === '1'
  } catch {
    return false
  }
}

export function setRestAlertsEnabledHere(on: boolean) {
  try {
    if (on) localStorage.setItem(FLAG_KEY, '1')
    else localStorage.removeItem(FLAG_KEY)
  } catch {
    // Not persisted; nothing else depends on it.
  }
}

// --- Browser subscription helpers ------------------------------------------

async function registration(): Promise<ServiceWorkerRegistration> {
  // ready never resolves without a worker; cap it so the UI can fall back.
  return Promise.race([
    navigator.serviceWorker.ready,
    new Promise<never>((_, reject) => setTimeout(() => reject(new Error('The app is not installed as a PWA yet. Reload and try again.')), 5000)),
  ])
}

export async function currentSubscription(): Promise<PushSubscription | null> {
  try {
    const reg = await registration()
    return await reg.pushManager.getSubscription()
  } catch {
    return null
  }
}

/** Ask for permission and subscribe this device. Throws with a readable message. */
export async function subscribeThisDevice(publicKey: string): Promise<SubscriptionBody> {
  const permission = await Notification.requestPermission()
  if (permission !== 'granted') {
    throw new Error(permission === 'denied'
      ? 'Notifications are blocked for this site. Allow them in your browser or device settings.'
      : 'Notification permission was not granted.')
  }
  const reg = await registration()
  const serverKey = urlBase64ToUint8Array(publicKey)
  let sub = await reg.pushManager.getSubscription()
  // A subscription made with an older VAPID key can't receive this
  // server's pushes; replace it instead of re-sending the stale one.
  if (sub && !sameKey(sub.options?.applicationServerKey ?? null, serverKey)) {
    await sub.unsubscribe().catch(() => false)
    sub = null
  }
  if (!sub) {
    sub = await reg.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: serverKey })
  }
  const body = subscriptionBody(sub.toJSON())
  if (!body) throw new Error('The browser returned an incomplete push subscription.')
  return body
}

/** Unsubscribe this device locally. Returns the endpoint to remove server-side. */
export async function unsubscribeThisDevice(): Promise<string | null> {
  const sub = await currentSubscription()
  if (!sub) return null
  const endpoint = sub.endpoint
  try {
    await sub.unsubscribe()
  } catch {
    // The server row is removed anyway; the browser cleans up on its own.
  }
  return endpoint
}

// --- Rest timer wrapper -----------------------------------------------------

export interface RestAlertSink {
  /** Schedule (or replace) the push for `seconds` from now. */
  schedule: (seconds: number) => void
  /** Drop the pending push. */
  cancel: () => void
}

/**
 * Wrap a rest timer so the server-side push follows it: start schedules,
 * +30s reschedules with the new remaining time, Skip cancels. The local
 * countdown, toast, and vibration are unchanged, so a browser without push
 * behaves exactly as before.
 */
export function withRestAlerts(timer: RestTimer, alerts: RestAlertSink | null): RestTimer {
  if (!alerts) return timer
  return {
    ...timer,
    start: (seconds: number) => {
      timer.start(seconds)
      if (seconds > 0) alerts.schedule(seconds)
    },
    extend: (seconds: number) => {
      timer.extend(seconds)
      if (timer.running) alerts.schedule(Math.max(1, timer.remaining + seconds))
    },
    skip: () => {
      timer.skip()
      alerts.cancel()
    },
  }
}
