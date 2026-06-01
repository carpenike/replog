import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api, ApiError } from '@/api/client'
import type { PitchSmartStatus } from '@/api/types'
import { Spinner } from '@/components/ui'
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
  const { id } = useParams<{ id: string }>()
  const athleteId = Number(id)

  const { data: athlete } = useQuery({
    queryKey: ['athlete', athleteId],
    queryFn: () => api.getAthlete(athleteId),
    enabled: !isNaN(athleteId),
  })

  const { data: load, isLoading, error } = useQuery({
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

  if (isLoading) return <Spinner />
  if (error) return <p className="text-destructive">Failed to load training load.</p>

  return (
    <div>
      <div className="mb-6">
        <p className="text-sm text-muted-foreground">
          <Link to={`/athletes/${athleteId}`} className="hover:text-foreground">
            {athlete?.name ?? 'Athlete'}
          </Link>
          {' / Load'}
        </p>
        <h1 className="text-2xl font-bold">Training load</h1>
        {load && (
          <p className="text-sm text-muted-foreground mt-1">
            Acute:chronic workload ratio per discipline, as of {load.as_of}. Read-only — for coach review.
          </p>
        )}
      </div>

      {load && load.disciplines.length === 0 ? (
        <p className="text-muted-foreground">No load data yet. Log some sessions to build history.</p>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 mb-8">
          {load?.disciplines.map(d => (
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

      <div className="max-w-xl">
        <h2 className="text-lg font-semibold mb-2">Pitch Smart</h2>
        {pitchLoading ? (
          <Spinner />
        ) : pitchSmart === null || pitchSmart === undefined ? (
          <p className="text-muted-foreground text-sm">No counted pitching sessions yet — no Pitch Smart advisory to show.</p>
        ) : (
          <Alert variant={pitchSmart.over_daily_max || pitchSmart.rest_days_owed > 0 ? 'warning' : 'default'}>
            <AlertTitle>
              {pitchSmart.age_bracket} · daily max {pitchSmart.daily_max} pitches
            </AlertTitle>
            <AlertDescription>
              <p>{pitchSmart.advisory}</p>
              {pitchSmart.rest_days_owed > 0 && pitchSmart.next_eligible_date && (
                <p className="mt-1">
                  Rest days owed: {pitchSmart.rest_days_owed} (next eligible {pitchSmart.next_eligible_date}).
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
