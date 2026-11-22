import { describe, expect, it } from 'vitest'
import type { PrescriptionSetData, WhiteboardLine } from '@/api/types'
import { boardBlocks, lineStatus, loggedChip, parseAthleteIds, prescribedSummary, progressPercent, toggleAthlete } from './whiteboard'

function ps(over: Partial<PrescriptionSetData>): PrescriptionSetData {
  return { set_number: 1, rep_type: 'reps', non_working: false, ...over } as PrescriptionSetData
}

function line(id: number, over: Partial<WhiteboardLine> = {}): WhiteboardLine {
  return { exercise_id: id, exercise_name: `Ex ${id}`, prescribed: [], logged: [], working_target: 3, working_done: 0, ...over }
}

describe('athlete selection', () => {
  it('parses, dedupes and caps ids', () => {
    expect(parseAthleteIds(null)).toEqual([])
    expect(parseAthleteIds('3, 1,3,x,-2,0,1.5')).toEqual([3, 1])
    expect(parseAthleteIds('1,2,3,4,5,6,7,8,9,10')).toHaveLength(8)
  })

  it('toggles within the cap', () => {
    expect(toggleAthlete([1, 2], 2)).toEqual([1])
    expect(toggleAthlete([1], 2)).toEqual([1, 2])
    const full = [1, 2, 3, 4, 5, 6, 7, 8]
    expect(toggleAthlete(full, 9)).toBe(full)
  })
})

describe('prescribedSummary', () => {
  it('compresses runs and sets warm-ups aside', () => {
    const sets = [
      ps({ reps: 5, non_working: true }),
      ps({ reps: 5, absolute_weight: 185 }),
      ps({ reps: 5, absolute_weight: 185 }),
      ps({ reps: 5, absolute_weight: 185 }),
      ps({ reps: 3, rep_type: 'amrap', target_weight: 202.5 }),
    ]
    expect(prescribedSummary(sets, 'lb')).toEqual({ runs: ['3 × 5 @ 185 lb', '1 × 3+ @ 202.5 lb'], warmups: 1 })
  })

  it('shows ranges, effort and percentages', () => {
    const sets = [ps({ reps: 8, reps_max: 12, rir_min: 2, rir_max: 3 }), ps({ reps: 8, reps_max: 12, rir_min: 2, rir_max: 3 }), ps({ reps: 5, percentage: 75 })]
    expect(prescribedSummary(sets).runs).toEqual(['2 × 8–12 (RIR 2–3)', '1 × 5 @ 75%'])
  })
})

describe('loggedChip', () => {
  it('formats weighted, bodyweight and warm-up sets', () => {
    expect(loggedChip({ reps: 5, weight: 185, set_type: 'working' })).toEqual({ text: '185×5', warmup: false })
    expect(loggedChip({ reps: 12, weight: null, set_type: 'working' })).toEqual({ text: '×12', warmup: false })
    expect(loggedChip({ reps: 5, weight: 95, set_type: 'warmup' })).toEqual({ text: '95×5', warmup: true })
    expect(loggedChip({ reps: 30, weight: 0, reps_label: '30s' })).toEqual({ text: '×30s', warmup: false })
  })
})

describe('lineStatus and progress', () => {
  it('classifies lines', () => {
    expect(lineStatus({ working_target: 3, working_done: 0 })).toBe('todo')
    expect(lineStatus({ working_target: 3, working_done: 1 })).toBe('active')
    expect(lineStatus({ working_target: 3, working_done: 4 })).toBe('done')
    expect(lineStatus({ working_target: 0, working_done: 2 })).toBe('extra')
  })

  it('clamps progress', () => {
    expect(progressPercent(0, 0)).toBe(0)
    expect(progressPercent(2, 3)).toBe(67)
    expect(progressPercent(5, 3)).toBe(100)
  })
})

describe('boardBlocks', () => {
  it('groups supersets with A1/A2 labels and leaves singletons plain', () => {
    const blocks = boardBlocks([
      line(1, { group_key: 'a' }),
      line(2),
      line(3, { group_key: 'A' }),
      line(4, { group_key: 'B' }),
    ])
    expect(blocks.map(b => b.key)).toEqual(['A', null, null])
    expect(blocks[0].lines.map(l => [l.line.exercise_id, l.label])).toEqual([[1, 'A1'], [3, 'A2']])
    expect(blocks[2].lines[0].label).toBeNull()
  })
})
