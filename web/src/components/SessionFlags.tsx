import { AlertTriangle, ArrowLeftRight, HeartPulse } from 'lucide-react'
import type { ExerciseSwap, RecoveryCheckin } from '@/api/types'

interface SessionFlagsProps {
  readiness?: RecoveryCheckin | null
  swaps?: ExerciseSwap[]
}

/** "Sleep 5h · Soreness 3 · Mood 7" from a readiness check-in. */
function readinessSummary(c: RecoveryCheckin): string {
  return [
    c.sleep_hours != null ? `Sleep ${c.sleep_hours}h` : null,
    c.soreness != null ? `Soreness ${c.soreness}/10` : null,
    c.mood != null ? `Mood ${c.mood}/10` : null,
    c.energy != null ? `Energy ${c.energy}/10` : null,
  ].filter(Boolean).join(' · ') || 'No answers'
}

/**
 * What the athlete reported around this session, for the coach's review:
 * the readiness check-in and any exercise swaps. It only informs; nothing
 * here changed the program.
 */
export function SessionFlags({ readiness, swaps = [] }: SessionFlagsProps) {
  if (!readiness && swaps.length === 0) return null
  return (
    <section aria-label="Session notes for the coach" className="mb-4 space-y-2 rounded-lg border border-border bg-card p-3 text-sm">
      {readiness && (
        <div className="flex items-start gap-2">
          <HeartPulse className="mt-0.5 size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
          <div className="min-w-0">
            <p>
              <span className="font-medium">Readiness check-in:</span> {readinessSummary(readiness)}
            </p>
            {readiness.low_readiness && (
              <p className="mt-0.5 inline-flex items-center gap-1 rounded-full bg-warning/10 px-2 py-0.5 text-xs text-warning-strong">
                <AlertTriangle className="size-3" aria-hidden="true" /> Low readiness
              </p>
            )}
          </div>
        </div>
      )}
      {swaps.map(s => (
        <div key={s.id} className="flex items-start gap-2">
          <ArrowLeftRight className="mt-0.5 size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
          <p className="min-w-0">
            <span className="font-medium">Swapped</span> {s.from_exercise_name} → {s.to_exercise_name}
            {s.reason && <span className="text-muted-foreground"> · “{s.reason}”</span>}
          </p>
        </div>
      ))}
      <p className="text-xs text-muted-foreground">Reported by the athlete. The program was not changed.</p>
    </section>
  )
}
