import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { ArrowLeftRight, Check } from 'lucide-react'
import { api } from '@/api/client'
import { Sheet, SheetContent, SheetDescription, SheetFooter, SheetHeader, SheetTitle, SheetTrigger } from '@/components/ui/sheet'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'

interface SwapExerciseSheetProps {
  athleteId: number
  exerciseId: number
  exerciseName: string
  pending: boolean
  onSwap: (toExerciseId: number, reason: string) => void
  className?: string
}

/**
 * "Swap exercise": pick a substitute with the same movement pattern. The
 * swap is logged on the workout with an optional reason for the coach's
 * review. The program is not changed.
 */
export function SwapExerciseSheet({ athleteId, exerciseId, exerciseName, pending, onSwap, className }: SwapExerciseSheetProps) {
  const [open, setOpen] = useState(false)
  const [choice, setChoice] = useState<number | null>(null)
  const [reason, setReason] = useState('')
  const subs = useQuery({
    queryKey: ['exercise-substitutes', athleteId, exerciseId],
    queryFn: () => api.listExerciseSubstitutes(athleteId, exerciseId),
    enabled: open,
    staleTime: 10 * 60_000,
  })
  const chosen = subs.data?.find(e => e.id === choice)
  const reasonId = `swap-reason-${exerciseId}`

  return (
    <Sheet open={open} onOpenChange={next => { setOpen(next); if (!next) { setChoice(null); setReason('') } }}>
      <SheetTrigger
        render={
          <Button variant="ghost" size="sm" className={cn('h-11 text-xs text-muted-foreground md:h-8', className)} aria-label={`Swap ${exerciseName}`} />
        }
      >
        <ArrowLeftRight aria-hidden="true" /> Swap
      </SheetTrigger>
      <SheetContent side="bottom" className="max-h-[85dvh] overflow-y-auto pb-[calc(1rem+env(safe-area-inset-bottom))] md:mx-auto md:max-w-2xl md:rounded-t-xl">
        <SheetHeader>
          <SheetTitle>Swap {exerciseName}</SheetTitle>
          <SheetDescription>
            Pick a lift with the same movement pattern. Your coach sees the swap in review; your program stays the same.
          </SheetDescription>
        </SheetHeader>
        <div className="space-y-4 px-4">
          {subs.isLoading ? (
            <div className="space-y-2">
              <Skeleton className="h-11 w-full" />
              <Skeleton className="h-11 w-full" />
            </div>
          ) : subs.isError ? (
            <p className="text-sm text-destructive">Couldn't load substitutes. Check your connection and try again.</p>
          ) : (subs.data ?? []).length === 0 ? (
            <p className="text-sm text-muted-foreground">
              No lifts share a movement pattern with {exerciseName} yet. Ask your coach to tag the catalog, or log it as prescribed.
            </p>
          ) : (
            <ul role="radiogroup" aria-label="Substitute exercise" className="divide-y divide-border rounded-lg border border-border">
              {subs.data?.map(e => (
                <li key={e.id}>
                  <button
                    type="button"
                    role="radio"
                    aria-checked={choice === e.id}
                    onClick={() => setChoice(e.id)}
                    className="flex min-h-11 w-full items-center justify-between gap-2 px-3 py-2 text-left text-sm outline-none hover:bg-muted/40 focus-visible:ring-2 focus-visible:ring-ring/50"
                  >
                    <span>{e.name}</span>
                    {choice === e.id && <Check className="size-4 text-primary" aria-hidden="true" />}
                  </button>
                </li>
              ))}
            </ul>
          )}
          <div>
            <Label htmlFor={reasonId}>Reason (optional)</Label>
            <Textarea
              id={reasonId}
              value={reason}
              onChange={e => setReason(e.target.value)}
              maxLength={500}
              rows={2}
              placeholder="e.g. Knee felt off, no rack free"
            />
          </div>
        </div>
        <SheetFooter>
          <Button
            size="touch"
            disabled={!chosen || pending}
            onClick={() => { if (chosen) { onSwap(chosen.id, reason.trim()); setOpen(false); setChoice(null); setReason('') } }}
          >
            {chosen ? `Swap to ${chosen.name}` : 'Pick a substitute'}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
