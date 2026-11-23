import { useState } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Check, Lock, Pencil, Plus, Trash2, X } from 'lucide-react'
import { api } from '@/api/client'
import type { ProgramTemplate } from '@/api/types'
import { EmptyState } from '@/components/ui'
import { PageHeader } from '@/components/PageHeader'
import { QueryState } from '@/components/QueryState'
import { Badge } from '@/components/ui/badge'
import { useConfirm } from '@/lib/useConfirm'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Card } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Checkbox } from '@/components/ui/checkbox'
import { dominantGroupKey, groupBlocks, groupKind, memberLabel } from '@/lib/groups'
import { GroupSelect } from '@/components/GroupSelect'
interface PrescribedSetData {
  id: number
  exercise_id: number
  exercise_name: string
  week: number
  day: number
  set_number: number
  reps: number | null
  percentage: number | null
  absolute_weight: number | null
  rest_seconds: number | null
  prescribed_rpe?: number | null
  group_label?: string | null
  rep_type: string
  notes: string | null
  sort_order?: number
  reps_max?: number | null
  rir_min?: number | null
  rir_max?: number | null
  non_working?: boolean
  group_key?: string | null
}
function formatSetInfo(s: PrescribedSetData): string {
  const parts: string[] = []
  if (s.non_working) parts.push('Warm-up')
  if (s.reps && s.reps_max && s.reps_max > s.reps) parts.push(`${s.reps}–${s.reps_max} reps`)
  else if (s.reps) parts.push(`${s.reps} reps`)
  else parts.push('AMRAP')
  if (s.percentage) parts.push(`@ ${s.percentage}%`)
  else if (s.absolute_weight) parts.push(`@ ${s.absolute_weight}`)
  if (s.rir_min != null && s.rir_max != null && s.rir_max !== s.rir_min) parts.push(`RIR ${s.rir_min}–${s.rir_max}`)
  else if (s.rir_min != null || s.rir_max != null) parts.push(`RIR ${s.rir_min ?? s.rir_max}`)
  if (s.prescribed_rpe != null) parts.push(`RPE ${s.prescribed_rpe}`)
  if (s.rest_seconds != null) parts.push(`${s.rest_seconds}s rest`)
  return parts.join(' ')
}

/**
 * Full update payload for a prescribed set: the PUT replaces the core
 * fields, so carry every current value and apply only the overrides.
 */
function setUpdatePayload(s: PrescribedSetData, overrides: Partial<Parameters<typeof api.updatePrescribedSet>[2]> = {}): Parameters<typeof api.updatePrescribedSet>[2] {
  return {
    exercise_id: s.exercise_id,
    set_number: s.set_number,
    reps: s.reps,
    percentage: s.percentage,
    absolute_weight: s.absolute_weight,
    rest_seconds: s.rest_seconds,
    sort_order: s.sort_order,
    rep_type: s.rep_type,
    notes: s.notes ?? '',
    prescribed_rpe: s.prescribed_rpe ?? null,
    group_label: s.group_label ?? undefined,
    ...overrides,
  }
}

/** Numeric input value → number, or null when blank. */
function numOrNull(v: string, parse: (s: string) => number = parseFloat): number | null {
  return v.trim() === '' ? null : parse(v)
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
  const [setRestSeconds, setSetRestSeconds] = useState('')
  const [setRepsMax, setSetRepsMax] = useState('')
  const [setRirMin, setSetRirMin] = useState('')
  const [setRirMax, setSetRirMax] = useState('')
  const [setWarmup, setSetWarmup] = useState(false)
  const [setGroup, setSetGroup] = useState<string | null>(null)
  const [showAddRule, setShowAddRule] = useState(false)
  const [ruleExId, setRuleExId] = useState('')
  const [ruleIncrement, setRuleIncrement] = useState('')
  const [copySourceWeek, setCopySourceWeek] = useState('')
  const [copyTargetWeek, setCopyTargetWeek] = useState('')
  const [editingSetId, setEditingSetId] = useState<number | null>(null)
  const [editReps, setEditReps] = useState('')
  const [editPercent, setEditPercent] = useState('')
  const [editAbsWeight, setEditAbsWeight] = useState('')
  const [editRestSeconds, setEditRestSeconds] = useState('')
  const [editNotes, setEditNotes] = useState('')
  const [editRepsMax, setEditRepsMax] = useState('')
  const [editRirMin, setEditRirMin] = useState('')
  const [editRirMax, setEditRirMax] = useState('')
  const [editWarmup, setEditWarmup] = useState(false)
  const [editGroup, setEditGroup] = useState<string | null>(null)
  const programQuery = useQuery({
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
      rest_seconds: setRestSeconds ? parseInt(setRestSeconds) : null,
      reps_max: numOrNull(setRepsMax, parseInt),
      rir_min: numOrNull(setRirMin, parseInt),
      rir_max: numOrNull(setRirMax, parseInt),
      non_working: setWarmup || undefined,
      group_key: setGroup ?? undefined,
    }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['program', programId] })
      setSetRepsMax('')
      setSetRirMin('')
      setSetRirMax('')
      setSetReps('')
      setSetPercent('')
      setSetAbsWeight('')
      setSetRestSeconds('')
    },
  })
  const deleteSetMutation = useMutation({
    mutationFn: (setId: number) => api.deletePrescribedSet(programId, setId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['program', programId] }),
  })
  const updateSetMutation = useMutation({
    mutationFn: (vars: { setId: number; data: PrescribedSetData }) => {
      const reps = editReps ? parseInt(editReps) : null
      // A half-filled RIR range is a single value; a negative on either
      // side would clear both, so mirror the filled side.
      const rirMin = numOrNull(editRirMin, parseInt)
      const rirMax = numOrNull(editRirMax, parseInt)
      return api.updatePrescribedSet(programId, vars.setId, setUpdatePayload(vars.data, {
        reps,
        percentage: numOrNull(editPercent),
        absolute_weight: numOrNull(editAbsWeight),
        rest_seconds: numOrNull(editRestSeconds, parseInt),
        notes: editNotes,
        // 0 clears the range; negative clears RIR; "" ungroups.
        reps_max: editRepsMax ? parseInt(editRepsMax) : 0,
        rir_min: rirMin ?? rirMax ?? -1,
        rir_max: rirMax ?? rirMin ?? -1,
        non_working: editWarmup,
        group_key: editGroup ?? '',
      }))
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['program', programId] })
      setEditingSetId(null)
    },
    onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to update set'),
  })
  // Put every set of one exercise on one day into a group (or ungroup it).
  const groupExerciseMutation = useMutation({
    mutationFn: async (vars: { sets: PrescribedSetData[]; key: string | null }) => {
      for (const set of vars.sets) {
        await api.updatePrescribedSet(programId, set.id, setUpdatePayload(set, { group_key: vars.key ?? '' }))
      }
      return vars
    },
    meta: { skipGlobalError: true },
    onSuccess: (vars) => {
      queryClient.invalidateQueries({ queryKey: ['program', programId] })
      const name = vars.sets[0]?.exercise_name ?? 'Exercise'
      toast.success(vars.key ? `${name} added to group ${vars.key}.` : `${name} ungrouped.`)
    },
    onError: (err) => {
      queryClient.invalidateQueries({ queryKey: ['program', programId] })
      toast.error(`Couldn't change the group: ${err instanceof Error ? err.message : 'something went wrong.'}`)
    },
  })

  function beginEditSet(s: PrescribedSetData) {
    setEditingSetId(s.id)
    setEditReps(s.reps == null ? '' : String(s.reps))
    setEditPercent(s.percentage == null ? '' : String(s.percentage))
    setEditAbsWeight(s.absolute_weight == null ? '' : String(s.absolute_weight))
    setEditRestSeconds(s.rest_seconds == null ? '' : String(s.rest_seconds))
    setEditNotes(s.notes ?? '')
    setEditRepsMax(s.reps_max == null ? '' : String(s.reps_max))
    setEditRirMin(s.rir_min == null ? '' : String(s.rir_min))
    setEditRirMax(s.rir_max == null ? '' : String(s.rir_max))
    setEditWarmup(!!s.non_working)
    setEditGroup(s.group_key ?? null)
  }
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
  const data = programQuery.data
  if (!data) {
    return (
      <div>
        <PageHeader title="Program" pageTitle={null} back={{ to: '/programs', label: 'Programs' }} />
        <QueryState
          query={programQuery}
          resource="program"
          skeleton="detail"
          notFound={<EmptyState icon="🔍" title="Program not found" action="All programs" actionTo="/programs" />}
        >
          {() => null}
        </QueryState>
      </div>
    )
  }
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
  // Render helper (not a component): one exercise with its sets. label is
  // the superset position (A1) or a lone group key.
  function renderExercise(exerciseName: string, exSets: PrescribedSetData[], label: string | undefined, scope: string) {
    const groupKey = dominantGroupKey(exSets)
    const selectId = `group-${scope}-${exSets[0]?.exercise_id}`
    return (
      <div key={exerciseName}>
        <div className="flex items-center justify-between gap-2">
          <p className="text-sm font-medium">
            {label && <span className="mr-1.5 rounded bg-primary/15 px-1.5 py-0.5 text-xs font-semibold text-primary">{label}</span>}
            {exerciseName}
          </p>
          {editing && (
            <div className="w-32 shrink-0">
              <Label htmlFor={selectId} className="sr-only">Superset group for {exerciseName}</Label>
              <GroupSelect
                id={selectId}
                value={groupKey}
                disabled={groupExerciseMutation.isPending}
                onChange={(key) => groupExerciseMutation.mutate({ sets: exSets, key })}
              />
            </div>
          )}
        </div>
        <div className="mt-1 space-y-0.5">
          {exSets.map(s => {
              if (editing && editingSetId === s.id) {
                return (
                  <div key={s.id} className="rounded-md border border-border bg-background p-2 my-1 space-y-2">
                    <div className="grid grid-cols-2 gap-2 md:grid-cols-4">
                      <div>
                        <Label htmlFor="prog-edit-reps" className="text-xs">Reps</Label>
                        <Input id="prog-edit-reps" type="number" min={1} value={editReps} onChange={e => setEditReps(e.target.value)} placeholder="AMRAP" />
                      </div>
                      <div>
                        <Label htmlFor="prog-edit-percent" className="text-xs">%</Label>
                        <Input id="prog-edit-percent" type="number" step="0.5" value={editPercent} onChange={e => setEditPercent(e.target.value)} />
                      </div>
                      <div>
                        <Label htmlFor="prog-edit-weight" className="text-xs">Weight</Label>
                        <Input id="prog-edit-weight" type="number" step="0.5" value={editAbsWeight} onChange={e => setEditAbsWeight(e.target.value)} />
                      </div>
                      <div>
                        <Label htmlFor="prog-edit-rest" className="text-xs">Rest (sec)</Label>
                        <Input id="prog-edit-rest" type="number" min={0} value={editRestSeconds} onChange={e => setEditRestSeconds(e.target.value)} />
                      </div>
                    </div>
                    <div className="grid grid-cols-2 gap-2 md:grid-cols-4">
                      <div>
                        <Label htmlFor="prog-edit-reps-max" className="text-xs">Reps max</Label>
                        <Input id="prog-edit-reps-max" type="number" min={1} value={editRepsMax} onChange={e => setEditRepsMax(e.target.value)} />
                      </div>
                      <div>
                        <Label htmlFor="prog-edit-rir-min" className="text-xs">RIR min</Label>
                        <Input id="prog-edit-rir-min" type="number" min={0} max={10} value={editRirMin} onChange={e => setEditRirMin(e.target.value)} />
                      </div>
                      <div>
                        <Label htmlFor="prog-edit-rir-max" className="text-xs">RIR max</Label>
                        <Input id="prog-edit-rir-max" type="number" min={0} max={10} value={editRirMax} onChange={e => setEditRirMax(e.target.value)} />
                      </div>
                      <div>
                        <Label htmlFor="prog-edit-group" className="text-xs">Group</Label>
                        <GroupSelect id="prog-edit-group" value={editGroup} onChange={setEditGroup} />
                      </div>
                    </div>
                    <label htmlFor="prog-edit-warmup" className="flex min-h-11 items-center gap-2 text-xs md:min-h-0">
                      <Checkbox id="prog-edit-warmup" checked={editWarmup} onCheckedChange={(c) => setEditWarmup(c === true)} />
                      Warm-up set
                    </label>
                    <div>
                      <Label htmlFor="prog-edit-notes" className="text-xs">Notes</Label>
                      <Input id="prog-edit-notes" type="text" value={editNotes} onChange={e => setEditNotes(e.target.value)} />
                    </div>
                    <div className="flex gap-2">
                      <Button
                        type="button"
                        size="sm"
                        disabled={updateSetMutation.isPending}
                        onClick={() => updateSetMutation.mutate({ setId: s.id, data: s })}
                      >
                        {updateSetMutation.isPending ? 'Saving…' : 'Save'}
                      </Button>
                      <Button type="button" size="sm" variant="ghost" onClick={() => setEditingSetId(null)}>
                        Cancel
                      </Button>
                    </div>
                  </div>
                )
              }
              return (
                <div key={s.id} className="flex items-center justify-between">
                  <p className="text-xs text-muted-foreground">
                    Set {s.set_number}: {formatSetInfo(s)}
                    {s.notes && ` — ${s.notes}`}
                  </p>
                  {editing && (
                    <div className="flex gap-1">
                      <Button variant="ghost" size="icon-sm" aria-label={`Edit set ${s.set_number}`} onClick={() => beginEditSet(s)}>
                        <Pencil aria-hidden="true" />
                      </Button>
                      <Button variant="ghost" size="icon-sm" aria-label={`Delete set ${s.set_number}`} className="hover:text-destructive" onClick={async () => {
                        if (await confirm({ title: 'Delete Set', description: 'Remove this prescribed set?', confirmLabel: 'Delete', variant: 'danger' }))
                          deleteSetMutation.mutate(s.id)
                      }}>
                        <X aria-hidden="true" />
                      </Button>
                    </div>
                  )}
                </div>
              )
            })}
        </div>
      </div>
    )
  }
  return (
    <div>
      <PageHeader
        title={program.name}
        back={{ to: '/programs', label: 'Programs' }}
        meta={program.locked && (
          <Badge variant="outline"><Lock aria-hidden="true" className="size-3" /> Locked · v{program.import_version ?? program.version}</Badge>
        )}
        description={
          <>
            {program.description && <p className="mb-1">{program.description}</p>}
            <p>
              {program.num_weeks} week{program.num_weeks !== 1 ? 's' : ''} · {program.num_days} day{program.num_days !== 1 ? 's' : ''}/week
              {program.is_loop && <> · <span className="text-primary">Looping</span></>}
            </p>
            {program.locked && <p className="mt-1">Imported templates are read-only. To change it, re-import a new version.</p>}
          </>
        }
        actions={!program.locked && (
          <>
            <Button variant={editing ? 'default' : 'outline'} onClick={() => { setEditing(!editing); setShowAddSet(false); setShowAddRule(false) }} aria-pressed={editing}>
              {editing ? <><Check aria-hidden="true" /> Done</> : <><Pencil aria-hidden="true" /> Edit</>}
            </Button>
            {editing && (
              <>
                <Button variant="outline" render={<Link to={`/programs/${programId}/edit`} />}>
                  Edit Details
                </Button>
                <Button variant="destructive" onClick={async () => {
                  if (await confirm({ title: 'Delete Program', description: `Delete ${program.name}? This will remove all prescribed sets.`, confirmLabel: 'Delete', variant: 'danger' }))
                    deleteProgramMutation.mutate()
                }}>
                  <Trash2 aria-hidden="true" /> Delete
                </Button>
              </>
            )}
          </>
        )}
      />
      {sets.length === 0 ? (
        <EmptyState
          icon="📊"
          title="No prescribed sets yet"
          description={program.locked ? undefined : 'Turn on editing to add the sets this program prescribes.'}
          action={!program.locked && !editing ? 'Start editing' : undefined}
          onAction={() => { setEditing(true); setShowAddSet(true) }}
        />
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
                      {groupBlocks(Array.from(exercises.entries()), ([, exSets]) => dominantGroupKey(exSets)).map((block, bi) => {
                        const grouped = block.key != null && block.items.length > 1
                        const content = block.items.map(([exerciseName, exSets], mi) => renderExercise(
                          exerciseName,
                          exSets,
                          grouped ? memberLabel(block.key!, mi) : block.key ?? undefined,
                          `w${week}-d${day}`,
                        ))
                        if (!grouped) return <div key={block.key ?? `solo-${bi}`}>{content}</div>
                        return (
                          <section
                            key={block.key}
                            aria-label={`${groupKind(block.items.length)} ${block.key}`}
                            className="rounded-md border-l-4 border-primary/60 bg-primary/5 py-2 pl-3 pr-2 space-y-3"
                          >
                            <p className="text-xs font-semibold uppercase tracking-wider text-primary">
                              {groupKind(block.items.length)} {block.key}
                              <span className="ml-1 font-normal normal-case tracking-normal text-muted-foreground">· alternate {block.items.map((_, i) => memberLabel(block.key!, i)).join(', ')}</span>
                            </p>
                            {content}
                          </section>
                        )
                      })}
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
            <Plus aria-hidden="true" /> Add Prescribed Set
          </Button>
        ) : (
          <form onSubmit={(e) => { e.preventDefault(); addSetMutation.mutate() }}
            className="rounded-lg border border-border bg-card p-4 space-y-3">
            <h3 className="text-sm font-medium">Add Prescribed Set</h3>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
              <div className="col-span-2">
                <Label htmlFor="prog-exercise">Exercise <span aria-hidden="true" className="text-destructive">*</span></Label>
                <Select value={setExId || null} onValueChange={(val) => setSetExId(val ?? "")} required>
                  <SelectTrigger id="prog-exercise" className="w-full">
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
                <Label htmlFor="prog-week">Week</Label>
                <Input id="prog-week" type="number" min={1} value={setWeek} onChange={e => setSetWeek(e.target.value)} />
              </div>
              <div>
                <Label htmlFor="prog-day">Day</Label>
                <Input id="prog-day" type="number" min={1} value={setDay} onChange={e => setSetDay(e.target.value)} />
              </div>
              <div>
                <Label htmlFor="prog-reps">Reps</Label>
                <Input id="prog-reps" type="number" min={1} value={setReps} onChange={e => setSetReps(e.target.value)} placeholder="empty=AMRAP" />
              </div>
              <div>
                <Label htmlFor="prog-percent">% of TM</Label>
                <Input id="prog-percent" type="number" step="0.5" value={setPercent} onChange={e => setSetPercent(e.target.value)} placeholder="e.g. 75" />
              </div>
              <div>
                <Label htmlFor="prog-abs-weight">Abs. Weight</Label>
                <Input id="prog-abs-weight" type="number" step="0.5" value={setAbsWeight} onChange={e => setSetAbsWeight(e.target.value)} />
              </div>
              <div>
                <Label htmlFor="prog-rest">Rest (sec)</Label>
                <Input id="prog-rest" type="number" min={0} value={setRestSeconds} onChange={e => setSetRestSeconds(e.target.value)} />
              </div>
              <div>
                <Label htmlFor="prog-reps-max">Reps (top of range)</Label>
                <Input id="prog-reps-max" type="number" min={1} value={setRepsMax} onChange={e => setSetRepsMax(e.target.value)} placeholder="e.g. 12" />
              </div>
              <div>
                <Label htmlFor="prog-rir-min">RIR min</Label>
                <Input id="prog-rir-min" type="number" min={0} max={10} value={setRirMin} onChange={e => setSetRirMin(e.target.value)} />
              </div>
              <div>
                <Label htmlFor="prog-rir-max">RIR max</Label>
                <Input id="prog-rir-max" type="number" min={0} max={10} value={setRirMax} onChange={e => setSetRirMax(e.target.value)} />
              </div>
              <div>
                <Label htmlFor="prog-group">Superset group</Label>
                <GroupSelect id="prog-group" value={setGroup} onChange={setSetGroup} />
              </div>
            </div>
            <label htmlFor="prog-warmup" className="flex min-h-11 items-center gap-2 text-sm md:min-h-0">
              <Checkbox id="prog-warmup" checked={setWarmup} onCheckedChange={(c) => setSetWarmup(c === true)} />
              Warm-up set (not counted toward progress or PRs)
            </label>
            <p className="text-xs text-muted-foreground">Exercises that share a group letter on the same day are done as a superset (two) or circuit (three or more), alternating A1, A2 each round.</p>
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
              <Label htmlFor="prog-copy-from-week">Copy from week</Label>
              <Select value={copySourceWeek || null} onValueChange={(val) => setCopySourceWeek(val ?? '')}>
                <SelectTrigger id="prog-copy-from-week" className="w-32">
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
              <Label htmlFor="prog-to-week">To week</Label>
              <Select value={copyTargetWeek || null} onValueChange={(val) => setCopyTargetWeek(val ?? '')}>
                <SelectTrigger id="prog-to-week" className="w-32">
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
              {showAddRule ? <><X aria-hidden="true" /> Cancel</> : <><Plus aria-hidden="true" /> Add Rule</>}
            </Button>
          )}
        </div>
        {editing && showAddRule && (
          <form onSubmit={(e) => { e.preventDefault(); addRuleMutation.mutate() }}
            className="rounded-lg border border-border bg-card p-4 mb-3 flex flex-wrap gap-3 items-end">
            <div className="flex-1 min-w-50">
              <Label htmlFor="prog-rule-exercise">Exercise <span aria-hidden="true" className="text-destructive">*</span></Label>
              <Select value={ruleExId || null} onValueChange={(val) => setRuleExId(val ?? "")} required>
                <SelectTrigger id="prog-rule-exercise" className="w-full">
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
              <Label htmlFor="prog-increment">Increment <span aria-hidden="true" className="text-destructive">*</span></Label>
              <Input id="prog-increment" type="number" step="0.5" min={0.5} value={ruleIncrement} onChange={e => setRuleIncrement(e.target.value)} required placeholder="5" />
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
                {editing && <TableHead className="w-12"><span className="sr-only">Actions</span></TableHead>}
              </TableRow>
            </TableHeader>
            <TableBody>
              {rules.map(rule => (
                <TableRow key={rule.id}>
                  <TableCell className="font-medium">{rule.exercise_name}</TableCell>
                  <TableCell className="text-muted-foreground">+{rule.increment} per cycle</TableCell>
                  {editing && (
                    <TableCell>
                      <Button variant="ghost" size="icon-sm" aria-label={`Delete ${rule.exercise_name} progression rule`} className="hover:text-destructive" onClick={async () => {
                        if (await confirm({ title: 'Delete Rule', description: 'Remove this progression rule?', confirmLabel: 'Delete', variant: 'danger' }))
                          deleteRuleMutation.mutate(rule.id)
                      }}>
                        <X aria-hidden="true" />
                      </Button>
                    </TableCell>
                  )}
                </TableRow>
              ))}
            </TableBody>
          </Table>
        ) : (
          <p className="text-sm text-muted-foreground">No progression rules. Training maxes change only when the coach applies a change.</p>
        )}
      </div>
      {confirmDialog()}
    </div>
  )
}