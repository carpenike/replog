import type { ReactNode } from 'react'
import { EmptyState, QueryError } from '@/components/ui'
import { Skeleton } from '@/components/ui/skeleton'

/** The parts of a TanStack Query result QueryState reads. */
export interface QueryLike<T> {
  data: T | undefined
  error: unknown
  refetch?: () => unknown
}

export interface EmptyProps {
  icon?: ReactNode
  title: string
  description?: string
  action?: string
  actionTo?: string
  onAction?: () => void
}

export type SkeletonKind = 'list' | 'cards' | 'detail' | 'form'

/** Placeholder shapes shown while a query loads. */
export function PageSkeleton({ kind = 'list', rows = 5 }: { kind?: SkeletonKind; rows?: number }) {
  if (kind === 'cards') {
    return (
      <div role="status" aria-label="Loading" className="grid gap-3 sm:grid-cols-2">
        {Array.from({ length: rows }).map((_, i) => <Skeleton key={i} className="h-24 w-full rounded-xl" />)}
      </div>
    )
  }
  if (kind === 'detail') {
    return (
      <div role="status" aria-label="Loading" className="space-y-4">
        <Skeleton className="h-8 w-1/2" />
        <Skeleton className="h-4 w-1/3" />
        <Skeleton className="h-32 w-full rounded-xl" />
        <Skeleton className="h-32 w-full rounded-xl" />
      </div>
    )
  }
  if (kind === 'form') {
    return (
      <div role="status" aria-label="Loading" className="max-w-lg space-y-4">
        {Array.from({ length: rows }).map((_, i) => (
          <div key={i} className="space-y-2">
            <Skeleton className="h-4 w-24" />
            <Skeleton className="h-11 w-full md:h-8" />
          </div>
        ))}
      </div>
    )
  }
  return (
    <div role="status" aria-label="Loading" className="space-y-2">
      {Array.from({ length: rows }).map((_, i) => <Skeleton key={i} className="h-12 w-full" />)}
    </div>
  )
}

function defaultIsEmpty(data: unknown): boolean {
  return Array.isArray(data) && data.length === 0
}

interface QueryStateProps<T> {
  query: QueryLike<T>
  /** Noun for the error copy, e.g. "athletes" → "Failed to load athletes." */
  resource?: string
  /** Loading placeholder: a preset or a custom node. */
  skeleton?: SkeletonKind | ReactNode
  /** Empty state shown when isEmpty(data). Omit to always render children. */
  empty?: EmptyProps
  /** Defaults to "is an empty array". */
  isEmpty?: (data: T) => boolean
  /** Custom node for a 404 (defaults to a "No <resource> found" empty state). */
  notFound?: ReactNode
  children: (data: T) => ReactNode
}

/**
 * One way to render a query's states: skeleton while loading, a retryable
 * error, an EmptyState with a call to action, or the data. Replaces the old
 * Spinner + "Failed to load" + plain-text empty pattern.
 */
export function QueryState<T>({ query, resource = 'data', skeleton = 'list', empty, isEmpty = defaultIsEmpty, notFound, children }: QueryStateProps<T>) {
  if (query.error) {
    return <QueryError error={query.error} onRetry={query.refetch ? () => { query.refetch?.() } : undefined} resource={resource} notFound={notFound} />
  }
  if (query.data === undefined) {
    return typeof skeleton === 'string' ? <PageSkeleton kind={skeleton as SkeletonKind} /> : <>{skeleton}</>
  }
  if (empty && isEmpty(query.data)) {
    return <EmptyState {...empty} />
  }
  return <>{children(query.data)}</>
}
