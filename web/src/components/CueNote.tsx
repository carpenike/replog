import { useState } from 'react'
import { Info } from 'lucide-react'
import { cn } from '@/lib/utils'

/** Past this many characters the cue starts clamped to two lines. */
const LONG_CUE = 110

/**
 * A coach's cue for one exercise, shown once. Long cues start clamped to two
 * lines with a "More" toggle so they don't push the set rows off screen.
 */
export function CueNote({ text, className }: { text: string; className?: string }) {
  const long = text.length > LONG_CUE
  const [open, setOpen] = useState(false)
  return (
    <div className={cn('mt-0.5 flex items-start gap-1.5 text-xs text-muted-foreground', className)}>
      <Info aria-hidden="true" className="mt-0.5 size-3.5 shrink-0" />
      <p className="min-w-0">
        <span className="sr-only">Coaching cue: </span>
        <span className={cn(long && !open && 'line-clamp-2')}>{text}</span>
        {long && (
          <button
            type="button"
            aria-expanded={open}
            onClick={() => setOpen(o => !o)}
            className="ml-1 inline-flex min-h-11 items-center font-medium text-primary outline-none focus-visible:underline md:min-h-0"
          >
            {open ? 'Less' : 'More'}
          </button>
        )}
      </p>
    </div>
  )
}
