// Athlete profile sub-navigation: tab ids, URL parsing and which session
// modalities to surface. Pure so it can be unit-tested.

export const ATHLETE_TABS = ['overview', 'training', 'sessions', 'program', 'tools'] as const
export type AthleteTab = (typeof ATHLETE_TABS)[number]

export const ATHLETE_TAB_LABELS: Record<AthleteTab, string> = {
  overview: 'Overview',
  training: 'Training',
  sessions: 'Sessions',
  program: 'Program',
  tools: 'Tools',
}

/** The tab named by ?tab=, falling back to overview for anything unknown. */
export function parseAthleteTab(raw: string | null | undefined): AthleteTab {
  return (ATHLETE_TABS as readonly string[]).includes(raw ?? '') ? (raw as AthleteTab) : 'overview'
}

export type Modality = 'throwing' | 'conditioning' | 'skill' | 'recovery'

export interface ModalityInfo {
  key: Modality
  count: number
  /** Most recent session date, if any. */
  last?: string
}

/**
 * Split session modalities into the ones to feature (they have data, or are
 * always relevant: the recovery check-in) and the rest, which the page
 * offers as smaller "start logging" links. The load dashboard only matters
 * once any featured modality other than recovery has data.
 */
export function splitModalities(list: ModalityInfo[]): { featured: ModalityInfo[]; others: ModalityInfo[]; showLoad: boolean } {
  const featured = list.filter(m => m.count > 0 || m.key === 'recovery')
  const others = list.filter(m => !featured.includes(m))
  const showLoad = list.some(m => m.key !== 'recovery' && m.count > 0)
  return { featured, others, showLoad }
}

/** Latest YYYY-MM-DD date in a list of dated rows. */
export function latestDate(rows: { date: string }[] | undefined): string | undefined {
  let best: string | undefined
  for (const r of rows ?? []) {
    const d = r.date.slice(0, 10)
    if (!best || d > best) best = d
  }
  return best
}
