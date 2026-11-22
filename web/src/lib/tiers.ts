/** Kid progression tiers, in order. Mirrors models.NextTier on the server. */
export const TIER_ORDER = ['foundational', 'intermediate', 'sport_performance'] as const

export function tierLabel(tier: string): string {
  switch (tier) {
    case 'foundational': return 'Foundational'
    case 'intermediate': return 'Intermediate'
    case 'sport_performance': return 'Sport Performance'
    default: return tier
  }
}

/** The tier a promotion moves to, or null at the top tier / for unknown tiers. */
export function nextTier(tier: string | null | undefined): string | null {
  if (!tier) return null
  const i = (TIER_ORDER as readonly string[]).indexOf(tier)
  if (i < 0 || i >= TIER_ORDER.length - 1) return null
  return TIER_ORDER[i + 1]
}

/** Confirm-dialog copy for a coach promoting an athlete. */
export function promoteCopy(name: string, tier: string): { title: string; description: string; confirmLabel: string } | null {
  const next = nextTier(tier)
  if (!next) return null
  return {
    title: `Promote to ${tierLabel(next)}?`,
    description: `${name} moves from ${tierLabel(tier)} to ${tierLabel(next)}. You can change the tier later from Edit.`,
    confirmLabel: `Promote to ${tierLabel(next)}`,
  }
}
