import type { ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { ApiError } from '@/api/client'
import { Button } from '@/components/ui/button'

export function Spinner({ className = '' }: { className?: string }) {
  return (
    <div className={`flex items-center justify-center py-8 ${className}`}>
      <div className="h-6 w-6 animate-spin rounded-full border-2 border-muted-foreground border-t-primary" />
    </div>
  )
}

export function LoadingPage() {
  return (
    <div className="flex items-center justify-center min-h-[40vh]">
      <div className="text-center">
        <div className="h-8 w-8 animate-spin rounded-full border-2 border-muted-foreground border-t-primary mx-auto mb-3" />
        <p className="text-sm text-muted-foreground">Loading...</p>
      </div>
    </div>
  )
}

export function ErrorMessage({ message = 'Something went wrong.' }: { message?: string }) {
  return (
    <div className="rounded-lg border border-destructive/30 bg-destructive/5 p-4">
      <p className="text-sm text-destructive">{message}</p>
    </div>
  )
}

export function EmptyState({
  icon = '📭',
  title,
  description,
  action,
  actionTo,
  onAction,
}: {
  icon?: string
  title: string
  description?: string
  /** Optional call-to-action label. Renders a link when actionTo is set, else a button when onAction is set. */
  action?: string
  actionTo?: string
  onAction?: () => void
}) {
  return (
    <div className="flex flex-col items-center justify-center py-12 text-center">
      <span className="text-3xl mb-2" aria-hidden="true">{icon}</span>
      <p className="text-muted-foreground font-medium">{title}</p>
      {description && <p className="text-sm text-muted-foreground mt-1">{description}</p>}
      {action && actionTo && (
        <Button render={<Link to={actionTo} />} className="mt-4">{action}</Button>
      )}
      {action && !actionTo && onAction && (
        <Button onClick={onAction} className="mt-4">{action}</Button>
      )}
    </div>
  )
}

/**
 * Shared, retryable error surface for failed queries. A 404 is rendered as a
 * soft "not found" empty state (an expected state, not an error); anything else
 * shows the message plus a Retry button.
 */
export function QueryError({
  error,
  onRetry,
  resource = 'data',
  notFound,
}: {
  error: unknown
  onRetry?: () => void
  /** Noun used in the fallback message, e.g. "workout" → "Failed to load workout." */
  resource?: string
  /** Custom node to render for a 404 instead of the default not-found state. */
  notFound?: ReactNode
}) {
  if (error instanceof ApiError && error.code === 404) {
    if (notFound !== undefined) return <>{notFound}</>
    return <EmptyState icon="🔍" title={`No ${resource} found`} />
  }
  const message = error instanceof Error ? error.message : `Failed to load ${resource}.`
  return (
    <div className="rounded-lg border border-destructive/30 bg-destructive/5 p-4 flex flex-col items-start gap-3">
      <p className="text-sm text-destructive">{message}</p>
      {onRetry && (
        <Button variant="outline" size="sm" onClick={onRetry}>Retry</Button>
      )}
    </div>
  )
}
