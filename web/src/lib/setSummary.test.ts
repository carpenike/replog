import { describe, expect, it } from 'vitest'
import { distinctNotes, formatRest, sharedValue, summarizeSets, type PrescribedSetLike } from '@/lib/setSummary'

function s(extra: Partial<PrescribedSetLike> = {}): PrescribedSetLike {
  return { reps: 5, reps_max: 8, percentage: null, absolute_weight: null, rest_seconds: 120, rir_min: 1, rir_max: 2, notes: null, ...extra }
}

describe('summarizeSets', () => {
  it('collapses identical sets into one line', () => {
    expect(summarizeSets([s(), s(), s()]).map(l => l.text)).toEqual(['3 × 5–8 · RIR 1–2 · 2 min rest'])
  })

  it('keeps differing sets as separate lines in order', () => {
    const lines = summarizeSets([
      s({ non_working: true, reps: 5, reps_max: null, percentage: 50, rir_min: null, rir_max: null }),
      s({ reps: 3, reps_max: null, percentage: 85 }),
      s({ reps: 5, reps_max: null, percentage: 75 }),
      s({ reps: 5, reps_max: null, percentage: 75 }),
    ])
    expect(lines.map(l => [l.warmup, l.text])).toEqual([
      [true, '1 × 5 @ 50% · 2 min rest'],
      [false, '1 × 3 @ 85% · RIR 1–2 · 2 min rest'],
      [false, '2 × 5 @ 75% · RIR 1–2 · 2 min rest'],
    ])
  })

  it('ignores notes when collapsing and can omit rest', () => {
    const lines = summarizeSets([s({ notes: 'a' }), s({ notes: 'b', rest_seconds: 90 })], { omitRest: true })
    expect(lines).toHaveLength(1)
    expect(lines[0].head).toBe('2 × 5–8')
    expect(lines[0].detail).toEqual(['RIR 1–2'])
  })

  it('shows AMRAP, absolute load, single RIR and RPE', () => {
    expect(summarizeSets([s({ reps: null, reps_max: null, absolute_weight: 135, rir_min: 2, rir_max: 2, prescribed_rpe: 8, rest_seconds: null })])[0].text)
      .toBe('1 × AMRAP @ 135 · RIR 2 · RPE 8')
  })
})

describe('helpers', () => {
  it('formatRest uses minutes only for whole minutes over one', () => {
    expect(formatRest(90)).toBe('90s rest')
    expect(formatRest(60)).toBe('60s rest')
    expect(formatRest(180)).toBe('3 min rest')
    expect(formatRest(null)).toBeNull()
  })

  it('distinctNotes drops blanks and repeats', () => {
    expect(distinctNotes([{ notes: ' cue ' }, { notes: 'cue' }, { notes: '' }, { notes: null }, { notes: 'other' }])).toEqual(['cue', 'other'])
  })

  it('sharedValue is undefined when values differ or the list is empty', () => {
    expect(sharedValue([1, 1], x => x)).toBe(1)
    expect(sharedValue([1, 2], x => x)).toBeUndefined()
    expect(sharedValue([], x => x)).toBeUndefined()
  })
})

describe('rep types and custom load', () => {
  it('formats AMRAP floors, seconds and per-side reps', () => {
    const base = { rir_min: null, rir_max: null, rest_seconds: null, reps_max: null }
    expect(summarizeSets([s({ ...base, reps: 5, rep_type: 'amrap' })])[0].head).toBe('1 × 5+')
    expect(summarizeSets([s({ ...base, reps: 30, rep_type: 'seconds' })])[0].head).toBe('1 × 30s')
    expect(summarizeSets([s({ ...base, reps: 8, rep_type: 'each_side' }), s({ ...base, reps: 8, rep_type: 'each_side' })])[0].head).toBe('2 × 8/side')
  })

  it('collapses on the formatted load from a custom load function', () => {
    const lines = summarizeSets(
      [s({ percentage: 75 }), s({ percentage: 75 }), s({ percentage: 85 })],
      { load: x => (x.percentage ? `${x.percentage * 3} lb` : null) },
    )
    expect(lines.map(l => l.head)).toEqual(['2 × 5–8 @ 225 lb', '1 × 5–8 @ 255 lb'])
  })
})
