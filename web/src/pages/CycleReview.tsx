import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { api } from '@/api/client'
import { PageHeader } from '@/components/PageHeader'
import { QueryState } from '@/components/QueryState'
import { EmptyState } from '@/components/ui'
import { useFriendlyDate } from '@/lib/dates'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import { Card, CardContent } from '@/components/ui/card'

export function CycleReview() {
  const { id } = useParams<{ id: string }>()
  const athleteId = Number(id)
  const queryClient = useQueryClient()
  const friendlyDate = useFriendlyDate()
  const { data: athlete } = useQuery({
    queryKey: ['athlete', athleteId],
    queryFn: () => api.getAthlete(athleteId),
    enabled: !isNaN(athleteId),
  })
  const query = useQuery({
    queryKey: ['cycle-review', athleteId],
    queryFn: () => api.getCycleReview(athleteId),
    enabled: !isNaN(athleteId),
    retry: false,
  })
  const review = query.data
  const [selections, setSelections] = useState<Record<number, boolean>>({})
  const applyMutation = useMutation({
    mutationFn: () => {
      const bumps = review?.suggestions
        .filter(s => selections[s.exercise_id] !== false)
        .map(s => ({ exercise_id: s.exercise_id, new_weight: s.suggested_tm })) ?? []
      return api.applyTMBumps(athleteId, bumps)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['training-maxes', athleteId] })
      toast.success('Training maxes updated')
    },
  })

  return (
    <div>
      <PageHeader
        title="Cycle Review"
        back={{ to: `/athletes/${athleteId}`, label: athlete?.name ?? 'Athlete' }}
        description={review && `Cycle ${review.cycle_number} · ${friendlyDate(review.cycle_start, { absolute: true })} → ${friendlyDate(review.cycle_end, { absolute: true })}`}
      />
      {/* A 404 is the expected "no active program/cycle" state; anything else is retryable. */}
      <QueryState
        query={query}
        resource="cycle review"
        notFound={<EmptyState icon="📊" title="No cycle to review" description="This athlete has no active program or cycle data yet." />}
        isEmpty={r => r.suggestions.length === 0}
        empty={{ icon: '✅', title: 'No training-max changes suggested', description: 'Nothing to review for this cycle.' }}
      >
        {review => (
          <>
            <p className="text-sm text-muted-foreground mb-4">
              Suggested from this cycle&apos;s logged sets. You decide which to apply.
            </p>
            <div className="space-y-3 mb-6">
              {review.suggestions.map(s => (
                <Card key={s.exercise_id}>
                  <CardContent>
                    <div className="flex items-center justify-between gap-3">
                      <div>
                        <p className="font-medium">{s.exercise_name}</p>
                        <p className="text-sm text-muted-foreground">
                          {s.current_tm} → <span className="text-primary font-medium">{s.suggested_tm}</span>
                          <span className="ml-2 text-xs">(+{s.increment})</span>
                        </p>
                      </div>
                      <Label className="min-h-11 md:min-h-0">
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
            <Button onClick={() => applyMutation.mutate()} disabled={applyMutation.isPending}>
              {applyMutation.isPending ? 'Applying...' : 'Apply Selected Changes'}
            </Button>
          </>
        )}
      </QueryState>
    </div>
  )
}
