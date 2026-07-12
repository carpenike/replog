import { useCallback, useEffect, useRef, useState } from 'react'

export interface RestTimer {
  running: boolean
  /** Seconds left, rounded up. 0 when idle. */
  remaining: number
  /** Total countdown length in seconds (grows with extend). 0 when idle. */
  total: number
  /** Start (or restart) the countdown. */
  start: (seconds: number) => void
  /** Add seconds to a running countdown; no-op when idle. */
  extend: (seconds: number) => void
  /** Stop without firing onComplete. */
  skip: () => void
}

/**
 * Countdown rest timer driven by wall-clock time (endsAt), so it stays
 * accurate when the tab is throttled in the background. onComplete fires
 * exactly once when the countdown expires.
 */
export function useRestTimer(onComplete?: () => void): RestTimer {
  const [timer, setTimer] = useState<{ endsAt: number; total: number } | null>(null)
  const [now, setNow] = useState(0)
  const onCompleteRef = useRef(onComplete)
  useEffect(() => {
    onCompleteRef.current = onComplete
  })

  useEffect(() => {
    if (!timer) return
    const id = window.setInterval(() => {
      const t = Date.now()
      if (t >= timer.endsAt) {
        setTimer(null)
        onCompleteRef.current?.()
      } else {
        setNow(t)
      }
    }, 250)
    return () => window.clearInterval(id)
  }, [timer])

  const start = useCallback((seconds: number) => {
    const t = Date.now()
    setNow(t)
    setTimer({ endsAt: t + seconds * 1000, total: seconds })
  }, [])

  const extend = useCallback((seconds: number) => {
    setTimer(t => t ? { endsAt: t.endsAt + seconds * 1000, total: t.total + seconds } : t)
  }, [])

  const skip = useCallback(() => setTimer(null), [])

  const remaining = timer ? Math.max(0, Math.ceil((timer.endsAt - now) / 1000)) : 0
  return { running: timer !== null, remaining, total: timer?.total ?? 0, start, extend, skip }
}
