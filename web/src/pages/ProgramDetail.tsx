import { useState } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { ProgramTemplate } from '@/api/types'
import { Spinner } from '@/components/ui'
import { useConfirm } from '@/lib/useConfirm'

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

  const { data, isLoading, error } = useQuery({
    queryKey: ['program', programId],
    queryFn: () => api.getProgramTemplate(programId),
    enabled: !isNaN(programId),
  })

  const { data: exercises } = useQuery({
    queryKey: ['exercises'],
    queryFn: () => api.listExercises(),
    enabled: showAddSet || showAddRule,
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
          <Link to={`/programs/${programId}/edit`}
            className="rounded-md border border-border px-3 py-1.5 text-sm hover:bg-accent transition-colors">
            ✏️ Edit
          </Link>
          <button onClick={async () => {
            if (await confirm({ title: 'Delete Program', description: `Delete ${program.name}? This will remove all prescribed sets.`, confirmLabel: 'Delete', variant: 'danger' }))
              deleteProgramMutation.mutate()
          }} className="text-sm text-destructive hover:text-destructive/80">
            Delete
          </button>
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
                  <div key={day} className="rounded-lg border border-border bg-card p-4">
                    <h3 className="text-sm font-medium text-muted-foreground mb-3">Day {day}</h3>
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
                                <button onClick={() => deleteSetMutation.mutate(s.id)}
                                  className="text-xs text-destructive hover:text-destructive/80 ml-2">×</button>
                              </div>
                            ))}
                          </div>
                        </div>
                      ))}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Add Set */}
      <div className="mt-6">
        {!showAddSet ? (
          <button onClick={() => setShowAddSet(true)}
            className="rounded-md border border-dashed border-border px-4 py-2 text-sm text-muted-foreground hover:border-primary/50 hover:text-foreground transition-colors w-full">
            + Add Prescribed Set
          </button>
        ) : (
          <form onSubmit={(e) => { e.preventDefault(); addSetMutation.mutate() }}
            className="rounded-lg border border-border bg-card p-4 space-y-3">
            <h3 className="text-sm font-medium">Add Prescribed Set</h3>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
              <div className="col-span-2">
                <label className="block text-xs text-muted-foreground mb-1">Exercise</label>
                <select value={setExId} onChange={e => setSetExId(e.target.value)} required
                  className="w-full rounded-md border border-border bg-background px-3 py-1.5 text-sm">
                  <option value="">Select...</option>
                  {exercises?.map(ex => <option key={ex.id} value={ex.id}>{ex.name}</option>)}
                </select>
              </div>
              <div>
                <label className="block text-xs text-muted-foreground mb-1">Week</label>
                <input type="number" min={1} value={setWeek} onChange={e => setSetWeek(e.target.value)}
                  className="w-full rounded-md border border-border bg-background px-3 py-1.5 text-sm" />
              </div>
              <div>
                <label className="block text-xs text-muted-foreground mb-1">Day</label>
                <input type="number" min={1} value={setDay} onChange={e => setSetDay(e.target.value)}
                  className="w-full rounded-md border border-border bg-background px-3 py-1.5 text-sm" />
              </div>
              <div>
                <label className="block text-xs text-muted-foreground mb-1">Reps</label>
                <input type="number" min={1} value={setReps} onChange={e => setSetReps(e.target.value)}
                  placeholder="empty=AMRAP"
                  className="w-full rounded-md border border-border bg-background px-3 py-1.5 text-sm" />
              </div>
              <div>
                <label className="block text-xs text-muted-foreground mb-1">% of TM</label>
                <input type="number" step="0.5" value={setPercent} onChange={e => setSetPercent(e.target.value)}
                  placeholder="e.g. 75"
                  className="w-full rounded-md border border-border bg-background px-3 py-1.5 text-sm" />
              </div>
              <div>
                <label className="block text-xs text-muted-foreground mb-1">Abs. Weight</label>
                <input type="number" step="0.5" value={setAbsWeight} onChange={e => setSetAbsWeight(e.target.value)}
                  className="w-full rounded-md border border-border bg-background px-3 py-1.5 text-sm" />
              </div>
            </div>
            <div className="flex gap-2">
              <button type="submit" disabled={addSetMutation.isPending || !setExId}
                className="rounded-md bg-primary px-4 py-1.5 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50">
                Add Set
              </button>
              <button type="button" onClick={() => setShowAddSet(false)}
                className="rounded-md border border-border px-4 py-1.5 text-sm hover:bg-accent">Cancel</button>
            </div>
          </form>
        )}
      </div>

      {/* Progression Rules */}
      <div className="mt-8">
        <div className="flex items-center justify-between mb-3">
          <h2 className="text-lg font-semibold">Progression Rules</h2>
          <button onClick={() => setShowAddRule(!showAddRule)}
            className="text-sm text-primary hover:text-primary/80">
            {showAddRule ? 'Cancel' : '+ Add Rule'}
          </button>
        </div>

        {showAddRule && (
          <form onSubmit={(e) => { e.preventDefault(); addRuleMutation.mutate() }}
            className="rounded-lg border border-border bg-card p-4 mb-3 flex flex-wrap gap-3 items-end">
            <div className="flex-1 min-w-50">
              <label className="block text-xs text-muted-foreground mb-1">Exercise</label>
              <select value={ruleExId} onChange={e => setRuleExId(e.target.value)} required
                className="w-full rounded-md border border-border bg-background px-3 py-1.5 text-sm">
                <option value="">Select...</option>
                {exercises?.map(ex => <option key={ex.id} value={ex.id}>{ex.name}</option>)}
              </select>
            </div>
            <div className="w-24">
              <label className="block text-xs text-muted-foreground mb-1">Increment</label>
              <input type="number" step="0.5" min={0.5} value={ruleIncrement} onChange={e => setRuleIncrement(e.target.value)}
                required placeholder="5"
                className="w-full rounded-md border border-border bg-background px-3 py-1.5 text-sm" />
            </div>
            <button type="submit" disabled={addRuleMutation.isPending || !ruleExId || !ruleIncrement}
              className="rounded-md bg-primary px-4 py-1.5 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50">
              Add
            </button>
          </form>
        )}

        {rules && rules.length > 0 ? (
          <div className="space-y-2">
            {rules.map(rule => (
              <div key={rule.id} className="flex items-center justify-between rounded-lg border border-border bg-card p-3">
                <div>
                  <p className="text-sm font-medium">{rule.exercise_name}</p>
                  <p className="text-xs text-muted-foreground">+{rule.increment} per cycle</p>
                </div>
                <button onClick={() => deleteRuleMutation.mutate(rule.id)}
                  className="text-xs text-destructive hover:text-destructive/80">×</button>
              </div>
            ))}
          </div>
        ) : (
          <p className="text-sm text-muted-foreground">No progression rules defined.</p>
        )}
      </div>

      {confirmDialog()}
    </div>
  )
}
