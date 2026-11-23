import { useId, useState } from 'react'
import { Info } from 'lucide-react'
import { cn } from '@/lib/utils'
import { distinctNotes, summarizeSets, type PrescribedSetLike } from '@/lib/setSummary'

interface Props {
  name: string
  sets: PrescribedSetLike[]
  /** Superset position (B1) or a lone group key. */
  label?: string
  /** Rest is shown on the group header instead. */
  omitRest?: boolean
  /** Inside a warm-up block, so the per-line "Warm-up" tag is redundant. */
  inWarmup?: boolean
}

/**
 * One prescribed exercise, read-only: identical sets on one line and the
 * coaching cue once. The cue sits behind an info button on phones and shows
 * inline from md up.
 */
export function PrescribedExercise({ name, sets, label, omitRest, inWarmup }: Props) {
  const [open, setOpen] = useState(false)
  const cueId = useId()
  const lines = summarizeSets(sets, { omitRest })
  const notes = distinctNotes(sets)
  return (
    <div className="py-1">
      <div className="flex items-start gap-2">
        {label && (
          <span className={cn(
            'mt-0.5 shrink-0 rounded px-1.5 py-0.5 text-xs font-semibold',
            inWarmup ? 'bg-warning/15 text-warning-strong' : 'bg-primary/15 text-primary',
          )}>{label}</span>
        )}
        <div className="min-w-0 flex-1">
          <p className="text-sm font-medium">{name}</p>
          <ul className="text-xs text-muted-foreground tabular-nums">
            {lines.map((l, i) => (
              <li key={i}>
                {l.warmup && !inWarmup && <span className="mr-1 font-medium text-warning-strong">Warm-up</span>}
                <span className="font-semibold text-foreground">{l.head}</span>
                {l.detail.map(d => <span key={d}> · {d}</span>)}
              </li>
            ))}
          </ul>
        </div>
        {notes.length > 0 && (
          <button
            type="button"
            aria-expanded={open}
            aria-controls={cueId}
            aria-label={`${open ? 'Hide' : 'Show'} coaching cue for ${name}`}
            onClick={() => setOpen(o => !o)}
            className={cn(
              'inline-flex size-11 shrink-0 items-center justify-center rounded-full text-muted-foreground outline-none hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50 md:hidden',
              open && 'text-primary',
            )}
          >
            <Info aria-hidden="true" className="size-4" />
          </button>
        )}
      </div>
      {notes.length > 0 && (
        <div
          id={cueId}
          className={cn(
            'mt-1 space-y-1 rounded-md bg-muted px-2 py-1.5 text-xs text-muted-foreground md:mt-0.5 md:block md:bg-transparent md:p-0 md:italic',
            label && 'md:ml-9',
            open ? 'block' : 'hidden',
          )}
        >
          {notes.map(n => <p key={n}>{n}</p>)}
        </div>
      )}
    </div>
  )
}
