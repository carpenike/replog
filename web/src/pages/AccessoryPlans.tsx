import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'

export function AccessoryPlans() {
  const { id } = useParams<{ id: string }>()
  const athleteId = Number(id)
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

  if (isLoading) return <p className="text-muted-foreground">Loading...</p>

  return (
    <div>
      <p className="text-sm text-muted-foreground mb-1">
        <Link to={`/athletes/${athleteId}`} className="hover:text-foreground">Athlete</Link>
        {' / Accessories'}
      </p>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">Accessory Plans</h1>
        <button onClick={() => setShowForm(!showForm)}
          className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90">
          {showForm ? 'Cancel' : '+ Add'}
        </button>
      </div>

      {showForm && (
        <form onSubmit={(e) => { e.preventDefault(); createMutation.mutate() }}
          className="rounded-lg border border-border bg-card p-4 mb-6 space-y-3">
          <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
            <div>
              <label className="block text-xs text-muted-foreground mb-1">Day</label>
              <select value={day} onChange={e => setDay(e.target.value)}
                className="w-full rounded-md border border-border bg-background px-3 py-1.5 text-sm">
                {[1,2,3,4,5,6,7].map(d => (
                  <option key={d} value={d}>Day {d}</option>
                ))}
              </select>
            </div>
            <div className="col-span-2 md:col-span-3">
              <label className="block text-xs text-muted-foreground mb-1">Exercise</label>
              <select value={exerciseId} onChange={e => setExerciseId(e.target.value)} required
                className="w-full rounded-md border border-border bg-background px-3 py-1.5 text-sm">
                <option value="">Select exercise...</option>
                {exercises?.map(ex => (
                  <option key={ex.id} value={ex.id}>{ex.name}</option>
                ))}
              </select>
            </div>
            <div>
              <label className="block text-xs text-muted-foreground mb-1">Sets</label>
              <input type="number" value={sets} onChange={e => setSets(e.target.value)} min={1}
                className="w-full rounded-md border border-border bg-background px-3 py-1.5 text-sm" />
            </div>
            <div>
              <label className="block text-xs text-muted-foreground mb-1">Min Reps</label>
              <input type="number" value={repMin} onChange={e => setRepMin(e.target.value)} min={1}
                className="w-full rounded-md border border-border bg-background px-3 py-1.5 text-sm" />
            </div>
            <div>
              <label className="block text-xs text-muted-foreground mb-1">Max Reps</label>
              <input type="number" value={repMax} onChange={e => setRepMax(e.target.value)} min={1}
                className="w-full rounded-md border border-border bg-background px-3 py-1.5 text-sm" />
            </div>
            <div>
              <label className="block text-xs text-muted-foreground mb-1">Weight</label>
              <input type="number" step="0.5" value={weight} onChange={e => setWeight(e.target.value)}
                className="w-full rounded-md border border-border bg-background px-3 py-1.5 text-sm" />
            </div>
          </div>
          <div>
            <label className="block text-xs text-muted-foreground mb-1">Notes</label>
            <input type="text" value={notes} onChange={e => setNotes(e.target.value)} placeholder="Optional"
              className="w-full rounded-md border border-border bg-background px-3 py-1.5 text-sm" />
          </div>
          <button type="submit" disabled={createMutation.isPending || !exerciseId}
            className="rounded-md bg-primary px-4 py-1.5 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50">
            Add Accessory
          </button>
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
                  <div key={plan.id} className="flex items-center justify-between rounded-lg border border-border bg-card p-3">
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
                    <button onClick={() => { if (confirm('Delete this accessory?')) deleteMutation.mutate(plan.id) }}
                      className="text-xs text-destructive hover:text-destructive/80">×</button>
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
