import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'
import { Spinner } from '@/components/ui'
import { useConfirm } from '@/lib/useConfirm'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Card, CardContent } from '@/components/ui/card'
export function AccessoryPlans() {
  const { id } = useParams<{ id: string }>()
  const athleteId = Number(id)
  const { confirm, dialog: confirmDialog } = useConfirm()
  const queryClient = useQueryClient()
  const [showForm, setShowForm] = useState(false)
  const [day, setDay] = useState('1')
  const [exerciseId, setExerciseId] = useState('')
  const [sets, setSets] = useState('')
  const [repMin, setRepMin] = useState('')
  const [repMax, setRepMax] = useState('')
  const [weight, setWeight] = useState('')
  const [notes, setNotes] = useState('')
  const { data: plans, isLoading } = useQuery({
    queryKey: ['accessories', athleteId],
    queryFn: () => api.listAccessoryPlans(athleteId),
    enabled: !isNaN(athleteId),
  })
  const { data: exercises } = useQuery({
    queryKey: ['exercises'],
    queryFn: () => api.listExercises(),
    enabled: showForm,
  })
  const createMutation = useMutation({
    mutationFn: () => api.createAccessoryPlan(athleteId, {
      day: parseInt(day),
      exercise_id: parseInt(exerciseId),
      target_sets: sets ? parseInt(sets) : undefined,
      target_rep_min: repMin ? parseInt(repMin) : undefined,
      target_rep_max: repMax ? parseInt(repMax) : undefined,
      target_weight: weight ? parseFloat(weight) : undefined,
      notes: notes || undefined,
    }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['accessories', athleteId] })
      setExerciseId('')
      setSets('')
      setRepMin('')
      setRepMax('')
      setWeight('')
      setNotes('')
      setShowForm(false)
    },
  })
  const deleteMutation = useMutation({
    mutationFn: (planId: number) => api.deleteAccessoryPlan(athleteId, planId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['accessories', athleteId] }),
  })
  // Group plans by day
  const byDay = new Map<number, typeof plans>()
  for (const p of plans ?? []) {
    if (!byDay.has(p.day)) byDay.set(p.day, [])
    byDay.get(p.day)!.push(p)
  }
  if (isLoading) return <Spinner />
  return (
    <div>
      <p className="text-sm text-muted-foreground mb-1">
        <Link to={`/athletes/${athleteId}`} className="hover:text-foreground">Athlete</Link>
        {' / Accessories'}
      </p>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">Accessory Plans</h1>
        <Button variant="ghost" onClick={() => setShowForm(!showForm)}
          >
          {showForm ? 'Cancel' : '+ Add'}
        </Button>
      </div>
      {showForm && (
        <form onSubmit={(e) => { e.preventDefault(); createMutation.mutate() }}
          className="rounded-lg border border-border bg-card p-4 mb-6 space-y-3">
          <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
            <div>
              <Label >Day</Label>
              <select value={day} onChange={e => setDay(e.target.value)}
                className="w-full rounded-md border border-border bg-background px-3 py-1.5 text-sm">
                {[1,2,3,4,5,6,7].map(d => (
                  <option key={d} value={d}>Day {d}</option>
                ))}
              </select>
            </div>
            <div className="col-span-2 md:col-span-3">
              <Label >Exercise</Label>
              <select value={exerciseId} onChange={e => setExerciseId(e.target.value)} required
                className="w-full rounded-md border border-border bg-background px-3 py-1.5 text-sm">
                <option value="">Select exercise...</option>
                {exercises?.map(ex => (
                  <option key={ex.id} value={ex.id}>{ex.name}</option>
                ))}
              </select>
            </div>
            <div>
              <Label >Sets</Label>
              <Input type="number" value={sets} onChange={e => setSets(e.target.value)} min={1} />
            </div>
            <div>
              <Label >Min Reps</Label>
              <Input type="number" value={repMin} onChange={e => setRepMin(e.target.value)} min={1} />
            </div>
            <div>
              <Label >Max Reps</Label>
              <Input type="number" value={repMax} onChange={e => setRepMax(e.target.value)} min={1} />
            </div>
            <div>
              <Label >Weight</Label>
              <Input type="number" step="0.5" value={weight} onChange={e => setWeight(e.target.value)} />
            </div>
          </div>
          <div>
            <Label >Notes</Label>
            <Input type="text" value={notes} onChange={e => setNotes(e.target.value)} placeholder="Optional" />
          </div>
          <Button type="submit" disabled={createMutation.isPending || !exerciseId}
            >
            Add Accessory
          </Button>
        </form>
      )}
      {plans && plans.length === 0 ? (
        <p className="text-muted-foreground">No accessory plans configured.</p>
      ) : (
        <div className="space-y-6">
          {Array.from(byDay.entries()).sort((a, b) => a[0] - b[0]).map(([dayNum, dayPlans]) => (
            <div key={dayNum}>
              <h2 className="text-sm font-semibold text-muted-foreground mb-2">Day {dayNum}</h2>
              <div className="space-y-2">
                {dayPlans?.map(plan => (
                  <Card size="sm" className="flex items-center justify-between">
                    <CardContent>
                    <div>
                      <p className="text-sm font-medium">{plan.exercise_name}</p>
                      <p className="text-xs text-muted-foreground">
                        {plan.target_sets && `${plan.target_sets}×`}
                        {plan.target_rep_min && plan.target_rep_max
                          ? `${plan.target_rep_min}-${plan.target_rep_max}`
                          : plan.target_rep_min ?? plan.target_rep_max ?? ''}
                        {plan.target_weight ? ` @ ${plan.target_weight}` : ''}
                        {plan.notes ? ` — ${plan.notes}` : ''}
                      </p>
                    </div>
                    <Button variant="ghost" onClick={async () => { if (await confirm({ title: 'Delete Accessory', description: 'Remove this accessory plan?', confirmLabel: 'Delete', variant: 'danger' })) deleteMutation.mutate(plan.id) }}
                      >×</Button>
                    </CardContent>
                  </Card>
                ))}
              </div>
            </div>
          ))}
        </div>
      )}
      {confirmDialog()}
    </div>
  )
}