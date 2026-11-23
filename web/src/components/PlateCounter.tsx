import { useMemo } from 'react'
import { Link } from 'react-router-dom'
import { Eraser, Undo2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  formatUnitWeight,
  fromKg,
  plateOptions,
  round1,
  stackedTotalKg,
  type PlateInventory,
  type WeightUnit,
} from '@/lib/plates'
import { cn } from '@/lib/utils'

export interface PlateCounterState {
  /** Plate keys on one sleeve, in the order they went on. */
  stack: string[]
  useCollars: boolean
}

interface PlateCounterProps {
  inventory: PlateInventory
  /** The athlete's logging unit; the total shows in it first. */
  unit: WeightUnit
  state: PlateCounterState
  onChange: (next: PlateCounterState) => void
  /** Puts the total into a set's weight field. Omit outside a session. */
  onUse?: (weight: number) => void
  className?: string
}

function unitLabel(u: WeightUnit): string {
  return u === 'kg' ? 'kg' : 'lb'
}

/**
 * Count what's on the bar: tap each plate as it goes on one sleeve (the other
 * sleeve mirrors it) and read the total in lb and kg. The reverse of the
 * plate suggestion: the plates are known, the weight is the answer.
 */
export function PlateCounter({ inventory, unit, state, onChange, onUse, className }: PlateCounterProps) {
  const options = useMemo(() => plateOptions(inventory), [inventory])
  const byKey = useMemo(() => new Map(options.map(o => [o.key, o])), [options])
  const { stack, useCollars } = state

  const totalKg = stackedTotalKg(inventory, stack, useCollars)
  const totalLb = round1(fromKg(totalKg, 'lbs'))
  const totalKgR = round1(totalKg)
  const primary = unit === 'kg' ? totalKgR : totalLb
  const secondary = unit === 'kg' ? `${totalLb} lb` : `${totalKgR} kg`

  const used = (key: string) => stack.filter(k => k === key).length
  const hasCollars = inventory.collar.weight > 0

  function add(key: string) {
    const opt = byKey.get(key)
    if (!opt || used(key) >= opt.pairs) return
    onChange({ ...state, stack: [...stack, key] })
  }
  function removeAt(i: number) {
    onChange({ ...state, stack: stack.filter((_, j) => j !== i) })
  }

  return (
    <div className={cn('space-y-4', className)}>
      <div className="rounded-lg bg-muted/50 px-4 py-3 text-center" aria-live="polite">
        <p className="text-xs font-medium text-muted-foreground">Total on the bar</p>
        <p className="text-4xl font-semibold tabular-nums" data-testid="plate-total">
          {primary} <span className="text-2xl font-medium">{unitLabel(unit)}</span>
        </p>
        <p className="text-sm text-muted-foreground tabular-nums">{secondary}</p>
      </div>

      <div>
        <div className="mb-1 flex items-center justify-between gap-2">
          <p className="text-xs font-medium text-muted-foreground">
            Each side{stack.length > 0 && <span> (tap to remove)</span>}
          </p>
          <div className="flex gap-1">
            <Button
              variant="ghost"
              size="sm"
              disabled={stack.length === 0}
              onClick={() => removeAt(stack.length - 1)}
              aria-label="Take off the last plate"
            >
              <Undo2 aria-hidden="true" /> Undo
            </Button>
            <Button
              variant="ghost"
              size="sm"
              disabled={stack.length === 0}
              onClick={() => onChange({ ...state, stack: [] })}
              aria-label="Clear the bar"
            >
              <Eraser aria-hidden="true" /> Clear
            </Button>
          </div>
        </div>
        {stack.length === 0 ? (
          <p className="rounded-md border border-dashed px-3 py-2 text-sm text-muted-foreground">
            Empty bar. Tap the plates below as you load one side.
          </p>
        ) : (
          <ul className="flex flex-wrap gap-1.5" aria-label="Plates on each side">
            {stack.map((key, i) => {
              const opt = byKey.get(key)
              if (!opt) return null
              return (
                <li key={`${key}-${i}`}>
                  <button
                    type="button"
                    onClick={() => removeAt(i)}
                    aria-label={`Take off ${formatUnitWeight(opt)}`}
                    className="min-h-9 rounded-md bg-primary/10 px-2.5 text-sm font-medium tabular-nums text-primary outline-none hover:bg-primary/20 focus-visible:ring-2 focus-visible:ring-ring/50"
                  >
                    {formatUnitWeight(opt)}
                  </button>
                </li>
              )
            })}
          </ul>
        )}
      </div>

      <div>
        <p className="mb-1 text-xs font-medium text-muted-foreground">Add a plate (each side)</p>
        <div className="grid grid-cols-4 gap-2" role="group" aria-label="Plates you own">
          {options.map(opt => {
            const left = opt.pairs - used(opt.key)
            return (
              <Button
                key={opt.key}
                variant="outline"
                size="touch"
                className="relative px-1 tabular-nums"
                disabled={left <= 0}
                onClick={() => add(opt.key)}
                aria-label={`Add ${formatUnitWeight(opt)}${opt.pairs > 1 ? `, ${left} of ${opt.pairs} pairs left` : ''}`}
              >
                {opt.weight}
                <span className="text-xs text-muted-foreground">{opt.unit === 'kg' ? 'kg' : 'lb'}</span>
                {opt.pairs > 1 && left > 0 && (
                  <span className="absolute top-0.5 right-1 text-[10px] text-muted-foreground" aria-hidden="true">×{left}</span>
                )}
              </Button>
            )
          })}
        </div>
      </div>

      <div className="flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
        <span>
          {formatUnitWeight(inventory.bar)} bar
          {useCollars && hasCollars && ` + ${formatUnitWeight(inventory.collar)} collars`}
        </span>
        {hasCollars && (
          <label className="flex min-h-9 items-center gap-2">
            <Checkbox
              checked={useCollars}
              onCheckedChange={v => onChange({ ...state, useCollars: v === true })}
            />
            Collars on
          </label>
        )}
      </div>

      {onUse && (
        <Button size="touch" className="w-full" onClick={() => onUse(primary)}>
          Use {primary} {unitLabel(unit)} for this set
        </Button>
      )}

      <Link to="/preferences#plate-setup" className="block text-xs text-primary underline-offset-4 hover:underline">
        Edit plate setup
      </Link>
    </div>
  )
}
