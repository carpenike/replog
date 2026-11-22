import { useMemo, useState } from 'react'
import { api, ApiError } from '@/api/client'
import { restAlertsEnabledHere, setRestAlertsEnabledHere, type RestAlertSink } from '@/lib/push'

/**
 * The server-side rest alert for this device, or null when this device
 * has not opted in (Preferences → Rest alerts). Calls are fire-and-forget:
 * a failed schedule never disturbs the in-page timer. If the server says
 * no device is subscribed or push is off, alerts switch off here too.
 */
export function useRestAlertSink(url: string): RestAlertSink | null {
  const [enabled, setEnabled] = useState(() => restAlertsEnabledHere())
  return useMemo<RestAlertSink | null>(() => {
    if (!enabled) return null
    const turnOff = () => {
      setRestAlertsEnabledHere(false)
      setEnabled(false)
    }
    return {
      schedule: (seconds: number) => {
        const s = Math.min(3600, Math.max(1, Math.round(seconds)))
        api.scheduleRestAlert({ seconds: s, url })
          .then(res => { if (!res.scheduled) turnOff() })
          .catch((err: unknown) => {
            if (err instanceof ApiError && (err.code === 503 || err.code === 403)) turnOff()
          })
      },
      cancel: () => {
        api.cancelRestAlert().catch(() => {
          // Offline: the push may still fire; harmless.
        })
      },
    }
  }, [enabled, url])
}
