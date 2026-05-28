import { useState } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { ProgramTemplate } from '@/api/types'
import { Spinner } from '@/components/ui'
import { useConfirm } from '@/lib/useConfirm'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Card } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
interface PrescribedSetData {
  id: number
  exercise_name: string
  week: number
  day: number
  set_number: number
  reps: number | null
  percentage: number | null
  absolute_weight: number | null
  rep_type: string
  notes: string | null
}
function formatSetInfo(s: PrescribedSetData): string {
  const parts: string[] = []
  if (s.reps) parts.push(`${s.reps} reps`)
  else parts.push('AMRAP')
  if (s.percentage) parts.push(`@ ${s.percentage}%`)
  else if (s.absolute_weight) parts.push(`@ ${s.absolute_weight}`)
  return parts.join(' ')
}
export function ProgramDetail() {
  const { id } = useParams<{ id: string }>()
  const programId = Number(id)
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { confirm, dialog: confirmDialog } = useConfirm()
  const [editing, setEditing] = useState(false)
  const [showAddSet, setShowAddSet] = useState(false)
  const [setExId, setSetExId] = useState('')
  const [setWeek, setSetWeek] = useState('1')
  const [setDay, setSetDay] = useState('1')
  const [setReps, setSetReps] = useState('')
  const [setPercent, setSetPercent] = useState('')
  const [setAbsWeight, setSetAbsWeight] = useState('')
  const [showAddRule, setShowAddRule] = useState(false)
  const [ruleExId, setRuleExId] = useState('')
  const [ruleIncrement, setRuleIncrement] = useState('')
  const [copySourceWeek, setCopySourceWeek] = useState('')
  const [copyTargetWeek, setCopyTargetWeek] = useState('')
  const { data, isLoading, error } = useQuery({
    queryKey: ['program', programId],
    queryFn: () => api.getProgramTemplate(programId),
    enabled: !isNaN(programId),
  })
  const { data: exercises } = useQuery({
    queryKey: ['exercises'],
    queryFn: () => api.listExercises(),
    enabled: editing,
  })
  const { data: rules } = useQuery({
    queryKey: ['program-rules', programId],
    queryFn: () => api.listProgressionRules(programId),
    enabled: !isNaN(programId),
  })
  const deleteProgramMutation = useMutation({
    mutationFn: () => api.deleteProgramTemplate(programId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['programs'] })
      navigate('/programs')
    },
  })
  const addSetMutation = useMutation({
    mutationFn: () => api.addPrescribedSet(programId, {
      exercise_id: parseInt(setExId),
      week: parseInt(setWeek),
      day: parseInt(setDay),
      set_number: 1,
      reps: setReps ? parseInt(setReps) : null,
      percentage: setPercent ? parseFloat(setPercent) : null,
      absolute_weight: setAbsWeight ? parseFloat(setAbsWeight) : null,
    }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['program', programId] })
      setSetReps('')
      setSetPercent('')
      setSetAbsWeight('')
    },
  })
  const deleteSetMutation = useMutation({
    mutationFn: (setId: number) => api.deletePrescribedSet(programId, setId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['program', programId] }),
  })
  const addRuleMutation = useMutation({
    mutationFn: () => api.setProgressionRule(programId, parseInt(ruleExId), parseFloat(ruleIncrement)),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['program-rules', programId] })
      setRuleExId('')
      setRuleIncrement('')
      setShowAddRule(false)
    },
  })
  const deleteRuleMutation = useMutation({
    mutationFn: (ruleId: number) => api.deleteProgressionRule(programId, ruleId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['program-rules', programId] }),
  })
  const copyWeekMutation = useMutation({
    mutationFn: () => api.copyWeek(programId, parseInt(copySourceWeek), parseInt(copyTargetWeek)),
    onSuccess: (res) => {
      queryClient.invalidateQueries({ queryKey: ['program', programId] })
      toast.success(`Copied ${res.sets_copied} set${res.sets_copied !== 1 ? 's' : ''} to week ${copyTargetWeek}`)
      setCopySourceWeek('')
      setCopyTargetWeek('')
    },
    onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to copy week'),
  })
  if (isLoading) return <Spinner />
  if (error) return <p className="text-destructive">Failed to load program.</p>
  if (!data) return <p className="text-muted-foreground">Program not found.</p>
  const program = data.program as ProgramTemplate
  const sets = data.sets as PrescribedSetData[]
  // Group sets by week → day → exercise
  const weeks = new Map<number, Map<number, Map<string, PrescribedSetData[]>>>()
  for (const s of sets) {
    if (!weeks.has(s.week)) weeks.set(s.week, new Map())
    const days = weeks.get(s.week)!
    if (!days.has(s.day)) days.set(s.day, new Map())
    const exercises = days.get(s.day)!
    const key = s.exercise_name
    if (!exercises.has(key)) exercises.set(key, [])
    exercises.get(key)!.push(s)
  }
  return (
    <div>
      <p className="text-sm text-muted-foreground mb-1">
        <Link to="/programs" className="hover:text-foreground">Programs</Link>
        {' / '}
        {program.name}
      </p>
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold mb-2">{program.name}</h1>
        <div className="flex gap-2">
          <Button variant={editing ? 'default' : 'outline'} size="sm" onClick={() => { setEditing(!editing); setShowAddSet(false); setShowAddRule(false) }}>
            {editing ? '✓ Done' : '✏️ Edit'}
          </Button>
          {editing && (
            <>
              <Link to={`/programs/${programId}/edit`}
                className="rounded-md border border-border px-3 py-1.5 text-sm hover:bg-accent transition-colors">
                Edit Details
              </Link>
              <Button variant="ghost" size="sm" onClick={async () => {
                if (await confirm({ title: 'Delete Program', description: `Delete ${program.name}? This will remove all prescribed sets.`, confirmLabel: 'Delete', variant: 'danger' }))
                  deleteProgramMutation.mutate()
              }} className="text-destructive">
                Delete
              </Button>
            </>
          )}
        </div>
      </div>
      {program.description && (
        <p className="text-muted-foreground mb-4">{program.description}</p>
      )}
      <div className="flex gap-3 mb-6 text-sm text-muted-foreground">
        <span>{program.num_weeks} week{program.num_weeks !== 1 ? 's' : ''}</span>
        <span>•</span>
        <span>{program.num_days} day{program.num_days !== 1 ? 's' : ''}/week</span>
        {program.is_loop && <><span>•</span><span className="text-primary">Looping</span></>}
      </div>
      {sets.length === 0 ? (
        <p className="text-muted-foreground">No prescribed sets defined.</p>
      ) : (
        <div className="space-y-8">
          {Array.from(weeks.entries()).sort((a, b) => a[0] - b[0]).map(([week, days]) => (
            <div key={week}>
              <h2 className="text-lg font-semibold mb-3">Week {week}</h2>
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                {Array.from(days.entries()).sort((a, b) => a[0] - b[0]).map(([day, exercises]) => (
                  <Card key={day} size="sm" className="p-3 gap-2">
                    <h3 className="text-sm font-semibold text-muted-foreground px-0">Day {day}</h3>
                    <div className="space-y-3">
                      {Array.from(exercises.entries()).map(([exerciseName, exSets]) => (
                        <div key={exerciseName}>
                          <p className="text-sm font-medium">{exerciseName}</p>
                          <div className="mt-1 space-y-0.5">
                            {exSets.map(s => (
                              <div key={s.id} className="flex items-center justify-between">
                                <p className="text-xs text-muted-foreground">
                                  Set {s.set_number}: {formatSetInfo(s)}
                                  {s.notes && ` — ${s.notes}`}
                                </p>
                                {editing && (
                                  <Button variant="ghost" size="xs" onClick={async () => {
                                    if (await confirm({ title: 'Delete Set', description: 'Remove this prescribed set?', confirmLabel: 'Delete', variant: 'danger' }))
                                      deleteSetMutation.mutate(s.id)
                                  }}>×</Button>
                                )}
                              </div>
                            ))}
                          </div>
                        </div>
                      ))}
                    </div>
                  </Card>
                ))}
              </div>
            </div>
          ))}
        </div>
      )}
      {/* Add Set */}
      {editing && (
      <div className="mt-6">
        {!showAddSet ? (
          <Button variant="ghost" onClick={() => setShowAddSet(true)}
            className="rounded-md border border-dashed border-border px-4 py-2 text-sm text-muted-foreground hover:border-primary/50 hover:text-foreground transition-colors w-full">
            + Add Prescribed Set
          </Button>
        ) : (
          <form onSubmit={(e) => { e.preventDefault(); addSetMutation.mutate() }}
            className="rounded-lg border border-border bg-card p-4 space-y-3">
            <h3 className="text-sm font-medium">Add Prescribed Set</h3>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
              <div className="col-span-2">
                <Label >Exercise</Label>
                <Select value={setExId || null} onValueChange={(val) => setSetExId(val ?? "")} required>
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
              <div>
                <Label >Week</Label>
                <Input type="number" min={1} value={setWeek} onChange={e => setSetWeek(e.target.value)} />
              </div>
              <div>
                <Label >Day</Label>
                <Input type="number" min={1} value={setDay} onChange={e => setSetDay(e.target.value)} />
              </div>
              <div>
                <Label >Reps</Label>
                <Input type="number" min={1} value={setReps} onChange={e => setSetReps(e.target.value)} placeholder="empty=AMRAP" />
              </div>
              <div>
                <Label >% of TM</Label>
                <Input type="number" step="0.5" value={setPercent} onChange={e => setSetPercent(e.target.value)} placeholder="e.g. 75" />
              </div>
              <div>
                <Label >Abs. Weight</Label>
                <Input type="number" step="0.5" value={setAbsWeight} onChange={e => setSetAbsWeight(e.target.value)} />
              </div>
            </div>
            <div className="flex gap-2">
              <Button type="submit" disabled={addSetMutation.isPending || !setExId}
                >
                Add Set
              </Button>
              <Button variant="ghost" type="button" onClick={() => setShowAddSet(false)}
                >Cancel</Button>
            </div>
          </form>
        )}
      </div>
      )}
      {/* Copy Week */}
      {editing && program.num_weeks > 1 && (
        <div className="mt-6">
          <form
            onSubmit={async (e) => {
              e.preventDefault()
              if (!copySourceWeek || !copyTargetWeek) return
              const target = parseInt(copyTargetWeek)
              const targetHasSets = sets.some(s => s.week === target)
              if (targetHasSets) {
                const ok = await confirm({
                  title: 'Target Week Has Sets',
                  description: `Week ${target} already has prescribed sets. The copied sets will be added alongside them.`,
                  confirmLabel: 'Copy anyway',
                })
                if (!ok) return
              }
              copyWeekMutation.mutate()
            }}
            className="rounded-lg border border-border bg-card p-4 flex flex-wrap gap-3 items-end"
          >
            <div>
              <Label>Copy from week</Label>
              <Select value={copySourceWeek || null} onValueChange={(val) => setCopySourceWeek(val ?? '')}>
                <SelectTrigger className="w-32">
                  <SelectValue placeholder="Source" />
                </SelectTrigger>
                <SelectContent>
                  {Array.from({ length: program.num_weeks }, (_, i) => i + 1).map(w => (
                    <SelectItem key={w} value={String(w)}>Week {w}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div>
              <Label>To week</Label>
              <Select value={copyTargetWeek || null} onValueChange={(val) => setCopyTargetWeek(val ?? '')}>
                <SelectTrigger className="w-32">
                  <SelectValue placeholder="Target" />
                </SelectTrigger>
                <SelectContent>
                  {Array.from({ length: program.num_weeks }, (_, i) => i + 1)
                    .filter(w => String(w) !== copySourceWeek)
                    .map(w => (
                      <SelectItem key={w} value={String(w)}>Week {w}</SelectItem>
                    ))}
                </SelectContent>
              </Select>
            </div>
            <Button
              type="submit"
              variant="outline"
              disabled={!copySourceWeek || !copyTargetWeek || copyWeekMutation.isPending}
            >
              {copyWeekMutation.isPending ? 'Copying...' : 'Copy week'}
            </Button>
          </form>
        </div>
      )}
      {/* Progression Rules */}
      <div className="mt-8">
        <div className="flex items-center justify-between mb-3">
          <h2 className="text-lg font-semibold">Progression Rules</h2>
          {editing && (
            <Button variant="ghost" onClick={() => setShowAddRule(!showAddRule)}
              className="text-sm text-primary hover:text-primary/80">
              {showAddRule ? 'Cancel' : '+ Add Rule'}
            </Button>
          )}
        </div>
        {editing && showAddRule && (
          <form onSubmit={(e) => { e.preventDefault(); addRuleMutation.mutate() }}
            className="rounded-lg border border-border bg-card p-4 mb-3 flex flex-wrap gap-3 items-end">
            <div className="flex-1 min-w-50">
              <Label >Exercise</Label>
              <Select value={ruleExId || null} onValueChange={(val) => setRuleExId(val ?? "")} required>
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
            <div className="w-24">
              <Label >Increment</Label>
              <Input type="number" step="0.5" min={0.5} value={ruleIncrement} onChange={e => setRuleIncrement(e.target.value)} required placeholder="5" />
            </div>
            <Button type="submit" disabled={addRuleMutation.isPending || !ruleExId || !ruleIncrement}
              >
              Add
            </Button>
          </form>
        )}
        {rules && rules.length > 0 ? (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Exercise</TableHead>
                <TableHead>Increment</TableHead>
                {editing && <TableHead className="w-12"></TableHead>}
              </TableRow>
            </TableHeader>
            <TableBody>
              {rules.map(rule => (
                <TableRow key={rule.id}>
                  <TableCell className="font-medium">{rule.exercise_name}</TableCell>
                  <TableCell className="text-muted-foreground">+{rule.increment} per cycle</TableCell>
                  {editing && (
                    <TableCell>
                      <Button variant="ghost" size="xs" onClick={async () => {
                        if (await confirm({ title: 'Delete Rule', description: 'Remove this progression rule?', confirmLabel: 'Delete', variant: 'danger' }))
                          deleteRuleMutation.mutate(rule.id)
                      }}>×</Button>
                    </TableCell>
                  )}
                </TableRow>
              ))}
            </TableBody>
          </Table>
        ) : (
          <p className="text-sm text-muted-foreground">No progression rules defined.</p>
        )}
      </div>
      {confirmDialog()}
    </div>
  )
}