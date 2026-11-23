import { describe, expect, it } from 'vitest'
import { KG_COMPETITION, type PlateInventory } from './plates'
import { generateWarmupRamp } from './warmups'

const LB_STANDARD: PlateInventory = {
  bar: { weight: 45, unit: 'lbs' },
  collar: { weight: 0, unit: 'lbs' },
  useCollars: false,
  plates: [45, 35, 25, 10, 5, 2.5].map(w => ({ weight: w, unit: 'lbs' as const, pairs: w === 45 ? 4 : 1 })),
}

describe('generateWarmupRamp', () => {
  it('ramps to 225 lb on kg plates, shown in lbs', () => {
    expect(generateWarmupRamp(225, KG_COMPETITION, 'lbs')).toEqual([
      { label: 'Bar', weight: 44.1, reps: 10 },
      { label: '40%', weight: 89.8, reps: 5 },
      { label: '60%', weight: 135, reps: 3 },
      { label: '80%', weight: 180.2, reps: 2 },
    ])
  })

  it('rounds each step to a loadable lb weight', () => {
    expect(generateWarmupRamp(135, LB_STANDARD, 'lbs').map(s => s.weight)).toEqual([45, 55, 80, 110])
  })

  it('works in kg for a kg logger', () => {
    expect(generateWarmupRamp(100, KG_COMPETITION, 'kg').map(s => s.weight)).toEqual([20, 40, 60, 80])
  })

  it('drops steps that round onto the bar or an earlier step', () => {
    expect(generateWarmupRamp(50, LB_STANDARD, 'lbs')).toEqual([{ label: 'Bar', weight: 45, reps: 10 }])
  })

  it('needs no ramp at or under the empty bar', () => {
    expect(generateWarmupRamp(45, LB_STANDARD, 'lbs')).toEqual([])
    expect(generateWarmupRamp(0, LB_STANDARD, 'lbs')).toEqual([])
  })

  it('never reaches the working weight', () => {
    const ramp = generateWarmupRamp(60, LB_STANDARD, 'lbs')
    expect(ramp.every(s => s.weight < 60)).toBe(true)
  })
})
