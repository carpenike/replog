import type { ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { ChevronLeft } from 'lucide-react'
import { usePageTitle } from '@/lib/usePageTitle'
import { cn } from '@/lib/utils'

export interface BackLink {
  /** Where the back link goes. Use the logical parent, not history.back(). */
  to: string
  /** Visible label, usually the parent page's name ("Athletes", "Ryan"). */
  label: string
}

interface PageHeaderProps {
  /** The h1. */
  title: ReactNode
  /**
   * document.title for the route. Defaults to `title` when it is a string.
   * Pass null while data loads to keep the base title.
   */
  pageTitle?: string | null
  /** Consistent back link for deep pages (one level up). */
  back?: BackLink
  /** One line under the title. */
  description?: ReactNode
  /** Right-aligned actions (buttons, links). Wraps below the title on phones. */
  actions?: ReactNode
  /** Extra content between title and actions, e.g. a badge. */
  meta?: ReactNode
  className?: string
}

/**
 * The page heading every route renders: optional back link, the h1, an
 * optional description, and right-aligned actions. It also sets
 * document.title, so pages don't call usePageTitle themselves.
 */
export function PageHeader({ title, pageTitle, back, description, actions, meta, className }: PageHeaderProps) {
  const docTitle = pageTitle !== undefined ? pageTitle : typeof title === 'string' ? title : null
  usePageTitle(docTitle)
  return (
    <header className={cn('mb-6', className)}>
      {back && (
        <Link
          to={back.to}
          className="-ml-1 mb-2 inline-flex min-h-11 items-center gap-1 rounded-md px-1 text-sm text-muted-foreground hover:text-foreground focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-ring/50 md:min-h-0 md:py-1"
        >
          <ChevronLeft className="size-4" aria-hidden="true" />
          {back.label}
        </Link>
      )}
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <h1 className="text-2xl font-bold break-words">{title}</h1>
            {meta}
          </div>
          {description && <div className="mt-1 text-sm text-muted-foreground">{description}</div>}
        </div>
        {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
      </div>
    </header>
  )
}
