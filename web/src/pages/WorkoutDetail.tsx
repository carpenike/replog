import { useState } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Check, Pencil, Play, Plus, Trash2, X } from 'lucide-react'
import { api } from '@/api/client'
import { EmptyState, QueryError } from '@/components/ui'
import { ExercisePicker, type PickedExercise } from '@/components/ExercisePicker'
import { useConfirm } from '@/lib/useConfirm'
import { usePageTitle } from '@/lib/usePageTitle'
import { formatDate, formatWeight, localDateISO } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Textarea } from '@/components/ui/textarea'
import { Card, CardContent } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import type { ExerciseGroup, Workout, WorkoutSet } from '@/api/types'

type WorkoutData = { workout: Workout; groups: ExerciseGroup[] }

export function WorkoutDetail() {
  const { id, workoutId } = useParams<{ id: string; workoutId: string }>()
  const athleteId = Number(id)
  const wId = Number(workoutId)
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const { confirm, dialog: confirmDialog } = useConfirm()
  const queryKey = ['workout', athleteId, wId] as const

  const [exerciseId, setExerciseId] = useState('')
  const [exerciseName, setExerciseName] = useState('')
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
  const [reviewStatus, setReviewStatus] = useState<'approved' | 'needs_work'>('approved')
  const [reviewNotes, setReviewNotes] = useState('')
  const [showReviewForm, setShowReviewForm] = useState(false)

  const { data, isLoading, error, refetch } = useQuery({
    queryKey,
    queryFn: () => api.getWorkout(athleteId, wId),
    enabled: !isNaN(athleteId) && !isNaN(wId),
  })
  const { data: me } = useQuery({
    queryKey: ['me'],
    queryFn: () => api.me(),
  })
  // Today's prescription drives the reps/weight prefill after picking a lift.
  const { data: prescription } = useQuery({
    queryKey: ['prescription', athleteId],
    queryFn: () => api.getPrescription(athleteId),
    enabled: showAddForm && !isNaN(athleteId),
    retry: false,
  })

  usePageTitle(data ? `Workout · ${formatDate(data.workout.date)}` : 'Workout')

  const addSetMutation = useMutation({
    mutationFn: () => api.addSet(athleteId, wId, {
      exercise_id: parseInt(exerciseId),
      reps: parseInt(reps),
      weight: setWeight ? parseFloat(setWeight) : undefined,
      rpe: rpe ? parseFloat(rpe) : undefined,
    }),
    onMutate: async () => {
      await queryClient.cancelQueries({ queryKey })
      const prev = queryClient.getQueryData<WorkoutData>(queryKey)
      const exName = exerciseName || 'Exercise'
      const exId = parseInt(exerciseId)
      const optimistic: WorkoutSet = {
        id: -Date.now(),
        workout_id: wId,
        exercise_id: exId,
        set_number: 0,
        reps: parseInt(reps),
        weight: setWeight ? parseFloat(setWeight) : null,
        rpe: rpe ? parseFloat(rpe) : null,
        rep_type: 'standard',
        category: '',
        notes: null,
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
      }
      if (prev) {
        const groups = [...prev.groups]
        const gi = groups.findIndex(g => g.exercise_id === exId)
        if (gi >= 0) {
          const sets = [...groups[gi].sets, { ...optimistic, set_number: groups[gi].sets.length + 1 }]
          groups[gi] = { ...groups[gi], sets }
        } else {
          groups.push({ exercise_id: exId, exercise_name: exName, sets: [{ ...optimistic, set_number: 1 }] })
        }
        queryClient.setQueryData<WorkoutData>(queryKey, { ...prev, groups })
      }
      return { prev }
    },
    onError: (_err, _vars, ctx) => {
      if (ctx?.prev) queryClient.setQueryData(queryKey, ctx.prev)
    },
    onSuccess: () => {
      // Keep exercise + weight sticky between sets; clear only reps/rpe.
      setReps('')
      setRpe('')
      toast.success('Set added')
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey }),
  })

  const deleteSetMutation = useMutation({
    mutationFn: (setId: number) => api.deleteSet(athleteId, wId, setId),
    onMutate: async (setId: number) => {
      await queryClient.cancelQueries({ queryKey })
      const prev = queryClient.getQueryData<WorkoutData>(queryKey)
      if (prev) {
        const groups = prev.groups
          .map(g => ({ ...g, sets: g.sets.filter(s => s.id !== setId) }))
          .filter(g => g.sets.length > 0)
        queryClient.setQueryData<WorkoutData>(queryKey, { ...prev, groups })
      }
      return { prev }
    },
    onError: (_err, _vars, ctx) => {
      if (ctx?.prev) queryClient.setQueryData(queryKey, ctx.prev)
    },
    onSuccess: () => toast.success('Set deleted'),
    onSettled: () => queryClient.invalidateQueries({ queryKey }),
  })

  const updateSetMutation = useMutation({
    mutationFn: (setId: number) => api.updateSet(athleteId, wId, setId, {
      // Send explicit values (not undefined) so an intentionally-cleared field
      // is cleared. The API now treats an OMITTED field as "leave unchanged"
      // (pointer/omitempty), so the SPA — which always edits the full set — must
      // send 0 / "" to clear weight/rpe/notes rather than dropping the key.
      reps: parseInt(editReps),
      weight: editWeight ? parseFloat(editWeight) : 0,
      rpe: editRpe ? parseFloat(editRpe) : 0,
      notes: editSetNotes,
    }),
    onMutate: async (setId: number) => {
      await queryClient.cancelQueries({ queryKey })
      const prev = queryClient.getQueryData<WorkoutData>(queryKey)
      if (prev) {
        const groups = prev.groups.map(g => ({
          ...g,
          sets: g.sets.map(s => s.id === setId ? {
            ...s,
            reps: parseInt(editReps),
            weight: editWeight ? parseFloat(editWeight) : null,
            rpe: editRpe ? parseFloat(editRpe) : null,
            notes: editSetNotes || null,
          } : s),
        }))
        queryClient.setQueryData<WorkoutData>(queryKey, { ...prev, groups })
      }
      return { prev }
    },
    onError: (_err, _vars, ctx) => {
      if (ctx?.prev) queryClient.setQueryData(queryKey, ctx.prev)
    },
    onSuccess: () => setEditingSetId(null),
    onSettled: () => queryClient.invalidateQueries({ queryKey }),
  })

  const deleteWorkoutMutation = useMutation({
    mutationFn: () => api.deleteWorkout(athleteId, wId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['workouts', athleteId] })
      navigate(`/athletes/${athleteId}/workouts`)
    },
  })
  const updateNotesMutation = useMutation({
    mutationFn: () => api.updateWorkoutNotes(athleteId, wId, notesText),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey })
      setEditingNotes(false)
    },
  })
  const submitReviewMutation = useMutation({
    mutationFn: () => api.submitReview(athleteId, wId, reviewStatus, reviewNotes),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey })
      queryClient.invalidateQueries({ queryKey: ['pending-reviews'] })
      setShowReviewForm(false)
      setReviewNotes('')
    },
  })
  const clearReviewMutation = useMutation({
    mutationFn: () => api.deleteReview(athleteId, wId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey })
      queryClient.invalidateQueries({ queryKey: ['pending-reviews'] })
    },
  })

  function pickExercise(ex: PickedExercise) {
    setExerciseId(String(ex.id))
    setExerciseName(ex.name)
    // Prefill reps/weight from today's prescription line where available.
    const line = prescription?.lines.find(l => l.exercise_id === ex.id)
    const firstSet = line?.sets?.[0]
    if (firstSet?.reps != null) setReps(String(firstSet.reps))
    const target = firstSet?.absolute_weight ?? firstSet?.target_weight
    if (target != null && !setWeight) setSetWeight(String(target))
  }

  function beginEdit(set: WorkoutSet) {
    setEditingSetId(set.id)
    setEditReps(set.reps.toString())
    setEditWeight(set.weight?.toString() ?? '')
    setEditRpe(set.rpe?.toString() ?? '')
    setEditSetNotes(set.notes ?? '')
  }

  if (isLoading) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-4 w-48" />
        <Skeleton className="h-8 w-40" />
        <Skeleton className="h-24 w-full" />
        <Skeleton className="h-40 w-full" />
      </div>
    )
  }
  if (error) return <QueryError error={error} onRetry={refetch} resource="workout" />
  if (!data) return <EmptyState title="Workout not found" description="It may have been deleted." />
  const { workout, groups } = data

  return (
    <div>
      <p className="text-sm text-muted-foreground mb-1">
        <Link to={`/athletes/${athleteId}`} className="hover:text-foreground">{workout.athlete_name || 'Athlete'}</Link>
        {' / '}
        <Link to={`/athletes/${athleteId}/workouts`} className="hover:text-foreground">Workouts</Link>
        {' / '}
        {formatDate(workout.date)}
      </p>
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold mb-2">{formatDate(workout.date)}</h1>
        <Button variant="ghost" onClick={async () => {
            if (await confirm({ title: 'Delete Workout', description: 'Delete this workout and all its sets? This cannot be undone.', confirmLabel: 'Delete', variant: 'danger' }))
              deleteWorkoutMutation.mutate()
          }}
          className="text-sm text-destructive hover:text-destructive/80">
          Delete
        </Button>
      </div>
      {workout.program_name && (
        <p className="text-sm text-muted-foreground mb-4">{workout.program_name}</p>
      )}
      {/* Today's workout gets a fast path into the live session screen. */}
      {formatDate(workout.date) === localDateISO() && (
        <Button
          size="touch"
          render={<Link to={`/athletes/${athleteId}/workouts/${wId}/session`} />}
          className="w-full mb-4"
        >
          <Play aria-hidden="true" /> Continue Session
        </Button>
      )}
      {/* Editable notes */}
      {editingNotes ? (
        <Card size="sm" className="mb-4">
          <CardContent>
          <Textarea value={notesText} onChange={e => setNotesText(e.target.value)}
            rows={3}
            placeholder="Session notes..."
          />
          <div className="flex gap-2 mt-2">
            <Button variant="ghost" onClick={() => updateNotesMutation.mutate()}
              disabled={updateNotesMutation.isPending}
              >
              Save
            </Button>
            <Button variant="ghost" onClick={() => setEditingNotes(false)}
              >
              Cancel
            </Button>
          </div>
          </CardContent>
        </Card>
      ) : (
        <button
          type="button"
          onClick={() => { setNotesText(workout.notes ?? ''); setEditingNotes(true) }}
          className="w-full text-left rounded-lg border border-border bg-card p-3 mb-4 text-sm cursor-pointer hover:border-primary/30 transition-colors"
        >
          {workout.notes || <span className="text-muted-foreground italic">Add notes...</span>}
        </button>
      )}
      {groups.length === 0 ? (
        <EmptyState icon="🏋️" title="No sets logged yet" description="Use Add Set below to start logging." />
      ) : (
        <div className="space-y-6">
          {groups.map(group => (
            <div key={group.exercise_id} className="rounded-lg border border-border overflow-hidden">
              <div className="bg-muted/50 px-4 py-2 border-b border-border flex items-center justify-between">
                <h3 className="font-semibold">{group.exercise_name}</h3>
                <Link to={`/athletes/${athleteId}/exercises/${group.exercise_id}/history`}
                  className="text-xs text-muted-foreground hover:text-primary">
                  History
                </Link>
              </div>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="w-12">Set</TableHead>
                    <TableHead>Reps</TableHead>
                    <TableHead>Weight</TableHead>
                    <TableHead>RPE</TableHead>
                    <TableHead className="whitespace-normal">Notes</TableHead>
                    <TableHead className="w-12 sm:w-24 text-right">Edit</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {group.sets.map(set => (
                    editingSetId === set.id ? (
                      <TableRow key={set.id} className="bg-muted/30">
                        <TableCell className="text-muted-foreground">{set.set_number}</TableCell>
                        <TableCell>
                          <Input type="number" inputMode="numeric" enterKeyHint="done" value={editReps} onChange={e => setEditReps(e.target.value)} min={1} className="h-11 w-16" aria-label="Reps" />
                        </TableCell>
                        <TableCell>
                          <Input type="number" inputMode="decimal" enterKeyHint="done" step="0.5" value={editWeight} onChange={e => setEditWeight(e.target.value)} className="h-11 w-20" aria-label="Weight" />
                        </TableCell>
                        <TableCell>
                          <Input type="number" inputMode="decimal" enterKeyHint="done" step="0.5" min={1} max={10} value={editRpe} onChange={e => setEditRpe(e.target.value)} className="h-11 w-16" aria-label="RPE" />
                        </TableCell>
                        <TableCell>
                          <Input type="text" value={editSetNotes} onChange={e => setEditSetNotes(e.target.value)} className="h-11" aria-label="Set notes" />
                        </TableCell>
                        <TableCell>
                          <div className="flex gap-1 justify-end">
                            <Button variant="ghost" size="icon-touch" aria-label="Save set" onClick={() => updateSetMutation.mutate(set.id)}
                              disabled={updateSetMutation.isPending}
                              ><Check aria-hidden="true" /></Button>
                            <Button variant="ghost" size="icon-touch" aria-label="Cancel edit" onClick={() => setEditingSetId(null)}
                              ><X aria-hidden="true" /></Button>
                          </div>
                        </TableCell>
                      </TableRow>
                    ) : (
                      <TableRow
                        key={set.id}
                        role="button"
                        tabIndex={0}
                        aria-label={`Edit set ${set.set_number}`}
                        className="cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/50"
                        onClick={() => beginEdit(set)}
                        onKeyDown={e => {
                          if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); beginEdit(set) }
                        }}
                      >
                        <TableCell className="text-muted-foreground">{set.set_number}</TableCell>
                        <TableCell>{set.reps_label ?? set.reps}</TableCell>
                        <TableCell>{formatWeight(set.weight)}</TableCell>
                        <TableCell>{set.rpe ?? '—'}</TableCell>
                        <TableCell className="text-muted-foreground text-xs whitespace-normal wrap-break-word">{set.notes ?? ''}</TableCell>
                        <TableCell onClick={e => e.stopPropagation()}>
                          <div className="flex gap-1 justify-end">
                            {/* Row tap already opens edit — the pencil is redundant at phone widths
                                and its 44px column pushed Notes off-screen. */}
                            <Button variant="ghost" size="icon-touch" aria-label={`Edit set ${set.set_number}`}
                              className="hidden sm:inline-flex"
                              onClick={() => beginEdit(set)}
                            ><Pencil aria-hidden="true" /></Button>
                            <Button variant="ghost" size="icon-touch" aria-label={`Delete set ${set.set_number}`}
                              className="text-muted-foreground hover:text-destructive"
                              onClick={async () => {
                                if (await confirm({ title: 'Delete Set', description: 'Remove this set?', confirmLabel: 'Delete', variant: 'danger' }))
                                  deleteSetMutation.mutate(set.id)
                              }}
                            ><Trash2 aria-hidden="true" /></Button>
                          </div>
                        </TableCell>
                      </TableRow>
                    )
                  ))}
                </TableBody>
              </Table>
            </div>
          ))}
        </div>
      )}
      {/* Add Set */}
      <div className="mt-6">
        {!showAddForm ? (
          <Button variant="outline" size="touch" onClick={() => setShowAddForm(true)}
            className="w-full border-dashed text-muted-foreground hover:text-foreground"
          >
            <Plus aria-hidden="true" /> Add Set
          </Button>
        ) : (
          <form
            onSubmit={(e) => { e.preventDefault(); addSetMutation.mutate() }}
            className="rounded-lg border border-border bg-card p-4 space-y-3"
          >
            <h3 className="text-sm font-medium">Log Set</h3>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
              <div className="col-span-2">
                <Label id="exercise-picker-label">Exercise</Label>
                <ExercisePicker
                  athleteId={athleteId}
                  value={exerciseId ? Number(exerciseId) : null}
                  onSelect={pickExercise}
                  triggerLabelId="exercise-picker-label"
                />
              </div>
              <div>
                <Label htmlFor="set-reps">Reps</Label>
                <Input id="set-reps" type="number" inputMode="numeric" enterKeyHint="done" value={reps} onChange={e => setReps(e.target.value)} required min={1} className="h-11 mt-1" />
              </div>
              <div>
                <Label htmlFor="set-weight">Weight</Label>
                <Input id="set-weight" type="number" inputMode="decimal" enterKeyHint="done" step="0.5" value={setWeight} onChange={e => setSetWeight(e.target.value)} className="h-11 mt-1" />
              </div>
              <div>
                <Label htmlFor="set-rpe">RPE</Label>
                <Input id="set-rpe" type="number" inputMode="decimal" enterKeyHint="done" step="0.5" min={1} max={10} value={rpe} onChange={e => setRpe(e.target.value)} className="h-11 mt-1" />
              </div>
            </div>
            <div className="flex gap-2">
              <Button type="submit" size="touch" disabled={addSetMutation.isPending || !exerciseId || !reps}>
                {addSetMutation.isPending ? 'Adding...' : 'Add Set'}
              </Button>
              <Button variant="ghost" size="touch" type="button" onClick={() => setShowAddForm(false)}>
                Cancel
              </Button>
            </div>
          </form>
        )}
      </div>
      {/* Coach Review Section */}
      {me && (me.is_coach || me.is_admin) && (
        <Card className="mt-6">
          <CardContent>
          <div className="flex items-center justify-between mb-3">
            <h3 className="font-semibold">Coach Review</h3>
            {workout.review_status && (
              <span className={`text-xs px-2 py-0.5 rounded-full ${
                workout.review_status === 'approved' ? 'bg-success/10 text-success' : 'bg-warning/10 text-warning'
              }`}>
                {workout.review_status === 'approved' ? '✓ Approved' : '⚠ Needs Work'}
              </span>
            )}
          </div>
          {showReviewForm ? (
            <div className="space-y-3">
              <div className="flex gap-3">
                <Label>
                  <input type="radio" name="review" checked={reviewStatus === 'approved'}
                    onChange={() => setReviewStatus('approved')} />
                  <span className="text-sm text-success">Approve</span>
                </Label>
                <Label>
                  <input type="radio" name="review" checked={reviewStatus === 'needs_work'}
                    onChange={() => setReviewStatus('needs_work')} />
                  <span className="text-sm text-warning">Needs Work</span>
                </Label>
              </div>
              <Textarea value={reviewNotes} onChange={e => setReviewNotes(e.target.value)}
                rows={2} placeholder="Review notes (optional)..."
                />
              <div className="flex gap-2">
                <Button variant="ghost" onClick={() => submitReviewMutation.mutate()}
                  disabled={submitReviewMutation.isPending}
                  >
                  {submitReviewMutation.isPending ? 'Submitting...' : 'Submit Review'}
                </Button>
                <Button variant="ghost" onClick={() => setShowReviewForm(false)}
                  >
                  Cancel
                </Button>
              </div>
            </div>
          ) : (
            <div className="flex gap-2 items-center">
              <Button variant="ghost" onClick={() => setShowReviewForm(true)}
                className="text-sm text-primary hover:text-primary/80">
                {workout.review_status ? 'Update Review' : 'Review Workout'}
              </Button>
              {workout.review_status && (
                <Button
                  variant="ghost"
                  className="text-sm text-muted-foreground hover:text-destructive"
                  disabled={clearReviewMutation.isPending}
                  onClick={async () => {
                    if (await confirm({
                      title: 'Clear Review',
                      description: 'Remove this review? The workout will return to pending.',
                      confirmLabel: 'Clear',
                      variant: 'danger',
                    })) clearReviewMutation.mutate()
                  }}
                >
                  Clear
                </Button>
              )}
            </div>
          )}
          </CardContent>
        </Card>
      )}
      {confirmDialog()}
    </div>
  )
}
