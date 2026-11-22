import type { ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { ChevronRight } from 'lucide-react'
import { cn } from '@/lib/utils'

/**
 * Phone-width replacement for a wide table: a stacked list of cards shown
 * below md. Pair it with the table wrapped in `hidden md:block`
 * (see {@link DesktopOnly}).
 */
export function CardList({ children, className, label }: { children: ReactNode; className?: string; label?: string }) {
  return (
    <ul aria-label={label} className={cn('space-y-2 md:hidden', className)}>
      {children}
    </ul>
  )
}

interface CardListItemProps {
  /** Makes the whole card a real link (keyboard reachable). */
  to?: string
  title: ReactNode
  /** Secondary line under the title. */
  subtitle?: ReactNode
  /** Right-aligned summary (a badge, a number). */
  trailing?: ReactNode
  /** Extra rows (key/value details) under the title. */
  children?: ReactNode
  /** Actions rendered outside the link so buttons stay separate targets. */
  actions?: ReactNode
  className?: string
}

export function CardListItem({ to, title, subtitle, trailing, children, actions, className }: CardListItemProps) {
  const body = (
    <div className="flex items-start justify-between gap-3">
      <div className="min-w-0 flex-1">
        <div className="font-medium break-words">{title}</div>
        {subtitle && <div className="text-sm text-muted-foreground">{subtitle}</div>}
        {children && <div className="mt-2 text-sm">{children}</div>}
      </div>
      {trailing && <div className="shrink-0 text-right text-sm">{trailing}</div>}
      {to && <ChevronRight className="mt-0.5 size-4 shrink-0 text-muted-foreground" aria-hidden="true" />}
    </div>
  )
  return (
    <li className={cn('rounded-lg border border-border bg-card', className)}>
      {to ? (
        <Link to={to} className="block min-h-11 rounded-lg p-3 transition-colors hover:bg-accent/50 focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-ring/50">
          {body}
        </Link>
      ) : (
        <div className="p-3">{body}</div>
      )}
      {actions && <div className="flex flex-wrap gap-2 border-t border-border px-3 py-2">{actions}</div>}
    </li>
  )
}

/** Key/value row for a CardListItem body. */
export function CardListDetail({ label, children }: { label: ReactNode; children: ReactNode }) {
  return (
    <div className="flex justify-between gap-3">
      <span className="text-muted-foreground">{label}</span>
      <span className="text-right">{children}</span>
    </div>
  )
}

/** Wrapper that shows its content (usually the wide table) from md up. */
export function DesktopOnly({ children }: { children: ReactNode }) {
  return <div className="hidden md:block">{children}</div>
}

/**
 * A table-cell link stretched over its whole row, so a clickable row is a
 * real, keyboard-reachable link. The row needs `relative`.
 */
export function RowLink({ to, children, className }: { to: string; children: ReactNode; className?: string }) {
  return (
    <Link
      to={to}
      className={cn(
        'font-medium hover:text-primary after:absolute after:inset-0 after:content-[""] focus-visible:outline-none focus-visible:after:ring-3 focus-visible:after:ring-ring/50 focus-visible:after:rounded-md',
        className,
      )}
    >
      {children}
    </Link>
  )
}
