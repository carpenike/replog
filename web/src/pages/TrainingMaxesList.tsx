import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'

function formatWeight(w: number): string {
  return w === Math.floor(w) ? w.toString() : w.toFixed(1)
}

export function TrainingMaxesList() {
  const { id } = useParams<{ id: string }>()
  const athleteId = Number(id)
  const queryClient = useQueryClient()

  const [showForm, setShowForm] = useState(false)
  const [exerciseId, setExerciseId] = useState('')
  const [weight, setWeight] = useState('')
  const [effectiveDate, setEffectiveDate] = useState(new Date().toISOString().slice(0, 10))
  const [notes, setNotes] = useState('')

  const { data: maxes, isLoading, error } = useQuery({
    queryKey: ['training-maxes', athleteId],
    queryFn: () => api.listTrainingMaxes(athleteId),
    enabled: !isNaN(athleteId),
  })

  const { data: exercises } = useQuery({
    queryKey: ['exercises'],
    queryFn: () => api.listExercises(),
    enabled: showForm,
  })

  const createMutation = useMutation({
    mutationFn: () => api.createTrainingMax(athleteId, {
      exercise_id: parseInt(exerciseId),
      weight: parseFloat(weight),
      effective_date: effectiveDate,
      notes: notes || undefined,
    }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['training-maxes', athleteId] })
      setWeight('')
      setNotes('')
      setShowForm(false)
    },
  })

  if (isLoading) return <p className="text-muted-foreground">Loading training maxes...</p>
  if (error) return <p className="text-destructive">Failed to load training maxes.</p>

  return (
    <div>
      <p className="text-sm text-muted-foreground mb-1">
        <Link to={`/athletes/${athleteId}`} className="hover:text-foreground">Athlete</Link>
        {' / Training Maxes'}
      </p>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">Training Maxes</h1>
        <button onClick={() => setShowForm(!showForm)}
          className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90">
          {showForm ? 'Cancel' : '+ Set TM'}
        </button>
      </div>

      {/* Add form */}
      {showForm && (
        <form onSubmit={(e) => { e.preventDefault(); createMutation.mutate() }}
          className="rounded-lg border border-border bg-card p-4 mb-6 space-y-3">
          <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
            <div className="col-span-2">
              <label htmlFor="tm-exercise" className="block text-xs text-muted-foreground mb-1">Exercise</label>
              <select id="tm-exercise" value={exerciseId} onChange={e => setExerciseId(e.target.value)} required
                className="w-full rounded-md border border-border bg-background px-3 py-1.5 text-sm">
                <option value="">Select exercise...</option>
                {exercises?.map(ex => (
                  <option key={ex.id} value={ex.id}>{ex.name}</option>
                ))}
              </select>
            </div>
            <div>
              <label htmlFor="tm-weight" className="block text-xs text-muted-foreground mb-1">Weight</label>
              <input id="tm-weight" type="number" step="0.5" value={weight} onChange={e => setWeight(e.target.value)}
                required min={1}
                className="w-full rounded-md border border-border bg-background px-3 py-1.5 text-sm" />
            </div>
            <div>
              <label htmlFor="tm-date" className="block text-xs text-muted-foreground mb-1">Effective Date</label>
              <input id="tm-date" type="date" value={effectiveDate} onChange={e => setEffectiveDate(e.target.value)}
                className="w-full rounded-md border border-border bg-background px-3 py-1.5 text-sm" />
            </div>
          </div>
          <div>
            <label htmlFor="tm-notes" className="block text-xs text-muted-foreground mb-1">Notes</label>
            <input id="tm-notes" type="text" value={notes} onChange={e => setNotes(e.target.value)}
              placeholder="Optional"
              className="w-full rounded-md border border-border bg-background px-3 py-1.5 text-sm" />
          </div>
          <button type="submit" disabled={createMutation.isPending || !exerciseId || !weight}
            className="rounded-md bg-primary px-4 py-1.5 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50">
            {createMutation.isPending ? 'Saving...' : 'Set Training Max'}
          </button>
        </form>
      )}

      {maxes && maxes.length === 0 ? (
        <p className="text-muted-foreground">No training maxes recorded.</p>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {maxes?.map(tm => (
            <div key={tm.id} className="rounded-lg border border-border bg-card p-4">
              <p className="text-sm text-muted-foreground">{tm.exercise_name}</p>
              <p className="text-2xl font-bold mt-1">{formatWeight(tm.weight)}</p>
              <p className="text-xs text-muted-foreground mt-1">
                Effective: {tm.effective_date}
              </p>
              {tm.notes && (
                <p className="text-xs text-muted-foreground mt-1">{tm.notes}</p>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
