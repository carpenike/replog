// Warm-up ramp generation. Pure so the ramp rules are unit-tested.
//
// A ramp climbs from the empty bar to the first working set: bar, 40%, 60%,
// 80% of the working weight by default, each rounded to a weight the plate
// inventory can actually load (kg plates shown in the logging unit). The
// ramp is a suggestion the athlete can edit; it never touches the program.

import { emptyBarKg, fromKg, round1, roundToLoadable, type PlateInventory, type WeightUnit } from './plates'

export interface RampStep {
  /** Fraction of the working weight; 0 for the empty bar. */
  pct: number
  reps: number
}

export interface WarmupSet {
  /** "Bar", "40%", ... */
  label: string
  /** Loadable weight in the logging unit, rounded to 0.1. */
  weight: number
  reps: number
}

export const DEFAULT_RAMP: RampStep[] = [
  { pct: 0, reps: 10 },
  { pct: 0.4, reps: 5 },
  { pct: 0.6, reps: 3 },
  { pct: 0.8, reps: 2 },
]

/**
 * Build a loadable warm-up ramp to `working` (in the logging unit). Steps
 * that round to the same weight as an earlier step, or to the working
 * weight or above, are dropped, so the ramp only ever climbs. A working
 * weight at or below the empty bar needs no ramp and returns [].
 */
export function generateWarmupRamp(
  working: number,
  inv: PlateInventory,
  unit: WeightUnit,
  steps: RampStep[] = DEFAULT_RAMP,
): WarmupSet[] {
  const bar = round1(fromKg(emptyBarKg(inv), unit))
  if (!(working > bar + 0.05)) return []
  const out: WarmupSet[] = []
  let last = -Infinity
  for (const step of steps) {
    const weight = step.pct === 0 ? bar : roundToLoadable(working * step.pct, inv, unit)
    if (weight <= last + 0.05 || weight >= working - 0.05) continue
    out.push({ label: step.pct === 0 ? 'Bar' : `${Math.round(step.pct * 100)}%`, weight, reps: step.reps })
    last = weight
  }
  return out
}
