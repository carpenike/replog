import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'
import { Spinner } from '@/components/ui'
import { useConfirm } from '@/lib/useConfirm'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Card, CardContent } from '@/components/ui/card'
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
        <Button variant="ghost" onClick={() => setShowAdd(!showAdd)}
          >
          {showAdd ? 'Cancel' : '+ Assign'}
        </Button>
      </div>
      {showAdd && (
        <form onSubmit={(e) => { e.preventDefault(); assignMutation.mutate() }}
          className="rounded-lg border border-border bg-card p-4 mb-6 flex flex-wrap gap-3 items-end">
          <div className="flex-1 min-w-50">
            <Label >Exercise</Label>
            <Select value={exerciseId || null} onValueChange={(val) => setExerciseId(val ?? "")} required>
              <SelectTrigger className="w-full">
                <SelectValue placeholder="Select...">
                  {(value: string | null) => {
                    if (!value) return 'Select...'
                    return exercises?.find(ex => String(ex.id) === value)?.name ?? value
                  }}
                </SelectValue>
              </SelectTrigger>
              <SelectContent>
                {exercises?.map(ex => <SelectItem key={ex.id} value={String(ex.id)}>{ex.name}</SelectItem>)}
              </SelectContent>
            </Select>
          </div>
          <div className="w-28">
            <Label >Target Reps</Label>
            <Input type="number" min={0} value={targetReps} onChange={e => setTargetReps(e.target.value)} placeholder="Optional" />
          </div>
          <Button type="submit" disabled={assignMutation.isPending || !exerciseId}
            >
            Assign
          </Button>
        </form>
      )}
      {assignments && assignments.length === 0 ? (
        <p className="text-muted-foreground">No active exercise assignments.</p>
      ) : (
        <div className="space-y-2">
          {assignments?.map(a => (
            <Card size="sm" className="flex items-center justify-between">
              <CardContent>
              <div>
                <p className="text-sm font-medium">{a.exercise_name}</p>
                <p className="text-xs text-muted-foreground">
                  {a.exercise_tier && <span className="capitalize">{a.exercise_tier.replace('_', ' ')} • </span>}
                  {a.target_reps ? `Target: ${a.target_reps} reps` : 'No target reps'}
                </p>
              </div>
              <Button variant="ghost" onClick={async () => {
                if (await confirm({ title: 'Deactivate Assignment', description: `Remove ${a.exercise_name} from assignments?`, confirmLabel: 'Deactivate', variant: 'danger' }))
                  deactivateMutation.mutate(a.id)
              }} className="text-xs text-muted-foreground hover:text-destructive">
                Deactivate
              </Button>
              </CardContent>
            </Card>
          ))}
        </div>
      )}
      {confirmDialog()}
    </div>
  )
}