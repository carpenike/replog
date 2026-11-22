import { describe, expect, it } from 'vitest'
import { classifyPR, computeBests } from './records'

describe('records ignore warm-ups', () => {
  it('a warm-up set is never a PR', () => {
    expect(classifyPR([{ reps: 5, weight: 100 }], { reps: 5, weight: 200, set_type: 'warmup' })).toBeNull()
  })

  it('prior warm-ups do not form the baseline', () => {
    // Only warm-ups before: no working history, so not a record.
    expect(classifyPR([{ reps: 5, weight: 300, set_type: 'warmup' }], { reps: 5, weight: 200 })).toBeNull()
    // A heavy warm-up does not mask a working-set weight PR.
    expect(classifyPR(
      [{ reps: 5, weight: 150 }, { reps: 1, weight: 250, set_type: 'warmup' }],
      { reps: 3, weight: 200 },
    )).toMatchObject({ kind: 'weight', value: 200, prev: 150 })
  })

  it('computeBests skips warm-ups', () => {
    const bests = computeBests([
      { date: '2026-05-01', reps: 5, weight: 100 },
      { date: '2026-05-02', reps: 20, weight: 45, set_type: 'warmup' },
      { date: '2026-05-02', reps: 1, weight: 300, set_type: 'warmup' },
    ])
    expect(bests.heaviest?.weight).toBe(100)
    expect(bests.mostReps?.reps).toBe(5)
  })
})
