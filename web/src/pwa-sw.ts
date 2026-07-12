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
registerRoute(
  ({ url, sameOrigin }) => sameOrigin && url.pathname.startsWith('/api/'),
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
  sameOrigin && url.pathname.startsWith('/api/athletes/')

for (const method of ['POST', 'PUT', 'DELETE'] as const) {
  registerRoute(
    matchAthleteMutation,
    new NetworkOnly({ plugins: [mutationQueue] }),
    method,
  )
}
