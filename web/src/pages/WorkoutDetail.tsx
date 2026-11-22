import { useState } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { AlertTriangle, Check, Film, Pencil, Play, Plus, Trash2, X } from 'lucide-react'
import { api } from '@/api/client'
import { EmptyState } from '@/components/ui'
import { PageHeader } from '@/components/PageHeader'
import { QueryState } from '@/components/QueryState'
import { useFriendlyDate } from '@/lib/dates'
import { ExercisePicker, type PickedExercise } from '@/components/ExercisePicker'
import { SessionFlags } from '@/components/SessionFlags'
import { SetVideoControl } from '@/components/SetVideoSheet'
import { Badge } from '@/components/ui/badge'
import { useConfirm } from '@/lib/useConfirm'
import { formatDate, formatWeight, localDateISO } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Textarea } from '@/components/ui/textarea'
import { Card, CardContent } from '@/components/ui/card'
import type { WorkoutDetailData as WorkoutData, WorkoutSet } from '@/api/types'

const SET_TYPE_BADGE: Record<string, string> = { warmup: 'W', drop: 'Drop', failure: 'Fail' }

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

  const friendlyDate = useFriendlyDate()
  const workoutQuery = useQuery({
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

  const data = workoutQuery.data

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

  if (!data) {
    return (
      <div>
        <PageHeader title="Workout" pageTitle={null} back={{ to: `/athletes/${athleteId}/workouts`, label: 'Workouts' }} />
        <QueryState
          query={workoutQuery}
          resource="workout"
          skeleton="detail"
          notFound={<EmptyState icon="🔍" title="Workout not found" description="It may have been deleted." action="All workouts" actionTo={`/athletes/${athleteId}/workouts`} />}
        >
          {() => null}
        </QueryState>
      </div>
    )
  }
  const { workout, groups } = data
  const videoCount = groups.reduce((n, g) => n + g.sets.filter(s => s.video).length, 0)
  const relative = friendlyDate(workout.date)
  const absolute = friendlyDate(workout.date, { absolute: true })

  return (
    <div>
      <PageHeader
        title={absolute}
        pageTitle={`Workout · ${absolute}`}
        back={{ to: `/athletes/${athleteId}/workouts`, label: workout.athlete_name ? `${workout.athlete_name}'s workouts` : 'Workouts' }}
        description={[relative !== absolute ? relative : null, workout.program_name].filter(Boolean).join(' · ') || undefined}
        actions={
          <Button variant="destructive" onClick={async () => {
            if (await confirm({ title: 'Delete Workout', description: 'Delete this workout and all its sets? This cannot be undone.', confirmLabel: 'Delete', variant: 'danger' }))
              deleteWorkoutMutation.mutate()
          }}>
            <Trash2 aria-hidden="true" /> Delete
          </Button>
        }
      />
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
          <Textarea aria-label="Session notes" value={notesText} onChange={e => setNotesText(e.target.value)}
            rows={3}
            placeholder="Session notes..."
          />
          <div className="flex gap-2 mt-2">
            <Button onClick={() => updateNotesMutation.mutate()}
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
      <SessionFlags readiness={data.readiness} swaps={data.swaps} />
      {groups.length === 0 ? (
        <EmptyState icon="🏋️" title="No sets logged yet" description="Use Add Set below to start logging." />
      ) : (
        <div className="space-y-6">
          {groups.map(group => (
            <div key={group.exercise_id} className="rounded-lg border border-border overflow-hidden">
              <div className="bg-muted/50 px-4 py-2 border-b border-border flex items-center justify-between">
                <h2 className="font-semibold">{group.exercise_name}</h2>
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
                    <TableHead className="w-24 sm:w-36 text-right">Edit</TableHead>
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
                      // Tapping anywhere on the row opens edit (pointer convenience);
                      // the set-number button is the keyboard and screen-reader path,
                      // so the row itself is not a fake button wrapping real ones.
                      <TableRow
                        key={set.id}
                        className="cursor-pointer"
                        onClick={() => beginEdit(set)}
                      >
                        <TableCell className="text-muted-foreground">
                          <button
                            type="button"
                            aria-label={`Edit set ${set.set_number}`}
                            className="min-h-11 min-w-8 rounded-md text-left focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-ring/50 md:min-h-0"
                            onClick={e => { e.stopPropagation(); beginEdit(set) }}
                          >
                            {set.set_number}
                          </button>
                          {set.set_type && SET_TYPE_BADGE[set.set_type] && (
                            <Badge variant="outline" className="ml-1 text-[10px]" aria-label={set.set_type === 'warmup' ? 'Warm-up set' : `${set.set_type} set`}>
                              {SET_TYPE_BADGE[set.set_type]}
                            </Badge>
                          )}
                        </TableCell>
                        <TableCell>{set.reps_label ?? set.reps}</TableCell>
                        <TableCell>{formatWeight(set.weight)}</TableCell>
                        <TableCell>{set.rpe ?? '—'}</TableCell>
                        <TableCell className="text-muted-foreground text-xs whitespace-normal wrap-break-word">{set.notes ?? ''}</TableCell>
                        <TableCell onClick={e => e.stopPropagation()}>
                          <div className="flex gap-1 justify-end">
                            {set.id > 0 && (
                              <SetVideoControl
                                athleteId={athleteId}
                                workoutId={wId}
                                setId={set.id}
                                setNumber={set.set_number}
                                exerciseName={group.exercise_name}
                                video={set.video}
                                canComment={!!me && (me.is_coach || me.is_admin)}
                              />
                            )}
                            {/* Row tap already opens edit — the pencil is redundant at phone widths
                                and its 44px column pushed Notes off-screen. */}
                            <Button variant="ghost" size="icon-touch" aria-hidden="true" tabIndex={-1}
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
            <h2 className="text-sm font-medium">Log Set</h2>
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
                <Label htmlFor="set-reps">Reps <span aria-hidden="true" className="text-destructive">*</span></Label>
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
            <h2 className="font-semibold">Coach Review</h2>
            {workout.review_status && (
              <span className={`text-xs px-2 py-0.5 rounded-full ${
                workout.review_status === 'approved' ? 'bg-success/10 text-success-strong' : 'bg-warning/10 text-warning-strong'
              }`}>
                {workout.review_status === 'approved'
                  ? <span className="inline-flex items-center gap-1"><Check className="size-3" aria-hidden="true" /> Approved</span>
                  : <span className="inline-flex items-center gap-1"><AlertTriangle className="size-3" aria-hidden="true" /> Needs Work</span>}
              </span>
            )}
          </div>
          {videoCount > 0 && (
            <p className="mb-3 flex items-center gap-1.5 text-sm text-muted-foreground">
              <Film className="size-4" aria-hidden="true" />
              {videoCount} form-check video{videoCount === 1 ? '' : 's'} to watch — open one from its set above to comment.
            </p>
          )}
          {showReviewForm ? (
            <div className="space-y-3">
              <div role="radiogroup" aria-label="Review outcome" className="flex gap-3">
                <Label className="min-h-11 md:min-h-0">
                  <input type="radio" name="review" checked={reviewStatus === 'approved'}
                    onChange={() => setReviewStatus('approved')} />
                  <span className="text-sm text-success-strong">Approve</span>
                </Label>
                <Label className="min-h-11 md:min-h-0">
                  <input type="radio" name="review" checked={reviewStatus === 'needs_work'}
                    onChange={() => setReviewStatus('needs_work')} />
                  <span className="text-sm text-warning-strong">Needs Work</span>
                </Label>
              </div>
              <Textarea aria-label="Review notes" value={reviewNotes} onChange={e => setReviewNotes(e.target.value)}
                rows={2} placeholder="Review notes (optional)..."
                />
              <div className="flex gap-2">
                <Button onClick={() => submitReviewMutation.mutate()}
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
