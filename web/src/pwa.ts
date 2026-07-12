// Service worker registration (worker source: src/pwa-sw.ts).
//
// autoUpdate flavor: the new worker skips waiting and workbox-window reloads
// open clients, so no "update available" UI is needed.
import { registerSW } from 'virtual:pwa-register'

registerSW({
  immediate: true,
  onRegisteredSW(_swUrl, registration) {
    if (!registration) return
    // Gym tablets keep the app open for days — poll for new builds hourly so
    // long-lived tabs still pick up deploys.
    setInterval(() => {
      registration.update().catch(() => {
        // Offline or transient failure; the next interval will retry.
      })
    }, 60 * 60 * 1000)
  },
})
