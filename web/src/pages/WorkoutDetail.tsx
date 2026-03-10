import { useState } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { api } from '@/api/client'
import { Spinner } from '@/components/ui'
import { useConfirm } from '@/lib/useConfirm'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Textarea } from '@/components/ui/textarea'
import { Card, CardContent } from '@/components/ui/card'
function formatWeight(w: number): string {
  return w === Math.floor(w) ? w.toString() : w.toFixed(1)
}
export function WorkoutDetail() {
  const { id, workoutId } = useParams<{ id: string; workoutId: string }>()
  const athleteId = Number(id)
  const wId = Number(workoutId)
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const { confirm, dialog: confirmDialog } = useConfirm()
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
  const [reviewStatus, setReviewStatus] = useState<'approved' | 'needs_work'>('approved')
  const [reviewNotes, setReviewNotes] = useState('')
  const [showReviewForm, setShowReviewForm] = useState(false)
  const { data, isLoading, error } = useQuery({
    queryKey: ['workout', athleteId, wId],
    queryFn: () => api.getWorkout(athleteId, wId),
    enabled: !isNaN(athleteId) && !isNaN(wId),
  })
  const { data: me } = useQuery({
    queryKey: ['me'],
    queryFn: () => api.me(),
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
      toast.success('Set added')
    },
  })
  const deleteSetMutation = useMutation({
    mutationFn: (setId: number) => api.deleteSet(athleteId, wId, setId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['workout', athleteId, wId] })
      toast.success('Set deleted')
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
  const submitReviewMutation = useMutation({
    mutationFn: () => api.submitReview(athleteId, wId, reviewStatus, reviewNotes),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['workout', athleteId, wId] })
      queryClient.invalidateQueries({ queryKey: ['pending-reviews'] })
      setShowReviewForm(false)
      setReviewNotes('')
    },
  })
  if (isLoading) return <Spinner />
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
      {/* Editable notes */}
      {editingNotes ? (
        <Card size="sm" className="mb-4">
          <CardContent>
          <Textarea value={notesText} onChange={e => setNotesText(e.target.value)}
            rows={3}
           
            placeholder="Session notes..."
          />
          <div className="flex gap-2">
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
                    <TableHead>Notes</TableHead>
                    <TableHead className="w-10"></TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {group.sets.map(set => (
                    editingSetId === set.id ? (
                      <TableRow key={set.id} className="bg-muted/30">
                        <TableCell className="text-muted-foreground">{set.set_number}</TableCell>
                        <TableCell>
                          <Input type="number" value={editReps} onChange={e => setEditReps(e.target.value)} min={1} className="w-16" />
                        </TableCell>
                        <TableCell>
                          <Input type="number" step="0.5" value={editWeight} onChange={e => setEditWeight(e.target.value)} className="w-20" />
                        </TableCell>
                        <TableCell>
                          <Input type="number" step="0.5" min={1} max={10} value={editRpe} onChange={e => setEditRpe(e.target.value)} className="w-16" />
                        </TableCell>
                        <TableCell>
                          <Input type="text" value={editSetNotes} onChange={e => setEditSetNotes(e.target.value)} />
                        </TableCell>
                        <TableCell>
                          <div className="flex gap-1">
                            <Button variant="ghost" onClick={() => updateSetMutation.mutate(set.id)}
                              disabled={updateSetMutation.isPending}
                              >✓</Button>
                            <Button variant="ghost" onClick={() => setEditingSetId(null)}
                              >✕</Button>
                          </div>
                        </TableCell>
                      </TableRow>
                    ) : (
                      <TableRow key={set.id} className="cursor-pointer"
                        onDoubleClick={() => {
                          setEditingSetId(set.id)
                          setEditReps(set.reps.toString())
                          setEditWeight(set.weight?.toString() ?? '')
                          setEditRpe(set.rpe?.toString() ?? '')
                          setEditSetNotes(set.notes ?? '')
                        }}>
                        <TableCell className="text-muted-foreground">{set.set_number}</TableCell>
                        <TableCell>{set.reps_label ?? set.reps}</TableCell>
                        <TableCell>{set.weight ? formatWeight(set.weight) : '—'}</TableCell>
                        <TableCell>{set.rpe ?? '—'}</TableCell>
                        <TableCell className="text-muted-foreground">{set.notes ?? ''}</TableCell>
                        <TableCell>
                          <Button variant="ghost" size="xs" onClick={async () => {
                              if (await confirm({ title: 'Delete Set', description: 'Remove this set?', confirmLabel: 'Delete', variant: 'danger' }))
                                deleteSetMutation.mutate(set.id)
                            }}
                          >×</Button>
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
          <Button variant="ghost" onClick={() => setShowAddForm(true)}
            className="rounded-md border border-dashed border-border px-4 py-2 text-sm text-muted-foreground hover:border-primary/50 hover:text-foreground transition-colors w-full"
          >
            + Add Set
          </Button>
        ) : (
          <form
            onSubmit={(e) => { e.preventDefault(); addSetMutation.mutate() }}
            className="rounded-lg border border-border bg-card p-4 space-y-3"
          >
            <h3 className="text-sm font-medium">Log Set</h3>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
              <div className="col-span-2">
                <Label>Exercise</Label>
                <Select value={exerciseId} onValueChange={(val) => setExerciseId(val ?? "")} required>
                  <SelectTrigger className="w-full">
                    <SelectValue placeholder="Select exercise..." />
                  </SelectTrigger>
                  <SelectContent>
                    {exercises?.map(ex => (
                      <SelectItem key={ex.id} value={String(ex.id)}>{ex.name}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div>
                <Label htmlFor="set-reps" >Reps</Label>
                <Input id="set-reps" type="number" value={reps} onChange={e => setReps(e.target.value)} required min={1} />
              </div>
              <div>
                <Label htmlFor="set-weight" >Weight</Label>
                <Input id="set-weight" type="number" step="0.5" value={setWeight} onChange={e => setSetWeight(e.target.value)} />
              </div>
              <div>
                <Label htmlFor="set-rpe" >RPE</Label>
                <Input id="set-rpe" type="number" step="0.5" min={1} max={10} value={rpe} onChange={e => setRpe(e.target.value)} />
              </div>
            </div>
            <div className="flex gap-2">
              <Button type="submit" disabled={addSetMutation.isPending || !exerciseId || !reps}
                >
                {addSetMutation.isPending ? 'Adding...' : 'Add Set'}
              </Button>
              <Button variant="ghost" type="button" onClick={() => setShowAddForm(false)}
                >
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
            <Button variant="ghost" onClick={() => setShowReviewForm(true)}
              className="text-sm text-primary hover:text-primary/80">
              {workout.review_status ? 'Update Review' : 'Review Workout'}
            </Button>
          )}
          </CardContent>
        </Card>
      )}
      {confirmDialog()}
    </div>
  )
}