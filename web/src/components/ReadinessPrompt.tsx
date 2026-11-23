import { useState } from 'react'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

export interface ReadinessAnswers {
  sleep_hours?: number
  soreness?: number
  mood?: number
}

interface Choice {
  value: number
  label: string
}

const SLEEP_CHOICES: Choice[] = [
  { value: 4, label: '≤4' },
  { value: 5, label: '5' },
  { value: 6, label: '6' },
  { value: 7, label: '7' },
  { value: 8, label: '8' },
  { value: 9, label: '9+' },
]

const SCALE: Choice[] = Array.from({ length: 10 }, (_, i) => ({ value: i + 1, label: String(i + 1) }))

function ChoiceGroup({ id, label, low, high, choices, value, onChange }: {
  id: string
  label: string
  low?: string
  high?: string
  choices: Choice[]
  value?: number
  onChange: (v: number | undefined) => void
}) {
  return (
    <fieldset>
      <legend id={id} className="mb-1.5 text-sm font-medium">{label}</legend>
      <div role="radiogroup" aria-labelledby={id} className={cn('grid gap-1.5', choices.length > 6 ? 'grid-cols-5' : 'grid-cols-6')}>
        {choices.map(c => {
          const selected = value === c.value
          return (
            <button
              key={c.value}
              type="button"
              role="radio"
              aria-checked={selected}
              // Tapping the selected answer clears it: every question is optional.
              onClick={() => onChange(selected ? undefined : c.value)}
              className={cn(
                'h-11 rounded-lg border text-sm font-medium tabular-nums outline-none transition-colors focus-visible:ring-2 focus-visible:ring-ring/50',
                selected ? 'border-primary bg-primary text-primary-foreground' : 'border-border bg-card hover:bg-muted',
              )}
            >
              {c.label}
            </button>
          )
        })}
      </div>
      {(low || high) && (
        <div className="mt-1 flex justify-between text-[11px] text-muted-foreground" aria-hidden="true">
          <span>{low}</span>
          <span>{high}</span>
        </div>
      )}
    </fieldset>
  )
}

interface ReadinessPromptProps {
  open: boolean
  pending: boolean
  onSubmit: (answers: ReadinessAnswers) => void
  onSkip: () => void
}

/**
 * Three quick questions when a session opens. The answers are saved as a
 * recovery check-in; a low answer flags the session for the coach in
 * review. Nothing here changes the plan.
 */
export function ReadinessPrompt({ open, pending, onSubmit, onSkip }: ReadinessPromptProps) {
  const [answers, setAnswers] = useState<ReadinessAnswers>({})
  const answered = answers.sleep_hours != null || answers.soreness != null || answers.mood != null

  return (
    <Dialog open={open} onOpenChange={next => { if (!next) onSkip() }}>
      <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Quick check-in</DialogTitle>
          <DialogDescription>
            Three taps before you lift. Your coach sees this in review. It doesn't change today's plan.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <ChoiceGroup
            id="readiness-sleep"
            label="Hours of sleep last night"
            choices={SLEEP_CHOICES}
            value={answers.sleep_hours}
            onChange={v => setAnswers(a => ({ ...a, sleep_hours: v }))}
          />
          <ChoiceGroup
            id="readiness-soreness"
            label="How sore are you?"
            low="1 · Not at all"
            high="10 · Very sore"
            choices={SCALE}
            value={answers.soreness}
            onChange={v => setAnswers(a => ({ ...a, soreness: v }))}
          />
          <ChoiceGroup
            id="readiness-mood"
            label="How's your mood?"
            low="1 · Rough"
            high="10 · Great"
            choices={SCALE}
            value={answers.mood}
            onChange={v => setAnswers(a => ({ ...a, mood: v }))}
          />
        </div>
        <DialogFooter>
          <Button variant="ghost" size="touch" onClick={onSkip}>Skip</Button>
          <Button size="touch" disabled={!answered || pending} onClick={() => onSubmit(answers)}>
            {pending ? 'Saving…' : 'Save check-in'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
