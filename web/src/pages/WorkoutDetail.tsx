import { useState } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'

function formatWeight(w: number): string {
  return w === Math.floor(w) ? w.toString() : w.toFixed(1)
}

export function WorkoutDetail() {
  const { id, workoutId } = useParams<{ id: string; workoutId: string }>()
  const athleteId = Number(id)
  const wId = Number(workoutId)
  const queryClient = useQueryClient()
  const navigate = useNavigate()

  const [exerciseId, setExerciseId] = useState('')
  const [reps, setReps] = useState('')
  const [setWeight, setSetWeight] = useState('')
  const [rpe, setRpe] = useState('')
  const [showAddForm, setShowAddForm] = useState(false)
  const [editingSetId, setEditingSetId] = useState<number | null>(null)
  const [editReps, setEditReps] = useState('')
  const [editWeight, setEditWeight] = useState('')
  const [editRpe, setEditRpe] = useState('')
  const [editSetNotes, setEditSetNotes] = useState('')
  const [editingNotes, setEditingNotes] = useState(false)
  const [notesText, setNotesText] = useState('')

  const { data, isLoading, error } = useQuery({
    queryKey: ['workout', athleteId, wId],
    queryFn: () => api.getWorkout(athleteId, wId),
    enabled: !isNaN(athleteId) && !isNaN(wId),
  })

  const { data: exercises } = useQuery({
    queryKey: ['exercises'],
    queryFn: () => api.listExercises(),
    enabled: showAddForm,
  })

  const addSetMutation = useMutation({
    mutationFn: () => api.addSet(athleteId, wId, {
      exercise_id: parseInt(exerciseId),
      reps: parseInt(reps),
      weight: setWeight ? parseFloat(setWeight) : undefined,
      rpe: rpe ? parseFloat(rpe) : undefined,
    }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['workout', athleteId, wId] })
      setReps('')
      setSetWeight('')
      setRpe('')
    },
  })

  const deleteSetMutation = useMutation({
    mutationFn: (setId: number) => api.deleteSet(athleteId, wId, setId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['workout', athleteId, wId] })
    },
  })

  const deleteWorkoutMutation = useMutation({
    mutationFn: () => api.deleteWorkout(athleteId, wId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['workouts', athleteId] })
      navigate(`/athletes/${athleteId}/workouts`)
    },
  })

  const updateSetMutation = useMutation({
    mutationFn: (setId: number) => api.updateSet(athleteId, wId, setId, {
      reps: parseInt(editReps),
      weight: editWeight ? parseFloat(editWeight) : undefined,
      rpe: editRpe ? parseFloat(editRpe) : undefined,
      notes: editSetNotes || undefined,
    }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['workout', athleteId, wId] })
      setEditingSetId(null)
    },
  })

  const updateNotesMutation = useMutation({
    mutationFn: () => api.updateWorkoutNotes(athleteId, wId, notesText),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['workout', athleteId, wId] })
      setEditingNotes(false)
    },
  })

  if (isLoading) return <p className="text-muted-foreground">Loading workout...</p>
  if (error) return <p className="text-destructive">Failed to load workout.</p>
  if (!data) return <p className="text-muted-foreground">Workout not found.</p>

  const { workout, groups } = data

  return (
    <div>
      <p className="text-sm text-muted-foreground mb-1">
        <Link to={`/athletes/${athleteId}`} className="hover:text-foreground">Athlete</Link>
        {' / '}
        <Link to={`/athletes/${athleteId}/workouts`} className="hover:text-foreground">Workouts</Link>
        {' / '}
        {workout.date}
      </p>
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold mb-2">{workout.date}</h1>
        <button
          onClick={() => { if (confirm('Delete this workout and all its sets?')) deleteWorkoutMutation.mutate() }}
          className="text-sm text-destructive hover:text-destructive/80">
          Delete
        </button>
      </div>
      {workout.program_name && (
        <p className="text-sm text-muted-foreground mb-4">{workout.program_name}</p>
      )}

      {/* Editable notes */}
      {editingNotes ? (
        <div className="rounded-lg border border-border bg-card p-3 mb-4">
          <textarea
            value={notesText}
            onChange={e => setNotesText(e.target.value)}
            rows={3}
            className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm mb-2"
            placeholder="Session notes..."
          />
          <div className="flex gap-2">
            <button onClick={() => updateNotesMutation.mutate()}
              disabled={updateNotesMutation.isPending}
              className="rounded-md bg-primary px-3 py-1 text-xs font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50">
              Save
            </button>
            <button onClick={() => setEditingNotes(false)}
              className="rounded-md border border-border px-3 py-1 text-xs hover:bg-accent">
              Cancel
            </button>
          </div>
        </div>
      ) : (
        <div
          onClick={() => { setNotesText(workout.notes ?? ''); setEditingNotes(true) }}
          className="rounded-lg border border-border bg-card p-3 mb-4 text-sm cursor-pointer hover:border-primary/30 transition-colors"
        >
          {workout.notes || <span className="text-muted-foreground italic">Click to add notes...</span>}
        </div>
      )}

      {groups.length === 0 ? (
        <p className="text-muted-foreground">No sets logged.</p>
      ) : (
        <div className="space-y-6">
          {groups.map(group => (
            <div key={group.exercise_id} className="rounded-lg border border-border overflow-hidden">
              <div className="bg-muted/50 px-4 py-2 border-b border-border">
                <h3 className="font-semibold">{group.exercise_name}</h3>
              </div>
              <table className="w-full">
                <thead>
                  <tr className="text-xs text-muted-foreground border-b border-border">
                    <th className="text-left px-4 py-2 w-12">Set</th>
                    <th className="text-left px-4 py-2">Reps</th>
                    <th className="text-left px-4 py-2">Weight</th>
                    <th className="text-left px-4 py-2">RPE</th>
                    <th className="text-left px-4 py-2">Notes</th>
                    <th className="px-4 py-2 w-10"></th>
                  </tr>
                </thead>
                <tbody>
                  {group.sets.map(set => (
                    editingSetId === set.id ? (
                      <tr key={set.id} className="border-b border-border last:border-0 text-sm bg-muted/30">
                        <td className="px-4 py-2 text-muted-foreground">{set.set_number}</td>
                        <td className="px-2 py-1">
                          <input type="number" value={editReps} onChange={e => setEditReps(e.target.value)}
                            min={1} className="w-16 rounded border border-border bg-background px-2 py-1 text-sm" />
                        </td>
                        <td className="px-2 py-1">
                          <input type="number" step="0.5" value={editWeight} onChange={e => setEditWeight(e.target.value)}
                            className="w-20 rounded border border-border bg-background px-2 py-1 text-sm" />
                        </td>
                        <td className="px-2 py-1">
                          <input type="number" step="0.5" min={1} max={10} value={editRpe} onChange={e => setEditRpe(e.target.value)}
                            className="w-16 rounded border border-border bg-background px-2 py-1 text-sm" />
                        </td>
                        <td className="px-2 py-1">
                          <input type="text" value={editSetNotes} onChange={e => setEditSetNotes(e.target.value)}
                            className="w-full rounded border border-border bg-background px-2 py-1 text-sm" />
                        </td>
                        <td className="px-2 py-1">
                          <div className="flex gap-1">
                            <button onClick={() => updateSetMutation.mutate(set.id)}
                              disabled={updateSetMutation.isPending}
                              className="text-xs text-primary hover:text-primary/80">✓</button>
                            <button onClick={() => setEditingSetId(null)}
                              className="text-xs text-muted-foreground hover:text-foreground">✕</button>
                          </div>
                        </td>
                      </tr>
                    ) : (
                      <tr key={set.id} className="border-b border-border last:border-0 text-sm hover:bg-muted/20 cursor-pointer"
                        onDoubleClick={() => {
                          setEditingSetId(set.id)
                          setEditReps(set.reps.toString())
                          setEditWeight(set.weight?.toString() ?? '')
                          setEditRpe(set.rpe?.toString() ?? '')
                          setEditSetNotes(set.notes ?? '')
                        }}>
                        <td className="px-4 py-2 text-muted-foreground">{set.set_number}</td>
                        <td className="px-4 py-2">{set.reps_label ?? set.reps}</td>
                        <td className="px-4 py-2">{set.weight ? formatWeight(set.weight) : '—'}</td>
                        <td className="px-4 py-2">{set.rpe ?? '—'}</td>
                        <td className="px-4 py-2 text-muted-foreground">{set.notes ?? ''}</td>
                        <td className="px-4 py-2">
                          <button
                            onClick={() => { if (confirm('Delete this set?')) deleteSetMutation.mutate(set.id) }}
                            className="text-xs text-destructive hover:text-destructive/80"
                          >×</button>
                        </td>
                      </tr>
                    )
                  ))}
                </tbody>
              </table>
            </div>
          ))}
        </div>
      )}

      {/* Add Set */}
      <div className="mt-6">
        {!showAddForm ? (
          <button
            onClick={() => setShowAddForm(true)}
            className="rounded-md border border-dashed border-border px-4 py-2 text-sm text-muted-foreground hover:border-primary/50 hover:text-foreground transition-colors w-full"
          >
            + Add Set
          </button>
        ) : (
          <form
            onSubmit={(e) => { e.preventDefault(); addSetMutation.mutate() }}
            className="rounded-lg border border-border bg-card p-4 space-y-3"
          >
            <h3 className="text-sm font-medium">Log Set</h3>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
              <div className="col-span-2">
                <label htmlFor="exercise" className="block text-xs text-muted-foreground mb-1">Exercise</label>
                <select id="exercise" value={exerciseId} onChange={e => setExerciseId(e.target.value)}
                  required
                  className="w-full rounded-md border border-border bg-background px-3 py-1.5 text-sm">
                  <option value="">Select exercise...</option>
                  {exercises?.map(ex => (
                    <option key={ex.id} value={ex.id}>{ex.name}</option>
                  ))}
                </select>
              </div>
              <div>
                <label htmlFor="set-reps" className="block text-xs text-muted-foreground mb-1">Reps</label>
                <input id="set-reps" type="number" value={reps} onChange={e => setReps(e.target.value)}
                  required min={1}
                  className="w-full rounded-md border border-border bg-background px-3 py-1.5 text-sm" />
              </div>
              <div>
                <label htmlFor="set-weight" className="block text-xs text-muted-foreground mb-1">Weight</label>
                <input id="set-weight" type="number" step="0.5" value={setWeight} onChange={e => setSetWeight(e.target.value)}
                  className="w-full rounded-md border border-border bg-background px-3 py-1.5 text-sm" />
              </div>
              <div>
                <label htmlFor="set-rpe" className="block text-xs text-muted-foreground mb-1">RPE</label>
                <input id="set-rpe" type="number" step="0.5" min={1} max={10} value={rpe} onChange={e => setRpe(e.target.value)}
                  className="w-full rounded-md border border-border bg-background px-3 py-1.5 text-sm" />
              </div>
            </div>
            <div className="flex gap-2">
              <button type="submit" disabled={addSetMutation.isPending || !exerciseId || !reps}
                className="rounded-md bg-primary px-4 py-1.5 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50">
                {addSetMutation.isPending ? 'Adding...' : 'Add Set'}
              </button>
              <button type="button" onClick={() => setShowAddForm(false)}
                className="rounded-md border border-border px-4 py-1.5 text-sm hover:bg-accent transition-colors">
                Cancel
              </button>
            </div>
          </form>
        )}
      </div>
    </div>
  )
}
