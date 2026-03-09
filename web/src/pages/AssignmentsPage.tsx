import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'
import { Spinner } from '@/components/ui'
import { useConfirm } from '@/lib/useConfirm'

export function AssignmentsPage() {
  const { id } = useParams<{ id: string }>()
  const athleteId = Number(id)
  const queryClient = useQueryClient()
  const { confirm, dialog: confirmDialog } = useConfirm()

  const [showAdd, setShowAdd] = useState(false)
  const [exerciseId, setExerciseId] = useState('')
  const [targetReps, setTargetReps] = useState('')

  const { data: athlete } = useQuery({
    queryKey: ['athlete', athleteId],
    queryFn: () => api.getAthlete(athleteId),
    enabled: !isNaN(athleteId),
  })

  const { data: assignments, isLoading } = useQuery({
    queryKey: ['assignments', athleteId],
    queryFn: () => api.listAssignments(athleteId),
    enabled: !isNaN(athleteId),
  })

  const { data: exercises } = useQuery({
    queryKey: ['exercises'],
    queryFn: () => api.listExercises(),
    enabled: showAdd,
  })

  const assignMutation = useMutation({
    mutationFn: () => api.assignExercise(athleteId, parseInt(exerciseId), targetReps ? parseInt(targetReps) : 0),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['assignments', athleteId] })
      setExerciseId('')
      setTargetReps('')
      setShowAdd(false)
    },
  })

  const deactivateMutation = useMutation({
    mutationFn: (assignmentId: number) => api.deactivateAssignment(athleteId, assignmentId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['assignments', athleteId] }),
  })

  if (isLoading) return <Spinner />

  return (
    <div>
      <p className="text-sm text-muted-foreground mb-1">
        <Link to={`/athletes/${athleteId}`} className="hover:text-foreground">{athlete?.name ?? 'Athlete'}</Link>
        {' / Assignments'}
      </p>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">Exercise Assignments</h1>
        <button onClick={() => setShowAdd(!showAdd)}
          className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90">
          {showAdd ? 'Cancel' : '+ Assign'}
        </button>
      </div>

      {showAdd && (
        <form onSubmit={(e) => { e.preventDefault(); assignMutation.mutate() }}
          className="rounded-lg border border-border bg-card p-4 mb-6 flex flex-wrap gap-3 items-end">
          <div className="flex-1 min-w-50">
            <label className="block text-xs text-muted-foreground mb-1">Exercise</label>
            <select value={exerciseId} onChange={e => setExerciseId(e.target.value)} required
              className="w-full rounded-md border border-border bg-background px-3 py-1.5 text-sm">
              <option value="">Select...</option>
              {exercises?.map(ex => <option key={ex.id} value={ex.id}>{ex.name}</option>)}
            </select>
          </div>
          <div className="w-28">
            <label className="block text-xs text-muted-foreground mb-1">Target Reps</label>
            <input type="number" min={0} value={targetReps} onChange={e => setTargetReps(e.target.value)}
              placeholder="Optional"
              className="w-full rounded-md border border-border bg-background px-3 py-1.5 text-sm" />
          </div>
          <button type="submit" disabled={assignMutation.isPending || !exerciseId}
            className="rounded-md bg-primary px-4 py-1.5 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50">
            Assign
          </button>
        </form>
      )}

      {assignments && assignments.length === 0 ? (
        <p className="text-muted-foreground">No active exercise assignments.</p>
      ) : (
        <div className="space-y-2">
          {assignments?.map(a => (
            <div key={a.id} className="flex items-center justify-between rounded-lg border border-border bg-card p-3">
              <div>
                <p className="text-sm font-medium">{a.exercise_name}</p>
                <p className="text-xs text-muted-foreground">
                  {a.exercise_tier && <span className="capitalize">{a.exercise_tier.replace('_', ' ')} • </span>}
                  {a.target_reps ? `Target: ${a.target_reps} reps` : 'No target reps'}
                </p>
              </div>
              <button onClick={async () => {
                if (await confirm({ title: 'Deactivate Assignment', description: `Remove ${a.exercise_name} from assignments?`, confirmLabel: 'Deactivate', variant: 'danger' }))
                  deactivateMutation.mutate(a.id)
              }} className="text-xs text-muted-foreground hover:text-destructive">
                Deactivate
              </button>
            </div>
          ))}
        </div>
      )}
      {confirmDialog()}
    </div>
  )
}
