import { queryOptions } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { ExerciseHistoryDayData } from '@/api/types'

// Safety cap: 25 pages × 20 workout days ≈ 500 sessions per exercise.
const MAX_HISTORY_PAGES = 25

/**
 * Every history day for an exercise, paging the offset-by-day endpoint until
 * exhausted (or capped). Newest first, matching the API.
 */
export async function fetchFullExerciseHistory(
  athleteId: number,
  exerciseId: number,
): Promise<ExerciseHistoryDayData[]> {
  const days: ExerciseHistoryDayData[] = []
  for (let page = 0; page < MAX_HISTORY_PAGES; page++) {
    const res = await api.listExerciseHistory(athleteId, exerciseId, days.length)
    days.push(...res.days)
    if (!res.has_more) break
  }
  return days
}

/**
 * Shared query for full (all-pages) exercise history, used by session PR
 * detection and the history page's Bests card. Memoized per exercise so a
 * session fetches each exercise's history at most once per staleTime window.
 */
export function fullHistoryQuery(athleteId: number, exerciseId: number) {
  return queryOptions({
    queryKey: ['exercise-history-full', athleteId, exerciseId],
    queryFn: () => fetchFullExerciseHistory(athleteId, exerciseId),
    staleTime: 5 * 60_000,
  })
}
