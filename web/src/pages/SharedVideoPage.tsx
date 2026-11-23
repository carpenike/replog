import { useLocation } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'
import { formatFriendlyDate } from '@/lib/dates'
import { formatShareExpiry, shareTokenFromPath } from '@/lib/videoShare'
import { usePageTitle } from '@/lib/usePageTitle'

/** Message for a link that does not open. Same for every reason. */
function Unavailable({ message }: { message: string }) {
  return (
    <div className="space-y-2 py-16 text-center">
      <h1 className="text-xl font-semibold">Link not available</h1>
      <p className="text-muted-foreground">{message}</p>
    </div>
  )
}

/**
 * The public, no-login viewer for a form-check share link (/s/{token}). It
 * shows the clip, its exercise and date, and when the link expires, and
 * nothing else: no athlete name, no account data, no links into the app.
 * App renders it before the auth bootstrap, so it never redirects to login.
 */
export function SharedVideoPage() {
  const { pathname } = useLocation()
  const token = shareTokenFromPath(pathname)
  const shared = useQuery({
    queryKey: ['shared-video', token],
    queryFn: () => api.getSharedVideo(token as string),
    enabled: !!token,
    retry: false,
    staleTime: Infinity,
    gcTime: 0,
    refetchOnWindowFocus: false,
  })
  usePageTitle(shared.data ? `Form check · ${shared.data.exercise_name}` : 'Form check')

  let body
  if (!token) {
    body = <Unavailable message="This link has expired or been revoked." />
  } else if (shared.isLoading) {
    body = <p role="status" className="py-16 text-center text-muted-foreground">Loading…</p>
  } else if (shared.error || !shared.data) {
    const code = (shared.error as { code?: number } | null)?.code
    body = <Unavailable message={code === 429 || code === 0
      ? (shared.error as Error).message
      : 'This link has expired or been revoked.'} />
  } else {
    const v = shared.data
    body = (
      <article className="space-y-3">
        <video controls playsInline preload="metadata" src={v.video_url}
          className="w-full max-h-[70dvh] rounded-lg bg-black" aria-label={`Form check video: ${v.exercise_name}`}>
          <source src={v.video_url} type={v.content_type} />
        </video>
        <div>
          <h1 className="text-lg font-semibold">{v.exercise_name}</h1>
          <p className="text-sm text-muted-foreground">{formatFriendlyDate(v.date, { absolute: true })}</p>
        </div>
        <p className="text-xs text-muted-foreground">Shared via RepLog · expires {formatShareExpiry(v.expires_at)}</p>
      </article>
    )
  }

  return (
    <main className="min-h-dvh bg-background text-foreground">
      <div className="mx-auto max-w-2xl px-4 py-6 pb-[calc(1.5rem+env(safe-area-inset-bottom))]">{body}</div>
    </main>
  )
}
