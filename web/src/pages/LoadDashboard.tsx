import { useParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api, ApiError } from '@/api/client'
import type { PitchSmartStatus } from '@/api/types'
import { EmptyState } from '@/components/ui'
import { PageHeader } from '@/components/PageHeader'
import { QueryState } from '@/components/QueryState'
import { useFriendlyDate } from '@/lib/dates'
import { Skeleton } from '@/components/ui/skeleton'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Alert, AlertTitle, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'

const DISCIPLINE_LABELS: Record<string, string> = {
  resistance: 'Resistance',
  throwing: 'Throwing',
  conditioning: 'Conditioning',
  skill: 'Skill',
}

export function LoadDashboard() {
  const friendlyDate = useFriendlyDate()
  const { id } = useParams<{ id: string }>()
  const athleteId = Number(id)

  const { data: athlete } = useQuery({
    queryKey: ['athlete', athleteId],
    queryFn: () => api.getAthlete(athleteId),
    enabled: !isNaN(athleteId),
  })

  const loadQuery = useQuery({
    queryKey: ['load-summary', athleteId],
    queryFn: () => api.getLoadSummary(athleteId),
    enabled: !isNaN(athleteId),
  })

  // Pitch Smart returns 404 when the athlete has no counted pitching history.
  // That is an expected "no advisory" state, not an error — fold it to null.
  const { data: pitchSmart, isLoading: pitchLoading } = useQuery<PitchSmartStatus | null>({
    queryKey: ['pitch-smart', athleteId],
    queryFn: async () => {
      try {
        return await api.getPitchSmartStatus(athleteId)
      } catch (err) {
        if (err instanceof ApiError && err.code === 404) return null
        throw err
      }
    },
    enabled: !isNaN(athleteId),
    retry: false,
  })

  const load = loadQuery.data

  return (
    <div>
      <PageHeader
        title="Training load"
        back={{ to: `/athletes/${athleteId}`, label: athlete?.name ?? 'Athlete' }}
        description={load && `Acute:chronic workload ratio per discipline, as of ${friendlyDate(load.as_of)}. Read-only, for coach review.`}
      />

      <QueryState
        query={loadQuery}
        resource="training load"
        skeleton="cards"
        isEmpty={l => l.disciplines.length === 0}
        empty={{ icon: '📈', title: 'No load data yet', description: 'Log some sessions to build history.' }}
      >
        {load => (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 mb-8">
          {load.disciplines.map(d => (
            <Card key={d.discipline}>
              <CardHeader>
                <CardTitle className="text-base">{DISCIPLINE_LABELS[d.discipline] ?? d.discipline}</CardTitle>
              </CardHeader>
              <CardContent className="space-y-2">
                <div className="flex justify-between text-sm">
                  <span className="text-muted-foreground">Acute (7d)</span>
                  <span className="font-medium">{d.acute_7d} {d.unit}</span>
                </div>
                <div className="flex justify-between text-sm">
                  <span className="text-muted-foreground">Chronic (28d avg)</span>
                  <span className="font-medium">{d.chronic_28d} {d.unit}</span>
                </div>
                <div className="flex justify-between items-center text-sm pt-1 border-t">
                  <span className="text-muted-foreground">ACWR</span>
                  {d.acwr === null ? (
                    <Badge variant="outline">Insufficient history</Badge>
                  ) : (
                    <span className="font-semibold">{d.acwr.toFixed(2)}</span>
                  )}
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
        )}
      </QueryState>

      <div className="max-w-xl mt-8">
        <h2 className="text-lg font-semibold mb-2">Pitch Smart</h2>
        {pitchLoading ? (
          <Skeleton className="h-20 w-full rounded-lg" />
        ) : pitchSmart === null || pitchSmart === undefined ? (
          <EmptyState icon="⚾" title="No Pitch Smart advisory" description="No counted pitching sessions yet." />
        ) : (
          <Alert variant={pitchSmart.over_daily_max || pitchSmart.rest_days_owed > 0 ? 'warning' : 'default'}>
            <AlertTitle>
              {pitchSmart.age_bracket} · daily max {pitchSmart.daily_max} pitches
            </AlertTitle>
            <AlertDescription>
              <p>{pitchSmart.advisory}</p>
              {pitchSmart.rest_days_owed > 0 && pitchSmart.next_eligible_date && (
                <p className="mt-1">
                  Rest days owed: {pitchSmart.rest_days_owed} (next eligible {friendlyDate(pitchSmart.next_eligible_date)}).
                </p>
              )}
            </AlertDescription>
          </Alert>
        )}
        <p className="text-xs text-muted-foreground mt-2">
          Advisory only, surfaced for coach review — based on MLB Pitch Smart guidelines. The coach makes the call.
        </p>
      </div>
    </div>
  )
}
