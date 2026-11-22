import { describe, expect, it } from 'vitest'
import { dominantGroupKey, groupBlocks, groupKind, memberLabel, nextFreeGroupKey, normalizeGroupKey } from '@/lib/groups'

describe('groupBlocks', () => {
  const rows = [
    { name: 'Squat', g: null },
    { name: 'Lateral Raise', g: 'b' },
    { name: 'Calf Raise', g: 'C' },
    { name: 'Hammer Curl', g: 'B' },
    { name: 'Plank', g: '' },
  ]

  it('collects shared keys in first-appearance order and keeps ungrouped items alone', () => {
    const blocks = groupBlocks(rows, r => r.g)
    expect(blocks.map(b => [b.key, b.items.map(i => i.name)])).toEqual([
      [null, ['Squat']],
      ['B', ['Lateral Raise', 'Hammer Curl']],
      ['C', ['Calf Raise']],
      [null, ['Plank']],
    ])
  })

  it('labels members and names the group by size', () => {
    expect(memberLabel('A', 0)).toBe('A1')
    expect(memberLabel('A', 1)).toBe('A2')
    expect(groupKind(2)).toBe('Superset')
    expect(groupKind(3)).toBe('Circuit')
  })
})

describe('group keys', () => {
  it('normalizes', () => {
    expect(normalizeGroupKey(' a ')).toBe('A')
    expect(normalizeGroupKey('')).toBeNull()
    expect(normalizeGroupKey(undefined)).toBeNull()
  })

  it('finds the next free letter', () => {
    expect(nextFreeGroupKey([])).toBe('A')
    expect(nextFreeGroupKey(['A', 'b', null])).toBe('C')
  })

  it('picks the most common key', () => {
    expect(dominantGroupKey([{ group_key: 'A' }, { group_key: 'B' }, { group_key: 'B' }])).toBe('B')
    expect(dominantGroupKey([{ group_key: null }])).toBeNull()
  })
})
