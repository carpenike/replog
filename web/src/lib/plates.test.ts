import { describe, expect, it } from 'vitest'
import {
  KG_COMPETITION,
  formatPerSide,
  inventoryFromSetup,
  maxLoadable,
  nearestLoadable,
  roundToLoadable,
  solvePlates,
  type PlateInventory,
} from './plates'

const LB_STANDARD: PlateInventory = {
  bar: { weight: 45, unit: 'lbs' },
  collar: { weight: 0, unit: 'lbs' },
  useCollars: false,
  plates: [
    { weight: 45, unit: 'lbs', pairs: 4 },
    { weight: 35, unit: 'lbs', pairs: 1 },
    { weight: 25, unit: 'lbs', pairs: 1 },
    { weight: 10, unit: 'lbs', pairs: 2 },
    { weight: 5, unit: 'lbs', pairs: 1 },
    { weight: 2.5, unit: 'lbs', pairs: 1 },
  ],
}

describe('solvePlates', () => {
  it('loads 225 lb on a 20 kg bar with kg plates and reports the real weight', () => {
    const s = solvePlates(225, KG_COMPETITION, 'lbs')
    expect(s.perSide).toEqual([
      { weight: 25, unit: 'kg', count: 1 },
      { weight: 15, unit: 'kg', count: 1 },
      { weight: 1, unit: 'kg', count: 1 },
    ])
    expect(s.loadedKg).toBeCloseTo(102, 6)
    expect(s.loaded).toBe(224.9)
    expect(s.difference).toBe(-0.1)
    expect(s.exact).toBe(false)
    expect(s.belowBar).toBe(false)
    expect(s.overCapacity).toBe(false)
    expect(formatPerSide(s.perSide)).toBe('25 + 15 + 1 kg')
  })

  it('offers the nearest loadable weights either side of 225 lb', () => {
    expect(nearestLoadable(225, KG_COMPETITION, 'lbs')).toEqual({ below: 224.9, above: 227.1 })
  })

  it('counts collars in the bar weight when they are used', () => {
    const s = solvePlates(225, { ...KG_COMPETITION, useCollars: true }, 'lbs')
    expect(s.perSide.map(p => p.weight)).toEqual([25, 10, 2.5, 1])
    expect(s.loaded).toBe(224.9)
  })

  it('solves an exact kg target logged in kg and gives its neighbours', () => {
    const s = solvePlates(100, KG_COMPETITION, 'kg')
    expect(s.perSide.map(p => p.weight)).toEqual([25, 15])
    expect(s.loaded).toBe(100)
    expect(s.difference).toBe(0)
    expect(s.exact).toBe(true)
    expect(nearestLoadable(100, KG_COMPETITION, 'kg')).toEqual({ below: 99, above: 101 })
  })

  it('uses several of one plate up to the pairs owned', () => {
    const s = solvePlates(315, LB_STANDARD, 'lbs')
    expect(s.perSide).toEqual([{ weight: 45, unit: 'lbs', count: 3 }])
    expect(s.exact).toBe(true)
    const t = solvePlates(95, LB_STANDARD, 'lbs')
    expect(t.perSide).toEqual([{ weight: 25, unit: 'lbs', count: 1 }])
  })

  it('stops at the empty bar for a target under the bar', () => {
    const s = solvePlates(30, KG_COMPETITION, 'lbs')
    expect(s.belowBar).toBe(true)
    expect(s.perSide).toEqual([])
    expect(s.loaded).toBe(44.1)
    expect(formatPerSide(s.perSide)).toBe('Empty bar')
  })

  it('flags a target heavier than the inventory can load', () => {
    const s = solvePlates(800, KG_COMPETITION, 'lbs')
    expect(s.overCapacity).toBe(true)
    expect(s.loadedKg).toBeCloseTo(335, 6)
    expect(maxLoadable(KG_COMPETITION, 'lbs')).toBe(738.5)
    expect(nearestLoadable(800, KG_COMPETITION, 'lbs').above).toBeUndefined()
  })

  it('handles lb plates on a kg bar', () => {
    const mixed: PlateInventory = { ...LB_STANDARD, bar: { weight: 20, unit: 'kg' } }
    const s = solvePlates(135, mixed, 'lbs')
    expect(s.perSide).toEqual([{ weight: 45, unit: 'lbs', count: 1 }])
    expect(s.loaded).toBe(134.1)
  })
})

describe('roundToLoadable', () => {
  it('rounds to the closest loadable weight in the logging unit', () => {
    expect(roundToLoadable(90, KG_COMPETITION, 'lbs')).toBe(90.4)
    expect(roundToLoadable(108, LB_STANDARD, 'lbs')).toBe(110)
    expect(roundToLoadable(10, LB_STANDARD, 'lbs')).toBe(45)
  })
})

describe('inventoryFromSetup', () => {
  it('maps the API setup and normalizes units', () => {
    const inv = inventoryFromSetup({
      bar: { weight: 20, unit: 'kg' },
      collar: { weight: 2.5, unit: 'kg' },
      use_collars: true,
      plates: [{ weight: 25, unit: 'kg', pairs: 2 }, { weight: 10, unit: 'lb', pairs: 1 }],
    })
    expect(inv.useCollars).toBe(true)
    expect(inv.plates[1].unit).toBe('lbs')
    expect(solvePlates(275.6, inv, 'lbs').perSide[0]).toEqual({ weight: 25, unit: 'kg', count: 2 })
  })
})
