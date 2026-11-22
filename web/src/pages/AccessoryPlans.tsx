import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { ArchiveX, Pencil, Plus, Trash2, X } from 'lucide-react'
import { api } from '@/api/client'
import type { AccessoryPlanData } from '@/api/types'
import { EmptyState } from '@/components/ui'
import { PageHeader } from '@/components/PageHeader'
import { QueryState } from '@/components/QueryState'
import { useConfirm } from '@/lib/useConfirm'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Card, CardContent } from '@/components/ui/card'
import { GroupSelect } from '@/components/GroupSelect'
import { groupBlocks, groupKind, memberLabel } from '@/lib/groups'
export function AccessoryPlans() {
  const { id } = useParams<{ id: string }>()
  const athleteId = Number(id)
  const { confirm, dialog: confirmDialog } = useConfirm()
  const queryClient = useQueryClient()
  const [showForm, setShowForm] = useState(false)
  const [day, setDay] = useState('1')
  const [exerciseId, setExerciseId] = useState('')
  const [sets, setSets] = useState('')
  const [repMin, setRepMin] = useState('')
  const [repMax, setRepMax] = useState('')
  const [weight, setWeight] = useState('')
  const [notes, setNotes] = useState('')
  const [group, setGroup] = useState<string | null>(null)
  const { data: athlete } = useQuery({
    queryKey: ['athlete', athleteId],
    queryFn: () => api.getAthlete(athleteId),
    enabled: !isNaN(athleteId),
  })
  const query = useQuery({
    queryKey: ['accessories', athleteId],
    queryFn: () => api.listAccessoryPlans(athleteId),
    enabled: !isNaN(athleteId),
  })
  const { data: exercises } = useQuery({
    queryKey: ['exercises'],
    queryFn: () => api.listExercises(),
    enabled: showForm,
  })
  const createMutation = useMutation({
    mutationFn: () => api.createAccessoryPlan(athleteId, {
      day: parseInt(day),
      exercise_id: parseInt(exerciseId),
      target_sets: sets ? parseInt(sets) : undefined,
      target_rep_min: repMin ? parseInt(repMin) : undefined,
      target_rep_max: repMax ? parseInt(repMax) : undefined,
      target_weight: weight ? parseFloat(weight) : undefined,
      notes: notes || undefined,
      group_key: group ?? undefined,
    }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['accessories', athleteId] })
      setExerciseId('')
      setSets('')
      setRepMin('')
      setRepMax('')
      setWeight('')
      setNotes('')
      setGroup(null)
      setShowForm(false)
    },
  })
  const deleteMutation = useMutation({
    mutationFn: (planId: number) => api.deleteAccessoryPlan(athleteId, planId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['accessories', athleteId] }),
  })
  const deactivateMutation = useMutation({
    mutationFn: (planId: number) => api.deactivateAccessoryPlan(athleteId, planId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['accessories', athleteId] }),
  })
  return (
    <div>
      <PageHeader
        title="Accessory Plans"
        back={{ to: `/athletes/${athleteId}`, label: athlete?.name ?? 'Athlete' }}
        actions={
          <Button variant={showForm ? 'ghost' : 'default'} onClick={() => setShowForm(!showForm)} aria-expanded={showForm}>
            {showForm ? <><X aria-hidden="true" /> Cancel</> : <><Plus aria-hidden="true" /> Add</>}
          </Button>
        }
      />
      {showForm && (
        <form onSubmit={(e) => { e.preventDefault(); createMutation.mutate() }}
          className="rounded-lg border border-border bg-card p-4 mb-6 space-y-3">
          <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
            <div>
              <Label htmlFor="acc-day">Day</Label>
              <Select value={day} onValueChange={(val) => setDay(val ?? "1")}>
                <SelectTrigger id="acc-day" className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {[1,2,3,4,5,6,7].map(d => (
                    <SelectItem key={d} value={String(d)}>Day {d}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="col-span-2 md:col-span-3">
              <Label htmlFor="acc-exercise">Exercise <span aria-hidden="true" className="text-destructive">*</span></Label>
              <Select value={exerciseId || null} onValueChange={(val) => setExerciseId(val ?? "")} required>
                <SelectTrigger id="acc-exercise" className="w-full">
                  <SelectValue placeholder="Select exercise...">
                    {(value: string | null) => {
                      if (!value) return 'Select exercise...'
                      return exercises?.find(ex => String(ex.id) === value)?.name ?? value
                    }}
                  </SelectValue>
                </SelectTrigger>
                <SelectContent>
                  {exercises?.map(ex => (
                    <SelectItem key={ex.id} value={String(ex.id)}>{ex.name}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div>
              <Label htmlFor="acc-sets">Sets</Label>
              <Input id="acc-sets" type="number" value={sets} onChange={e => setSets(e.target.value)} min={1} />
            </div>
            <div>
              <Label htmlFor="acc-min-reps">Min Reps</Label>
              <Input id="acc-min-reps" type="number" value={repMin} onChange={e => setRepMin(e.target.value)} min={1} />
            </div>
            <div>
              <Label htmlFor="acc-max-reps">Max Reps</Label>
              <Input id="acc-max-reps" type="number" value={repMax} onChange={e => setRepMax(e.target.value)} min={1} />
            </div>
            <div>
              <Label htmlFor="acc-weight">Weight</Label>
              <Input id="acc-weight" type="number" step="0.5" value={weight} onChange={e => setWeight(e.target.value)} />
            </div>
            <div>
              <Label htmlFor="acc-group">Superset group</Label>
              <GroupSelect id="acc-group" value={group} onChange={setGroup} />
            </div>
          </div>
          <p className="text-xs text-muted-foreground">Accessories on the same day with the same group letter are done together as a superset or circuit.</p>
          <div>
            <Label htmlFor="acc-notes">Notes</Label>
            <Input id="acc-notes" type="text" value={notes} onChange={e => setNotes(e.target.value)} placeholder="Optional" />
          </div>
          <Button type="submit" disabled={createMutation.isPending || !exerciseId}
            >
            Add Accessory
          </Button>
        </form>
      )}
      <QueryState
        query={query}
        resource="accessory plans"
        empty={{ icon: '🧩', title: 'No accessory plans yet', description: 'Add accessories to show up on the athlete\'s training days.', action: showForm ? undefined : 'Add accessory', onAction: () => setShowForm(true) }}
      >
        {plans => {
          // Group plans by day (active only — inactive split out into history)
          const activePlans = plans.filter(p => p.active)
          const inactivePlans = plans.filter(p => !p.active)
          const byDay = new Map<number, AccessoryPlanData[]>()
          for (const p of activePlans) {
            if (!byDay.has(p.day)) byDay.set(p.day, [])
            byDay.get(p.day)!.push(p)
          }
          return (
        <>
          {activePlans.length === 0 && (
            <EmptyState icon="🧩" title="No active accessory plans" />
          )}
          <div className="space-y-6">
            {Array.from(byDay.entries()).sort((a, b) => a[0] - b[0]).map(([dayNum, dayPlans]) => (
              <div key={dayNum}>
                <h2 className="text-sm font-semibold text-muted-foreground mb-2">Day {dayNum}</h2>
                <div className="space-y-2">
                  {groupBlocks(dayPlans, p => p.group_key).map((block, bi) => {
                    const grouped = block.key != null && block.items.length > 1
                    const rows = block.items.map((plan, mi) => (
                    <AccessoryPlanRow
                      key={plan.id}
                      plan={plan}
                      label={grouped ? memberLabel(block.key!, mi) : block.key ?? undefined}
                      athleteId={athleteId}
                      onDeactivate={async () => {
                        if (await confirm({ title: 'Deactivate Plan', description: 'Mark this accessory plan as inactive? It will be hidden but kept in history.', confirmLabel: 'Deactivate' }))
                          deactivateMutation.mutate(plan.id)
                      }}
                      onDelete={async () => {
                        if (await confirm({ title: 'Delete Accessory', description: 'Permanently remove this accessory plan?', confirmLabel: 'Delete', variant: 'danger' }))
                          deleteMutation.mutate(plan.id)
                      }}
                    />
                    ))
                    if (!grouped) return <div key={block.key ?? `solo-${bi}`} className="space-y-2">{rows}</div>
                    return (
                      <section
                        key={block.key}
                        aria-label={`${groupKind(block.items.length)} ${block.key}`}
                        className="space-y-2 rounded-lg border-l-4 border-primary/60 bg-primary/5 p-2"
                      >
                        <p className="px-1 text-xs font-semibold uppercase tracking-wider text-primary">
                          {groupKind(block.items.length)} {block.key}
                        </p>
                        {rows}
                      </section>
                    )
                  })}
                </div>
              </div>
            ))}
          </div>
          {inactivePlans.length > 0 && (
            <details className="mt-6">
              <summary className="text-sm text-muted-foreground cursor-pointer hover:text-foreground">
                {inactivePlans.length} inactive plan{inactivePlans.length !== 1 ? 's' : ''}
              </summary>
              <div className="mt-2 space-y-1">
                {inactivePlans.map(plan => (
                  <div key={plan.id} className="flex items-center justify-between py-1 text-sm text-muted-foreground">
                    <span>
                      Day {plan.day} • {plan.exercise_name}
                      {plan.target_sets ? ` • ${plan.target_sets}×` : ''}
                      {plan.target_rep_min && plan.target_rep_max ? `${plan.target_rep_min}-${plan.target_rep_max}` : ''}
                    </span>
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      aria-label={`Delete inactive ${plan.exercise_name} plan`}
                      className="hover:text-destructive"
                      onClick={async () => {
                        if (await confirm({ title: 'Delete Plan', description: 'Permanently remove this inactive plan from history?', confirmLabel: 'Delete', variant: 'danger' }))
                          deleteMutation.mutate(plan.id)
                      }}
                    >
                      <Trash2 aria-hidden="true" />
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

function AccessoryPlanRow({
  plan,
  label,
  athleteId,
  onDeactivate,
  onDelete,
}: {
  plan: AccessoryPlanData
  /** Superset position (A1) or a lone group key. */
  label?: string
  athleteId: number
  onDeactivate: () => void
  onDelete: () => void
}) {
  const queryClient = useQueryClient()
  const [editing, setEditing] = useState(false)
  const [editSets, setEditSets] = useState(plan.target_sets?.toString() ?? '')
  const [editMin, setEditMin] = useState(plan.target_rep_min?.toString() ?? '')
  const [editMax, setEditMax] = useState(plan.target_rep_max?.toString() ?? '')
  const [editWeight, setEditWeight] = useState(plan.target_weight?.toString() ?? '')
  const [editNotes, setEditNotes] = useState(plan.notes ?? '')
  const [editGroup, setEditGroup] = useState<string | null>(plan.group_key ?? null)

  const updateMutation = useMutation({
    mutationFn: () => api.updateAccessoryPlan(athleteId, plan.id, {
      target_sets: editSets ? parseInt(editSets) : 0,
      target_rep_min: editMin ? parseInt(editMin) : 0,
      target_rep_max: editMax ? parseInt(editMax) : 0,
      target_weight: editWeight ? parseFloat(editWeight) : 0,
      notes: editNotes,
      sort_order: plan.sort_order,
      group_key: editGroup ?? '',
    }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['accessories', athleteId] })
      setEditing(false)
    },
    onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to update plan'),
  })

  const groupMutation = useMutation({
    // The update replaces the targets, so carry the current values.
    mutationFn: (key: string | null) => api.updateAccessoryPlan(athleteId, plan.id, {
      target_sets: plan.target_sets ?? 0,
      target_rep_min: plan.target_rep_min ?? 0,
      target_rep_max: plan.target_rep_max ?? 0,
      target_weight: plan.target_weight ?? 0,
      notes: plan.notes ?? '',
      sort_order: plan.sort_order,
      group_key: key ?? '',
    }),
    onSuccess: (_d, key) => {
      queryClient.invalidateQueries({ queryKey: ['accessories', athleteId] })
      toast.success(key ? `${plan.exercise_name} added to group ${key}.` : `${plan.exercise_name} ungrouped.`)
    },
  })

  if (editing) {
    return (
      <Card size="sm">
        <CardContent className="space-y-2">
          <p className="text-sm font-medium">{plan.exercise_name}</p>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-2">
            <div>
              <Label htmlFor={`acc-${plan.id}-edit-sets`} className="text-xs">Sets</Label>
              <Input id={`acc-${plan.id}-edit-sets`} type="number" min={0} value={editSets} onChange={e => setEditSets(e.target.value)} />
            </div>
            <div>
              <Label htmlFor={`acc-${plan.id}-edit-min`} className="text-xs">Min</Label>
              <Input id={`acc-${plan.id}-edit-min`} type="number" min={0} value={editMin} onChange={e => setEditMin(e.target.value)} />
            </div>
            <div>
              <Label htmlFor={`acc-${plan.id}-edit-max`} className="text-xs">Max</Label>
              <Input id={`acc-${plan.id}-edit-max`} type="number" min={0} value={editMax} onChange={e => setEditMax(e.target.value)} />
            </div>
            <div>
              <Label htmlFor={`acc-${plan.id}-edit-weight`} className="text-xs">Weight</Label>
              <Input id={`acc-${plan.id}-edit-weight`} type="number" step="0.5" value={editWeight} onChange={e => setEditWeight(e.target.value)} />
            </div>
            <div>
              <Label htmlFor={`acc-${plan.id}-edit-group`} className="text-xs">Group</Label>
              <GroupSelect id={`acc-${plan.id}-edit-group`} value={editGroup} onChange={setEditGroup} />
            </div>
          </div>
          <div>
            <Label htmlFor={`acc-${plan.id}-edit-notes`} className="text-xs">Notes</Label>
            <Input id={`acc-${plan.id}-edit-notes`} type="text" value={editNotes} onChange={e => setEditNotes(e.target.value)} />
          </div>
          <div className="flex gap-2">
            <Button type="button" size="sm" disabled={updateMutation.isPending} onClick={() => updateMutation.mutate()}>
              {updateMutation.isPending ? 'Saving…' : 'Save'}
            </Button>
            <Button type="button" size="sm" variant="ghost" onClick={() => setEditing(false)}>Cancel</Button>
          </div>
        </CardContent>
      </Card>
    )
  }

  return (
    <Card size="sm">
      <CardContent className="flex items-center justify-between gap-3">
        <div className="min-w-0 flex-1">
          <p className="text-sm font-medium">
            {label && <span className="mr-1.5 rounded bg-primary/15 px-1.5 py-0.5 text-xs font-semibold text-primary">{label}</span>}
            {plan.exercise_name}
          </p>
          <p className="text-xs text-muted-foreground">
            {plan.target_sets && `${plan.target_sets}×`}
            {plan.target_rep_min && plan.target_rep_max
              ? `${plan.target_rep_min}-${plan.target_rep_max}`
              : plan.target_rep_min ?? plan.target_rep_max ?? ''}
            {plan.target_weight ? ` @ ${plan.target_weight}` : ''}
            {plan.notes ? ` — ${plan.notes}` : ''}
          </p>
        </div>
        <div className="flex items-center gap-1">
          <div className="w-28">
            <Label htmlFor={`acc-${plan.id}-group`} className="sr-only">Superset group for {plan.exercise_name}</Label>
            <GroupSelect id={`acc-${plan.id}-group`} value={plan.group_key ?? null} disabled={groupMutation.isPending} onChange={key => groupMutation.mutate(key)} />
          </div>
          <Button variant="ghost" size="icon-sm" aria-label={`Edit ${plan.exercise_name}`} onClick={() => setEditing(true)}>
            <Pencil aria-hidden="true" />
          </Button>
          <Button variant="ghost" size="icon-sm" aria-label={`Deactivate ${plan.exercise_name}`} title="Deactivate" onClick={onDeactivate}>
            <ArchiveX aria-hidden="true" />
          </Button>
          <Button variant="ghost" size="icon-sm" aria-label={`Delete ${plan.exercise_name}`} title="Delete" className="hover:text-destructive" onClick={onDelete}>
            <X aria-hidden="true" />
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}