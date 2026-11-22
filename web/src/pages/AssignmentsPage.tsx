import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'
import { Plus, RotateCcw, X } from 'lucide-react'
import { PageHeader } from '@/components/PageHeader'
import { QueryState } from '@/components/QueryState'
import { useFriendlyDate } from '@/lib/dates'
import { useConfirm } from '@/lib/useConfirm'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Card, CardContent } from '@/components/ui/card'
import { EmptyState } from '@/components/ui'
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
  const friendlyDate = useFriendlyDate()
  const query = useQuery({
    queryKey: ['assignments', athleteId, 'all'],
    queryFn: () => api.listAssignments(athleteId, true),
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
      queryClient.invalidateQueries({ queryKey: ['assignments', athleteId, 'all'] })
      setExerciseId('')
      setTargetReps('')
      setShowAdd(false)
    },
  })
  const deactivateMutation = useMutation({
    mutationFn: (assignmentId: number) => api.deactivateAssignment(athleteId, assignmentId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['assignments', athleteId, 'all'] }),
  })
  const reactivateMutation = useMutation({
    mutationFn: (vars: { exerciseId: number; targetReps: number }) =>
      api.reactivateAssignment(athleteId, vars.exerciseId, vars.targetReps),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['assignments', athleteId, 'all'] }),
  })
  return (
    <div>
      <PageHeader
        title="Exercise Assignments"
        back={{ to: `/athletes/${athleteId}`, label: athlete?.name ?? 'Athlete' }}
        actions={
          <Button variant={showAdd ? 'ghost' : 'default'} onClick={() => setShowAdd(!showAdd)} aria-expanded={showAdd}>
            {showAdd ? <><X aria-hidden="true" /> Cancel</> : <><Plus aria-hidden="true" /> Assign</>}
          </Button>
        }
      />
      {showAdd && (
        <form onSubmit={(e) => { e.preventDefault(); assignMutation.mutate() }}
          className="rounded-lg border border-border bg-card p-4 mb-6 flex flex-wrap gap-3 items-end">
          <div className="flex-1 min-w-50">
            <Label htmlFor="assign-exercise">Exercise <span aria-hidden="true" className="text-destructive">*</span></Label>
            <Select value={exerciseId || null} onValueChange={(val) => setExerciseId(val ?? "")} required>
              <SelectTrigger id="assign-exercise" className="w-full">
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
            <Label htmlFor="assign-reps">Target Reps</Label>
            <Input id="assign-reps" type="number" inputMode="numeric" min={0} value={targetReps} onChange={e => setTargetReps(e.target.value)} placeholder="Optional" />
          </div>
          <Button type="submit" disabled={assignMutation.isPending || !exerciseId}
            >
            Assign
          </Button>
        </form>
      )}
      <QueryState query={query} resource="assignments">
      {assignments => {
        const active = assignments.filter(a => a.active)
        const inactive = assignments.filter(a => !a.active)
        return (
        <>
      {active.length === 0 ? (
        <EmptyState
          icon="📋"
          title="No active exercise assignments"
          description="Assign exercises this athlete should work on outside a program."
          action={showAdd ? undefined : 'Assign an exercise'}
          onAction={() => setShowAdd(true)}
        />
      ) : (
        <div className="space-y-2">
          {active.map(a => (
            <Card key={a.id} size="sm">
              <CardContent className="flex items-center justify-between gap-3">
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
              }} className="text-muted-foreground hover:text-destructive">
                Deactivate
              </Button>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      {inactive.length > 0 && (
        <details className="mt-6">
          <summary className="text-sm text-muted-foreground cursor-pointer hover:text-foreground">
            {inactive.length} previously assigned exercise{inactive.length !== 1 ? 's' : ''}
          </summary>
          <div className="mt-2 space-y-1">
            {inactive.map(a => (
              <div key={a.id} className="flex items-center justify-between py-1 text-sm text-muted-foreground">
                <span>
                  {a.exercise_name}
                  {a.target_reps ? ` • ${a.target_reps} reps` : ''}
                  {a.deactivated_at ? ` • ended ${friendlyDate(a.deactivated_at, { timestamp: true })}` : ''}
                </span>
                <Button
                  variant="ghost"
                  size="sm"
                  disabled={reactivateMutation.isPending}
                  onClick={() => reactivateMutation.mutate({
                    exerciseId: a.exercise_id,
                    targetReps: a.target_reps ?? 0,
                  })}
                >
                  <RotateCcw aria-hidden="true" /> Reactivate
                </Button>
              </div>
            ))}
          </div>
        </details>
      )}
        </>
        )
      }}
      </QueryState>
      {confirmDialog()}
    </div>
  )
}