import { useEffect } from 'react'

/**
 * Keep the screen awake while the component is mounted. The browser drops the
 * lock whenever the page is hidden, so it is re-acquired on visibilitychange.
 * Silently no-ops where the Screen Wake Lock API is unsupported or denied.
 */
export function useWakeLock() {
  useEffect(() => {
    if (!('wakeLock' in navigator)) return
    let sentinel: WakeLockSentinel | null = null
    let unmounted = false

    const acquire = async () => {
      try {
        const lock = await navigator.wakeLock.request('screen')
        if (unmounted) {
          void lock.release().catch(() => {})
        } else {
          sentinel = lock
        }
      } catch {
        // Denied (battery saver, permissions policy) — dimming is acceptable.
      }
    }

    const onVisibility = () => {
      if (document.visibilityState === 'visible') void acquire()
    }

    void acquire()
    document.addEventListener('visibilitychange', onVisibility)
    return () => {
      unmounted = true
      document.removeEventListener('visibilitychange', onVisibility)
      void sentinel?.release().catch(() => {})
    }
  }, [])
}
