import { useState } from 'react'
import { Link } from 'react-router-dom'
import { Disc3 } from 'lucide-react'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle, SheetTrigger } from '@/components/ui/sheet'
import { Tabs, TabsList, TabsPanel, TabsTab } from '@/components/ui/tabs'
import { PlateCounter, type PlateCounterState } from '@/components/PlateCounter'
import { Button } from '@/components/ui/button'
import {
  emptyBarKg,
  formatPerSide,
  formatUnitWeight,
  fromKg,
  maxLoadable,
  nearestLoadable,
  round1,
  solvePlates,
  type PlateInventory,
  type WeightUnit,
} from '@/lib/plates'
import { cn } from '@/lib/utils'

function unitLabel(u: WeightUnit): string {
  return u === 'kg' ? 'kg' : 'lb'
}

function fmt(v: number, u: WeightUnit): string {
  return `${round1(v)} ${unitLabel(u)}`
}

interface PlateCalculatorProps {
  /** Target weight in the logging unit, when the set has one. */
  target?: number
  inventory: PlateInventory
  unit: WeightUnit
  /** Puts a weight into the set's weight field. */
  onUse: (weight: number) => void
  /** Accessible context, e.g. "Set 2". */
  label: string
  className?: string
}

/**
 * Plate math for one set's weight field, in a bottom sheet with two tabs:
 * "Count plates" totals what's actually stacked on the bar (kg plates shown
 * in lb and kg) and puts it in the set, and "Plates for" suggests what to load
 * for the set's weight. The row hint shows the suggestion when there's a
 * weight, and a "Count plates" link when there isn't.
 */
export function PlateCalculator({ target, inventory, unit, onUse, label, className }: PlateCalculatorProps) {
  const [open, setOpen] = useState(false)
  const [tab, setTab] = useState<'count' | 'suggest'>('count')
  const [counter, setCounter] = useState<PlateCounterState>({ stack: [], useCollars: inventory.useCollars })
  const hasTarget = target != null && target > 0
  const solution = hasTarget ? solvePlates(target, inventory, unit) : null
  const summary = !solution ? 'Count plates' : solution.belowBar ? 'Under the bar' : formatPerSide(solution.perSide)

  function use(weight: number) {
    onUse(weight)
    setOpen(false)
  }

  return (
    <Sheet open={open} onOpenChange={setOpen}>
      <SheetTrigger
        render={
          <button
            type="button"
            aria-label={solution
              ? `Plates for ${label}: ${summary} per side, ${fmt(solution.loaded, unit)} loaded`
              : `Count plates for ${label}`}
            className={cn(
              'flex h-8 w-full min-w-0 items-center gap-1.5 rounded-md px-1 text-left text-xs text-muted-foreground outline-none transition-colors hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/50',
              className,
            )}
          />
        }
      >
        <Disc3 className="size-3.5 shrink-0" aria-hidden="true" />
        <span className="truncate">
          {summary}
          {solution && !solution.belowBar && solution.perSide.length > 0 && ' /side'}
          {solution && !solution.exact && <span className="text-foreground"> · {fmt(solution.loaded, unit)}</span>}
        </span>
      </SheetTrigger>
      <SheetContent side="bottom" className="max-h-[90dvh] overflow-y-auto pb-[calc(1rem+env(safe-area-inset-bottom))] md:mx-auto md:max-w-md md:rounded-t-xl">
        <SheetHeader>
          <SheetTitle>Plates · {label}</SheetTitle>
          <SheetDescription>Count what's on the bar, or see what to load.</SheetDescription>
        </SheetHeader>
        <div className="px-4">
          <Tabs value={tab} onValueChange={v => setTab(v as 'count' | 'suggest')}>
            <TabsList className="w-full md:w-full">
              <TabsTab value="count" className="flex-1">Count plates</TabsTab>
              <TabsTab value="suggest" className="flex-1" disabled={!hasTarget}>
                {hasTarget ? `Plates for ${fmt(target, unit)}` : 'Plates for a weight'}
              </TabsTab>
            </TabsList>
            <TabsPanel value="count">
              <PlateCounter inventory={inventory} unit={unit} state={counter} onChange={setCounter} onUse={use} />
            </TabsPanel>
            {hasTarget && (
              <TabsPanel value="suggest">
                <PlateCalculatorBody target={target} inventory={inventory} unit={unit} onUse={use} />
              </TabsPanel>
            )}
          </Tabs>
        </div>
      </SheetContent>
    </Sheet>
  )
}

function PlateCalculatorBody({ target, inventory, unit, onUse }: { target: number; inventory: PlateInventory; unit: WeightUnit; onUse: (weight: number) => void }) {
  const s = solvePlates(target, inventory, unit)
  const near = nearestLoadable(target, inventory, unit)
  const bar = round1(fromKg(emptyBarKg(inventory), unit))
  const loadedKg = round1(s.loadedKg)
  // The greedy load is usually also the nearest weight below; don't offer it twice.
  const below = !s.exact && near.below != null && Math.abs(near.below - s.loaded) < 0.05 ? undefined : near.below
  const above = near.above
  const mixedUnits = inventory.bar.unit !== unit || inventory.plates.some(p => p.unit !== unit)

  return (
    <div className="space-y-3">
      <p className="text-xs text-muted-foreground">
        {formatUnitWeight(inventory.bar)} bar
        {inventory.useCollars && inventory.collar.weight > 0 && ` + ${formatUnitWeight(inventory.collar)} collars`}
      </p>

      {s.belowBar ? (
        <p className="text-sm">Lighter than the empty bar ({fmt(bar, unit)}).</p>
      ) : s.perSide.length === 0 ? (
        <p className="text-sm">Empty bar.</p>
      ) : (
        <div>
          <p className="mb-1 text-xs font-medium text-muted-foreground">Each side</p>
          <ul className="flex flex-wrap gap-1.5" aria-label="Plates on each side">
            {s.perSide.map(p => (
              <li key={`${p.weight}${p.unit}`} className="rounded-md bg-muted px-2 py-1 text-sm font-medium tabular-nums">
                {formatUnitWeight(p)}{p.count > 1 && <span className="text-muted-foreground"> ×{p.count}</span>}
              </li>
            ))}
          </ul>
        </div>
      )}

      <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-sm">
        <dt className="text-muted-foreground">Loaded</dt>
        <dd className="font-medium tabular-nums">
          {fmt(s.loaded, unit)}
          {mixedUnits && unit !== 'kg' && <span className="text-muted-foreground"> ({loadedKg} kg)</span>}
        </dd>
        <dt className="text-muted-foreground">Difference</dt>
        <dd className="tabular-nums">
          {s.exact ? 'Exact' : `${s.difference > 0 ? '+' : '−'}${Math.abs(s.difference)} ${unitLabel(unit)} from target`}
        </dd>
      </dl>

      {s.overCapacity && (
        <p className="text-xs text-warning-strong">
          Not enough plates for this weight. The most this setup loads is {fmt(maxLoadable(inventory, unit), unit)}.
        </p>
      )}

      <div className="flex flex-col gap-2">
        {!s.exact && (
          <Button size="touch" onClick={() => onUse(s.loaded)}>
            Use {fmt(s.loaded, unit)}
          </Button>
        )}
        {(below != null || above != null) && (
          <div className="grid grid-cols-2 gap-2">
            {below != null ? (
              <Button variant="outline" size="touch" onClick={() => onUse(below)} aria-label={`Use ${fmt(below, unit)}, the nearest loadable weight below`}>
                ↓ {fmt(below, unit)}
              </Button>
            ) : <span />}
            {above != null && (
              <Button variant="outline" size="touch" onClick={() => onUse(above)} aria-label={`Use ${fmt(above, unit)}, the nearest loadable weight above`}>
                ↑ {fmt(above, unit)}
              </Button>
            )}
          </div>
        )}
      </div>

      <Link to="/preferences#plate-setup" className="block text-xs text-primary underline-offset-4 hover:underline">
        Edit plate setup
      </Link>
    </div>
  )
}
