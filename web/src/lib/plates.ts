// Barbell plate math. Pure so the loading rules are unit-tested.
//
// The bar, the collars and each plate carry their own unit, separate from
// the athlete's logging unit: Ryan loads kg competition plates but logs in
// lbs. Everything is solved in kg and converted back to the logging unit at
// the end, so "225 lb" on a 20 kg bar comes back as the weight actually on
// the bar ("224.9 lb") plus the difference from the target.

export const LB_PER_KG = 2.20462

export type WeightUnit = 'kg' | 'lbs'

export interface UnitWeight {
  weight: number
  unit: WeightUnit
}

export interface PlateStock extends UnitWeight {
  /** Matched pairs the gym owns (one plate per sleeve per pair). */
  pairs: number
}

export interface PlateInventory {
  bar: UnitWeight
  /** Weight of one collar. */
  collar: UnitWeight
  useCollars: boolean
  plates: PlateStock[]
}

/** One plate size loaded on each sleeve. */
export interface PlateLoad extends UnitWeight {
  count: number
}

export interface PlateSolution {
  /** Plates on each sleeve, heaviest first. */
  perSide: PlateLoad[]
  /** Everything on the bar (bar + collars + plates), in kg. */
  loadedKg: number
  /** The loaded weight in the logging unit, rounded to 0.1. */
  loaded: number
  /** loaded − target, in the logging unit, rounded to 0.1. */
  difference: number
  /** True when the loaded weight matches the target (within 0.05). */
  exact: boolean
  /** Target is lighter than the empty bar (plus collars). */
  belowBar: boolean
  /** The inventory ran out before reaching the target. */
  overCapacity: boolean
}

const EPS = 1e-6

/** Normalize the API's unit strings ("lb", "lbs", "kg"). */
export function normalizeUnit(u: string | null | undefined): WeightUnit {
  return u === 'kg' ? 'kg' : 'lbs'
}

export function toKg(w: number, unit: WeightUnit): number {
  return unit === 'kg' ? w : w / LB_PER_KG
}

export function fromKg(kg: number, unit: WeightUnit): number {
  return unit === 'kg' ? kg : kg * LB_PER_KG
}

/** Round to one decimal place, avoiding -0. */
export function round1(v: number): number {
  const r = Math.round(v * 10) / 10
  return Object.is(r, -0) ? 0 : r
}

/** Bar plus (when used) two collars, in kg. */
export function emptyBarKg(inv: PlateInventory): number {
  const collars = inv.useCollars ? 2 * toKg(inv.collar.weight, inv.collar.unit) : 0
  return toKg(inv.bar.weight, inv.bar.unit) + collars
}

function sortedPlates(inv: PlateInventory): (PlateStock & { kg: number })[] {
  return inv.plates
    .filter(p => p.weight > 0 && p.pairs > 0)
    .map(p => ({ ...p, kg: toKg(p.weight, p.unit) }))
    .sort((a, b) => b.kg - a.kg)
}

// Plates heavier than a 20 kg / 45 lb plate (25 kg, 55 lb) are not the first
// plate anyone puts on the bar. The solver loads the 20s first and reaches for
// a heavier plate only when the 20s and smaller can't get as close.
const LEAD_PLATE_MAX_KG = 20.5

function greedyPerSide(remainingKg: number, plates: (PlateStock & { kg: number })[]) {
  const perSide: PlateLoad[] = []
  let remaining = remainingKg
  let sideKg = 0
  for (const p of plates) {
    const n = Math.min(p.pairs, Math.floor((remaining + EPS) / p.kg))
    if (n > 0) {
      perSide.push({ weight: p.weight, unit: p.unit, count: n })
      remaining -= n * p.kg
      sideKg += n * p.kg
    }
  }
  return { perSide, remaining, sideKg }
}

/**
 * Solve the plates for a target weight given in the logging unit. Per side,
 * take the heaviest plate that still fits under the remaining weight, as
 * many as the inventory allows (greedy), preferring 20 kg / 45 lb plates over
 * heavier ones when both reach the same weight. The loaded weight never
 * exceeds the target by more than rounding; use nearestLoadable for the next
 * weight up.
 */
export function solvePlates(target: number, inv: PlateInventory, unit: WeightUnit): PlateSolution {
  const targetKg = toKg(Math.max(0, target), unit)
  const barKg = emptyBarKg(inv)
  const half = (targetKg - barKg) / 2
  const belowBar = half < -EPS
  const plates = sortedPlates(inv)
  let side = { perSide: [] as PlateLoad[], remaining: half, sideKg: 0 }
  if (!belowBar) {
    side = greedyPerSide(half, plates)
    const leadFirst = plates.filter(p => p.kg <= LEAD_PLATE_MAX_KG)
    if (leadFirst.length < plates.length) {
      const alt = greedyPerSide(half, leadFirst)
      if (alt.sideKg >= side.sideKg - EPS) side = alt
    }
  }
  const { perSide, remaining, sideKg } = side
  const loadedKg = barKg + 2 * sideKg
  const loaded = round1(fromKg(loadedKg, unit))
  const difference = round1(fromKg(loadedKg, unit) - target)
  const smallest = plates.at(-1)?.kg ?? 0
  return {
    perSide,
    loadedKg,
    loaded,
    difference,
    exact: Math.abs(difference) < 0.05,
    belowBar,
    // Greedy takes any plate that fits while stock lasts, so a remainder of
    // at least the smallest plate means the fitting sizes ran out.
    overCapacity: !belowBar && smallest > 0 && remaining >= smallest - EPS,
  }
}

/** One plate size the athlete owns, as offered by the plate counter. */
export interface PlateOption extends PlateStock {
  /** Stable key, e.g. "20kg". */
  key: string
  kg: number
}

/** The owned plate sizes, heaviest first, keyed for the plate counter. */
export function plateOptions(inv: PlateInventory): PlateOption[] {
  return sortedPlates(inv).map(p => ({ ...p, key: `${p.weight}${p.unit}` }))
}

/**
 * Total on the bar for plates the athlete has stacked. `stack` lists the plate
 * keys on one sleeve in the order they went on; the other sleeve mirrors it.
 * Returns kg; convert with fromKg for display.
 */
export function stackedTotalKg(inv: PlateInventory, stack: string[], useCollars = inv.useCollars): number {
  const byKey = new Map(plateOptions(inv).map(p => [p.key, p.kg]))
  const side = stack.reduce((sum, k) => sum + (byKey.get(k) ?? 0), 0)
  return emptyBarKg({ ...inv, useCollars }) + 2 * side
}

/** Plate keys from a solution, in loading order (heaviest first). */
export function stackFromSolution(perSide: PlateLoad[]): string[] {
  return perSide.flatMap(p => Array.from({ length: p.count }, () => `${p.weight}${p.unit}`))
}

// Cap on distinct per-side totals tracked; real inventories stay far below it.
const MAX_STATES = 20000

/**
 * Every distinct total (in kg) the inventory can load symmetrically, sorted
 * ascending, starting with the empty bar.
 */
export function loadableTotalsKg(inv: PlateInventory): number[] {
  const key = (v: number) => Math.round(v * 1e4)
  let sides = new Map<number, number>([[0, 0]])
  for (const p of sortedPlates(inv)) {
    const next = new Map(sides)
    for (const base of sides.values()) {
      for (let n = 1; n <= p.pairs; n++) {
        const v = base + n * p.kg
        next.set(key(v), v)
      }
      if (next.size > MAX_STATES) break
    }
    sides = next
    if (sides.size > MAX_STATES) break
  }
  const bar = emptyBarKg(inv)
  return [...sides.values()].map(s => bar + 2 * s).sort((a, b) => a - b)
}

export interface NearestLoadable {
  /** Heaviest loadable weight below the target (in the logging unit), if any. */
  below?: number
  /** Lightest loadable weight above the target, if any. */
  above?: number
}

/**
 * The loadable weights on either side of a target in the logging unit,
 * rounded to 0.1. A target that is itself loadable returns its neighbours.
 */
export function nearestLoadable(target: number, inv: PlateInventory, unit: WeightUnit): NearestLoadable {
  const totals = loadableTotalsKg(inv).map(kg => fromKg(kg, unit))
  const tol = 0.05
  let below: number | undefined
  let above: number | undefined
  for (const t of totals) {
    if (t < target - tol) below = t
    else if (t > target + tol) {
      above = t
      break
    }
  }
  return {
    below: below != null ? round1(below) : undefined,
    above: above != null ? round1(above) : undefined,
  }
}

/**
 * The loadable weight closest to the target (ties go to the lighter one), in
 * the logging unit, rounded to 0.1. Never lighter than the empty bar.
 */
export function roundToLoadable(target: number, inv: PlateInventory, unit: WeightUnit): number {
  const totals = loadableTotalsKg(inv).map(kg => fromKg(kg, unit))
  let best = totals[0]
  for (const t of totals) {
    if (Math.abs(t - target) < Math.abs(best - target) - 1e-9) best = t
  }
  return round1(best)
}

/** Heaviest weight the inventory can load, in the logging unit. */
export function maxLoadable(inv: PlateInventory, unit: WeightUnit): number {
  const totals = loadableTotalsKg(inv)
  return round1(fromKg(totals[totals.length - 1], unit))
}

/** "25 kg", "2.5 lb" — a plate or bar label in its own unit. */
export function formatUnitWeight(w: UnitWeight): string {
  return `${w.weight} ${w.unit === 'kg' ? 'kg' : 'lb'}`
}

/** "25 + 15 + 1 kg" — compact per-side summary for a row hint. */
export function formatPerSide(perSide: PlateLoad[]): string {
  if (perSide.length === 0) return 'Empty bar'
  const units = new Set(perSide.map(p => p.unit))
  const parts = perSide.flatMap(p => Array.from({ length: p.count }, () =>
    units.size === 1 ? String(p.weight) : formatUnitWeight(p)))
  return units.size === 1 ? `${parts.join(' + ')} ${perSide[0].unit === 'kg' ? 'kg' : 'lb'}` : parts.join(' + ')
}

/** Convert the API plate setup shape (all fields optional in the spec). */
export function inventoryFromSetup(setup: {
  bar?: { weight?: number; unit?: string }
  collar?: { weight?: number; unit?: string }
  use_collars?: boolean
  plates?: { weight?: number; unit?: string; pairs?: number }[]
}): PlateInventory {
  return {
    bar: { weight: setup.bar?.weight ?? 0, unit: normalizeUnit(setup.bar?.unit) },
    collar: { weight: setup.collar?.weight ?? 0, unit: normalizeUnit(setup.collar?.unit) },
    useCollars: setup.use_collars ?? false,
    plates: (setup.plates ?? []).map(p => ({
      weight: p.weight ?? 0,
      unit: normalizeUnit(p.unit),
      pairs: p.pairs ?? 0,
    })),
  }
}

/** The default home setup: 20 kg bar, 2.5 kg collars (off), competition kg plates. */
export const KG_COMPETITION: PlateInventory = {
  bar: { weight: 20, unit: 'kg' },
  collar: { weight: 2.5, unit: 'kg' },
  useCollars: false,
  plates: [25, 20, 15, 10, 5, 2.5, 2, 1.5, 1, 0.5, 0.25, 0.125].map(w => ({ weight: w, unit: 'kg' as const, pairs: w === 20 ? 2 : 1 })),
}
