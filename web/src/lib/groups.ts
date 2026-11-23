// Superset / circuit grouping helpers shared by the program editor and the
// accessory plans page. Pure so the grouping rules are unit-tested.

/** Group keys offered in editors. The server accepts 1–4 letters/digits. */
export const GROUP_KEY_OPTIONS = ['A', 'B', 'C', 'D', 'E', 'F', 'G', 'H']

export interface GroupBlock<T> {
  /** null for an exercise that is not in a group. */
  key: string | null
  items: T[]
}

/** Upper-cased, trimmed key, or null for blank. */
export function normalizeGroupKey(key: string | null | undefined): string | null {
  const k = (key ?? '').trim().toUpperCase()
  return k === '' ? null : k
}

/**
 * Collect items into blocks in first-appearance order. Items sharing a group
 * key land in one block (even if not adjacent); each ungrouped item is its
 * own block. A "group" of one is shown as a plain item.
 */
export function groupBlocks<T>(items: T[], keyOf: (item: T) => string | null | undefined): GroupBlock<T>[] {
  const blocks: GroupBlock<T>[] = []
  const byKey = new Map<string, GroupBlock<T>>()
  for (const item of items) {
    const key = normalizeGroupKey(keyOf(item))
    if (key == null) {
      blocks.push({ key: null, items: [item] })
      continue
    }
    let block = byKey.get(key)
    if (!block) {
      block = { key, items: [] }
      byKey.set(key, block)
      blocks.push(block)
    }
    block.items.push(item)
  }
  return blocks
}

/** "Superset" for two exercises, "Circuit" for three or more. */
export function groupKind(size: number): 'Superset' | 'Circuit' {
  return size >= 3 ? 'Circuit' : 'Superset'
}

/** Position label inside a group: A1, A2, … */
export function memberLabel(key: string, index: number): string {
  return `${key}${index + 1}`
}

/** First option letter not already used, for "New group". */
export function nextFreeGroupKey(used: Iterable<string | null | undefined>): string {
  const taken = new Set<string>()
  for (const k of used) {
    const n = normalizeGroupKey(k)
    if (n) taken.add(n)
  }
  return GROUP_KEY_OPTIONS.find(k => !taken.has(k)) ?? `G${taken.size + 1}`
}

/** The group key most of these rows carry (ties go to the first seen), or null. */
export function dominantGroupKey(rows: { group_key?: string | null }[]): string | null {
  const counts = new Map<string, number>()
  let best: string | null = null
  for (const r of rows) {
    const k = normalizeGroupKey(r.group_key)
    if (!k) continue
    const n = (counts.get(k) ?? 0) + 1
    counts.set(k, n)
    if (best == null || n > (counts.get(best) ?? 0)) best = k
  }
  return best
}
