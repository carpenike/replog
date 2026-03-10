import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'
import { Spinner } from '@/components/ui'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import { Card, CardContent } from '@/components/ui/card'
import { Alert } from '@/components/ui/alert'
export function CycleReview() {
  const { id } = useParams<{ id: string }>()
  const athleteId = Number(id)
  const queryClient = useQueryClient()
  const { data: athlete } = useQuery({
    queryKey: ['athlete', athleteId],
    queryFn: () => api.getAthlete(athleteId),
    enabled: !isNaN(athleteId),
  })
  const { data: review, isLoading, error } = useQuery({
    queryKey: ['cycle-review', athleteId],
    queryFn: () => api.getCycleReview(athleteId),
    enabled: !isNaN(athleteId),
  })
  const [selections, setSelections] = useState<Record<number, boolean>>({})
  const [applied, setApplied] = useState(false)
  const applyMutation = useMutation({
    mutationFn: () => {
      const bumps = review?.suggestions
        .filter(s => selections[s.exercise_id] !== false)
        .map(s => ({ exercise_id: s.exercise_id, new_weight: s.suggested_tm })) ?? []
      return api.applyTMBumps(athleteId, bumps)
    },
    onSuccess: (result) => {
      queryClient.invalidateQueries({ queryKey: ['training-maxes', athleteId] })
      setApplied(true)
      setTimeout(() => setApplied(false), 3000)
      void result
    },
  })
  if (isLoading) return <Spinner />
  if (error) return (
    <div>
      <p className="text-sm text-muted-foreground mb-1">
        <Link to={`/athletes/${athleteId}`} className="hover:text-foreground">{athlete?.name ?? 'Athlete'}</Link>
        {' / Cycle Review'}
      </p>
      <h1 className="text-2xl font-bold mb-4">Cycle Review</h1>
      <Card className="text-center">
        <CardContent>
        <p className="text-muted-foreground">No active program or cycle data available.</p>
        </CardContent>
      </Card>
    </div>
  )
  return (
    <div>
      <p className="text-sm text-muted-foreground mb-1">
        <Link to={`/athletes/${athleteId}`} className="hover:text-foreground">{athlete?.name ?? 'Athlete'}</Link>
        {' / Cycle Review'}
      </p>
      <h1 className="text-2xl font-bold mb-2">Cycle Review</h1>
      <p className="text-sm text-muted-foreground mb-6">
        Cycle {review?.cycle_number} • {review?.cycle_start} → {review?.cycle_end}
      </p>
      {applied && (
        <Alert variant="success" className="mb-4">
          Training maxes updated!
        </Alert>
      )}
      {review?.suggestions.length === 0 ? (
        <p className="text-muted-foreground">No TM bump suggestions for this cycle.</p>
      ) : (
        <div className="space-y-3 mb-6">
          {review?.suggestions.map(s => (
            <Card>
              <CardContent>
              <div className="flex items-center justify-between">
                <div>
                  <p className="font-medium">{s.exercise_name}</p>
                  <p className="text-sm text-muted-foreground">
                    {s.current_tm} → <span className="text-primary font-medium">{s.suggested_tm}</span>
                    <span className="ml-2 text-xs">(+{s.increment})</span>
                  </p>
                </div>
                <Label>
                  <Checkbox
                    checked={selections[s.exercise_id] !== false}
                    onCheckedChange={(checked) => setSelections(prev => ({ ...prev, [s.exercise_id]: checked }))}
                  />
                  <span className="text-sm">Apply</span>
                </Label>
              </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}
      {review && review.suggestions.length > 0 && (
        <Button variant="ghost" onClick={() => applyMutation.mutate()}
          disabled={applyMutation.isPending}
          >
          {applyMutation.isPending ? 'Applying...' : 'Apply Selected Bumps'}
        </Button>
      )}
    </div>
  )
}