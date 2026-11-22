/// <reference lib="webworker" />
// RepLog service worker, built by vite-plugin-pwa (injectManifest strategy).
//
// We use a hand-written worker instead of generateSW because the offline
// mutation queue needs ONE BackgroundSyncPlugin instance shared across
// POST/PUT/DELETE routes — generateSW's runtimeCaching would construct a
// separate plugin (and thus a duplicate-named queue, which Workbox rejects)
// per method.
import { clientsClaim } from 'workbox-core'
import {
  cleanupOutdatedCaches,
  createHandlerBoundToURL,
  precacheAndRoute,
} from 'workbox-precaching'
import type { PrecacheEntry } from 'workbox-precaching'
import { NavigationRoute, registerRoute } from 'workbox-routing'
import { NetworkFirst, NetworkOnly } from 'workbox-strategies'
import { CacheableResponsePlugin } from 'workbox-cacheable-response'
import { ExpirationPlugin } from 'workbox-expiration'
import { BackgroundSyncPlugin } from 'workbox-background-sync'
import { isSetVideoPath } from './lib/swRoutes'

declare let self: ServiceWorkerGlobalScope & {
  __WB_MANIFEST: Array<PrecacheEntry | string>
}

// autoUpdate flavor: a freshly installed worker activates immediately and
// takes over open clients; workbox-window (see pwa.ts) reloads them.
self.skipWaiting()
clientsClaim()

self.addEventListener('message', (event) => {
  if (event.data?.type === 'SKIP_WAITING') void self.skipWaiting()
})

// --- App shell -------------------------------------------------------------
// Precache the built assets (js/css/html/icons/fonts — see injectManifest
// globPatterns in vite.config.ts).
precacheAndRoute(self.__WB_MANIFEST)
cleanupOutdatedCaches()

// SPA navigation fallback: serve the precached index.html for client-side
// routes. Backend-served paths must NOT fall back to the shell:
//   /api/**     — JSON API
//   /metrics, /healthz — ops endpoints
//   /auth/oidc/** — OIDC relying-party redirects served by the Go binary
//     (note: /auth/token/:token IS a client-side SPA route, so only the
//     /auth/oidc subtree is denied)
//   /avatars/** — backend-served images
registerRoute(
  new NavigationRoute(createHandlerBoundToURL('/index.html'), {
    denylist: [/^\/api\//, /^\/metrics/, /^\/healthz/, /^\/auth\/oidc/, /^\/avatars\//],
  }),
)

// --- API reads: network-first with short timeout ---------------------------
// A kid mid-workout on flaky gym wifi still sees today's prescription: fall
// back to the last-known-good response if the network doesn't answer within
// 3s. GET only (registerRoute defaults to GET); never cache errors.
// Form-check video routes are not registered at all (see isSetVideoPath):
// the browser fetches clips directly, uncached, with Range support.
registerRoute(
  ({ url, sameOrigin }) => sameOrigin && url.pathname.startsWith('/api/') && !isSetVideoPath(url.pathname),
  new NetworkFirst({
    cacheName: 'api-cache',
    networkTimeoutSeconds: 3,
    plugins: [
      new CacheableResponsePlugin({ statuses: [200] }),
      new ExpirationPlugin({ maxEntries: 200, maxAgeSeconds: 24 * 60 * 60 }),
    ],
  }),
)

// --- API mutations: offline queue ------------------------------------------
// Sets logged while offline are queued and replayed (in order) when
// connectivity returns. One shared queue across all mutating methods.
// Caveat: iOS Safari has no Background Sync API — there Workbox replays the
// queue only when the SW next starts (i.e. while the app is open), not in
// the background.
const mutationQueue = new BackgroundSyncPlugin('replog-mutations', {
  maxRetentionTime: 24 * 60, // minutes — drop mutations older than 24h
})

const matchAthleteMutation = ({ url, sameOrigin }: { url: URL; sameOrigin: boolean }) =>
  sameOrigin && url.pathname.startsWith('/api/athletes/') && !isSetVideoPath(url.pathname)

for (const method of ['POST', 'PUT', 'DELETE'] as const) {
  registerRoute(
    matchAthleteMutation,
    new NetworkOnly({ plugins: [mutationQueue] }),
    method,
  )
}

// --- Web Push: rest alerts --------------------------------------------------
// The server schedules a push for the moment the rest timer ends, so a
// locked phone still buzzes. These listeners only add notification
// handling; they register no routes, so the caching and the offline
// mutation queue above are untouched. (Push calls live under /api/push,
// outside the /api/athletes mutation queue: a rest alert replayed after
// reconnecting would be stale.)
interface PushPayload {
  kind?: string
  title?: string
  body?: string
  tag?: string
  url?: string
  sent_at?: number
}

// Fields some browsers support that the TS DOM lib doesn't list yet.
type ExtendedNotificationOptions = NotificationOptions & { renotify?: boolean; vibrate?: number[] }

/** An in-app path, or "/" for anything else (never navigate off-origin). */
function inAppURL(raw: unknown): string {
  try {
    const url = new URL(typeof raw === 'string' && raw ? raw : '/', self.location.origin)
    return url.origin === self.location.origin ? url.pathname + url.search + url.hash : '/'
  } catch {
    return '/'
  }
}

self.addEventListener('push', (event) => {
  let data: PushPayload = {}
  try {
    data = (event.data?.json() as PushPayload | undefined) ?? {}
  } catch {
    data = { title: event.data?.text() || 'RepLog' }
  }
  event.waitUntil((async () => {
    const windows = await self.clients.matchAll({ type: 'window', includeUncontrolled: true })
    // The page already toasts when it is in front; stay quiet there. A
    // notification is still shown either way: browsers require every push
    // to surface one or they revoke the subscription.
    const inFront = windows.some(c => (c as WindowClient).focused && c.visibilityState === 'visible')
    const options: ExtendedNotificationOptions = {
      body: data.body,
      tag: data.tag || 'replog',
      renotify: true,
      silent: inFront,
      vibrate: inFront ? undefined : [200, 100, 200],
      icon: '/pwa-192x192.png',
      badge: '/pwa-192x192.png',
      data: { url: inAppURL(data.url) },
    }
    await self.registration.showNotification(data.title || 'RepLog', options)
  })())
})

self.addEventListener('notificationclick', (event) => {
  event.notification.close()
  const target = inAppURL((event.notification.data as { url?: string } | null)?.url)
  event.waitUntil((async () => {
    const windows = (await self.clients.matchAll({ type: 'window', includeUncontrolled: true })) as WindowClient[]
    const exact = windows.find(c => new URL(c.url).pathname === new URL(target, self.location.origin).pathname)
    if (exact) {
      await exact.focus()
      return
    }
    const any = windows[0]
    if (any) {
      try {
        await any.focus()
        await any.navigate(target)
        return
      } catch {
        // Not controlled by this worker yet; open a fresh window instead.
      }
    }
    await self.clients.openWindow(target)
  })())
})

// The browser rotated the subscription (expiry, key change). Re-subscribe
// with the server's key and store the new endpoint so alerts keep working.
self.addEventListener('pushsubscriptionchange', (event) => {
  const e = event as ExtendableEvent & { oldSubscription?: PushSubscription | null }
  e.waitUntil((async () => {
    try {
      const res = await fetch('/api/push/config', { credentials: 'same-origin' })
      if (!res.ok) return
      const cfg = (await res.json()) as { enabled?: boolean; public_key?: string }
      if (!cfg.enabled || !cfg.public_key) return
      const key = cfg.public_key.replace(/-/g, '+').replace(/_/g, '/')
      const raw = atob(key + '='.repeat((4 - (key.length % 4)) % 4))
      const bytes = new Uint8Array(new ArrayBuffer(raw.length))
      for (let i = 0; i < raw.length; i++) bytes[i] = raw.charCodeAt(i)
      const sub = await self.registration.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: bytes })
      await fetch('/api/push/subscriptions', {
        method: 'POST',
        credentials: 'same-origin',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(sub.toJSON()),
      })
      if (e.oldSubscription && e.oldSubscription.endpoint !== sub.endpoint) {
        await fetch('/api/push/subscriptions', {
          method: 'DELETE',
          credentials: 'same-origin',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ endpoint: e.oldSubscription.endpoint }),
        })
      }
    } catch {
      // Best effort: the Preferences page can re-enable alerts.
    }
  })())
})
