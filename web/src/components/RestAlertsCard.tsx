import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { BellOff, BellRing, Send } from 'lucide-react'
import { api, ApiError } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import {
  currentSubscription,
  detectPushSupport,
  readPushEnv,
  restAlertsEnabledHere,
  setRestAlertsEnabledHere,
  subscribeThisDevice,
  unsubscribeThisDevice,
  type PushSupport,
} from '@/lib/push'

const IOS_NOTE =
  'On iPhone or iPad, lock-screen alerts need RepLog on your Home Screen: tap Share, then Add to Home Screen (iOS 16.4 or later), open RepLog from that icon, and turn alerts on there.'

const SUPPORT_COPY: Record<Exclude<PushSupport, 'supported'>, string> = {
  'ios-install': IOS_NOTE,
  'ios-update': 'Lock-screen alerts need iOS 16.4 or later. Update iOS, then open RepLog from the Home Screen and turn alerts on.',
  insecure: 'Push notifications need a secure (HTTPS) connection to RepLog.',
  unsupported: "This browser can't receive push notifications.",
}

function errorMessage(err: unknown, fallback: string): string {
  if (err instanceof ApiError || err instanceof Error) return err.message
  return fallback
}

/**
 * Preferences opt-in for rest alerts: a push to this device when the rest
 * timer ends, so a locked phone still buzzes. Falls back to the in-page
 * timer, toast, and vibration everywhere push is unavailable.
 */
export function RestAlertsCard() {
  const queryClient = useQueryClient()
  const [support] = useState<PushSupport>(() => detectPushSupport(readPushEnv()))
  const config = useQuery({ queryKey: ['push-config'], queryFn: () => api.getPushConfig(), staleTime: 60_000 })
  const device = useQuery({
    queryKey: ['push-device'],
    queryFn: async () => {
      const sub = await currentSubscription()
      return { subscribed: sub !== null && restAlertsEnabledHere(), endpoint: sub?.endpoint ?? null }
    },
    enabled: support === 'supported',
  })
  const refresh = () => {
    void queryClient.invalidateQueries({ queryKey: ['push-config'] })
    void queryClient.invalidateQueries({ queryKey: ['push-device'] })
  }

  const enable = useMutation({
    mutationFn: async () => {
      const key = config.data?.public_key
      if (!key) throw new Error('Push is not set up on this server.')
      const body = await subscribeThisDevice(key)
      await api.subscribePush(body)
    },
    onSuccess: () => {
      setRestAlertsEnabledHere(true)
      toast.success('Rest alerts are on for this device')
      refresh()
    },
    onError: (err) => toast.error(errorMessage(err, 'Could not turn on rest alerts')),
  })

  const disable = useMutation({
    mutationFn: async () => {
      setRestAlertsEnabledHere(false)
      const endpoint = await unsubscribeThisDevice()
      if (endpoint) {
        await api.unsubscribePush(endpoint).catch((err: unknown) => {
          // Already gone on the server is fine.
          if (!(err instanceof ApiError && err.code === 404)) throw err
        })
      }
    },
    onSuccess: () => {
      toast.success('Rest alerts are off for this device')
      refresh()
    },
    onError: (err) => toast.error(errorMessage(err, 'Could not turn off rest alerts')),
  })

  const test = useMutation({
    mutationFn: () => api.testPush(),
    onSuccess: (res) => {
      if (res.sent > 0) toast.success(`Test sent to ${res.sent} device${res.sent === 1 ? '' : 's'}`)
      else toast.error('No device accepted the test. Turn alerts off and on again.')
    },
    onError: (err) => toast.error(errorMessage(err, 'The test push failed')),
  })

  const permissionDenied = support === 'supported' && typeof Notification !== 'undefined' && Notification.permission === 'denied'

  let body
  if (config.isLoading) {
    body = <Skeleton className="h-16 w-full" />
  } else if (!config.data?.enabled) {
    body = <p className="text-sm text-muted-foreground">Push isn't set up on this server. The in-app rest timer still counts down and vibrates.</p>
  } else if (support !== 'supported') {
    body = (
      <p className="text-sm text-muted-foreground">
        {SUPPORT_COPY[support]} The in-app rest timer still counts down and vibrates while RepLog is open.
      </p>
    )
  } else if (permissionDenied) {
    body = <p className="text-sm text-muted-foreground">Notifications are blocked for RepLog. Allow them in your browser or device settings, then come back here.</p>
  } else {
    const on = device.data?.subscribed ?? false
    body = (
      <div className="space-y-3">
        <p className="text-sm" aria-live="polite">
          {on ? 'On for this device.' : 'Off for this device.'}
          {config.data.subscriptions > 0 && (
            <span className="text-muted-foreground"> {config.data.subscriptions} device{config.data.subscriptions === 1 ? '' : 's'} on your account.</span>
          )}
        </p>
        <div className="flex flex-wrap gap-2">
          {on ? (
            <Button variant="outline" onClick={() => disable.mutate()} disabled={disable.isPending}>
              <BellOff aria-hidden="true" /> {disable.isPending ? 'Turning off…' : 'Turn off'}
            </Button>
          ) : (
            <Button onClick={() => enable.mutate()} disabled={enable.isPending || device.isLoading}>
              <BellRing aria-hidden="true" /> {enable.isPending ? 'Turning on…' : 'Turn on rest alerts'}
            </Button>
          )}
          {on && (
            <Button variant="ghost" onClick={() => test.mutate()} disabled={test.isPending}>
              <Send aria-hidden="true" /> Send a test
            </Button>
          )}
        </div>
        <p className="text-xs text-muted-foreground">{IOS_NOTE}</p>
      </div>
    )
  }

  return (
    <Card className="mt-8">
      <CardHeader>
        <CardTitle>Rest alerts</CardTitle>
        <CardDescription>Buzz this device when your rest is up, even with the screen locked.</CardDescription>
      </CardHeader>
      <CardContent>{body}</CardContent>
    </Card>
  )
}
