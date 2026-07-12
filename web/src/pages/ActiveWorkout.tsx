import { useEffect, useMemo, useState } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { useQuery, useQueries, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Check, ChevronLeft, Plus, Timer, Trash2, X } from 'lucide-react'
import { api } from '@/api/client'
import { EmptyState, QueryError } from '@/components/ui'
import { ExercisePicker, type PickedExercise } from '@/components/ExercisePicker'
import { celebrate } from '@/lib/confetti'
import { fullHistoryQuery } from '@/lib/fullHistory'
import { classifyPR, type PRKind, type PRResult } from '@/lib/records'
import { useConfirm } from '@/lib/useConfirm'
import { usePageTitle } from '@/lib/usePageTitle'
import { useRestTimer, type RestTimer } from '@/lib/useRestTimer'
import { useWakeLock } from '@/lib/useWakeLock'
import { formatWeight, cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import type { ExerciseGroup, ExerciseHistoryDayData, PrescriptionSetData, Workout, WorkoutSet } from '@/api/types'

type WorkoutData = { workout: Workout; groups: ExerciseGroup[] }

interface Section {
  exerciseId: number
  name: string
  trainingMax?: number | null
  prescribed: PrescriptionSetData[]
  logged: WorkoutSet[]
}

type Row =
  | { kind: 'done'; index: number; set: WorkoutSet; prescribed?: PrescriptionSetData }
  | { kind: 'pending'; index: number; prescribed?: PrescriptionSetData }

interface RowDraft {
  reps?: string
  weight?: string
  rpe?: string
}

interface AddSetVars {
  exerciseId: number
  exerciseName: string
  rowKey: string
  slotIndex: number
  reps: number
  weight?: number
  rpe?: number
  repType?: string
  /** Row came from extraPending (freeform) rather than a prescribed slot. */
  freeform: boolean
}

/**
 * Map logged sets onto display rows. Sets explicitly claimed by a row (the
 * athlete checked that row this session) keep their slot even when completed
 * out of order; unclaimed sets (e.g. after a reload) fill remaining prescribed
 * slots in set_number order, and anything left over becomes a surplus row.
 */
function buildRows(section: Section, claims: Record<number, number>, extra: number): Row[] {
  const n = section.prescribed.length
  const slots = new Map<number, WorkoutSet>()
  const unclaimed: WorkoutSet[] = []
  const surplus: WorkoutSet[] = []
  const logged = [...section.logged].sort((a, b) => a.set_number - b.set_number)
  for (const s of logged) {
    const idx = claims[s.id]
    if (idx !== undefined && idx < n && !slots.has(idx)) slots.set(idx, s)
    else unclaimed.push(s)
  }
  let cursor = 0
  for (const s of unclaimed) {
    while (cursor < n && slots.has(cursor)) cursor++
    if (cursor < n) slots.set(cursor++, s)
    else surplus.push(s)
  }
  const rows: Row[] = []
  for (let i = 0; i < n; i++) {
    const set = slots.get(i)
    rows.push(set
      ? { kind: 'done', index: i, set, prescribed: section.prescribed[i] }
      : { kind: 'pending', index: i, prescribed: section.prescribed[i] })
  }
  surplus.forEach((s, j) => rows.push({ kind: 'done', index: n + j, set: s }))
  for (let k = 0; k < extra; k++) rows.push({ kind: 'pending', index: n + surplus.length + k })
  return rows
}

function formatElapsed(sec: number): string {
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  const ss = String(sec % 60).padStart(2, '0')
  return h > 0 ? `${h}:${String(m).padStart(2, '0')}:${ss}` : `${String(m).padStart(2, '0')}:${ss}`
}

function shortDate(isoDate: string): string {
  return new Date(`${isoDate.split('T')[0]}T00:00:00`)
    .toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
}

/** One-line "Last: 115 × 8, 8, 7 · Jun 28" summary of the previous session. */
function lastSummary(day: ExerciseHistoryDayData): string {
  const sets = [...day.sets].sort((a, b) => a.set_number - b.set_number)
  if (sets.length === 0) return ''
  const weights = new Set(sets.map(s => s.weight ?? 0))
  const body = weights.size === 1
    ? `${sets[0].weight != null ? `${formatWeight(sets[0].weight)} × ` : ''}${sets.map(s => s.reps).join(', ')}`
    : sets.map(s => `${s.weight != null ? formatWeight(s.weight) : 'BW'}×${s.reps}`).join(', ')
  return `${body} · ${shortDate(day.workout_date)}`
}

/** Rep-type chip / target-reps label for a prescribed set. */
function TargetLabel({ pset }: { pset?: PrescriptionSetData }) {
  if (!pset) return <span className="text-xs text-muted-foreground">any</span>
  const reps = pset.reps
  switch (pset.rep_type) {
    case 'amrap':
      return <Badge className="bg-warning/15 text-warning">{reps != null ? `${reps}+` : 'AMRAP'}</Badge>
    case 'each_side':
      return (
        <span className="flex items-center gap-1">
          {reps != null && <span className="text-sm font-medium">{reps}</span>}
          <Badge variant="secondary">/side</Badge>
        </span>
      )
    case 'seconds':
      return (
        <span className="flex items-center gap-1">
          {reps != null && <span className="text-sm font-medium">{reps}</span>}
          <Badge variant="secondary">sec</Badge>
        </span>
      )
    case 'distance':
      return (
        <span className="flex items-center gap-1">
          {reps != null && <span className="text-sm font-medium">{reps}</span>}
          <Badge variant="secondary">dist</Badge>
        </span>
      )
    default:
      return <span className="text-sm text-muted-foreground">× {reps ?? '—'}</span>
  }
}

function RestTimerBar({ timer }: { timer: RestTimer }) {
  if (!timer.running) return null
  const pct = timer.total > 0 ? (timer.remaining / timer.total) * 100 : 0
  const m = Math.floor(timer.remaining / 60)
  const s = String(timer.remaining % 60).padStart(2, '0')
  return (
    <div className="fixed inset-x-0 bottom-0 z-20 border-t border-border bg-card/95 px-4 pt-3 pb-[calc(0.75rem+env(safe-area-inset-bottom))] backdrop-blur md:left-56">
      <div className="mx-auto max-w-2xl">
        <div className="mb-2 h-1 overflow-hidden rounded-full bg-muted">
          <div className="h-full bg-primary" style={{ width: `${pct}%` }} />
        </div>
        <div className="flex items-center justify-between gap-3">
          <div className="flex items-center gap-2">
            <Timer className="size-5 text-muted-foreground" aria-hidden="true" />
            <span className="text-2xl font-bold tabular-nums" role="timer" aria-label={`Rest timer: ${m}:${s} remaining`}>
              {m}:{s}
            </span>
          </div>
          <div className="flex gap-2">
            <Button variant="outline" size="touch" onClick={() => timer.extend(30)}>+30s</Button>
            <Button variant="ghost" size="touch" onClick={timer.skip}>Skip</Button>
          </div>
        </div>
      </div>
    </div>
  )
}

function prToast(exerciseName: string, pr: PRResult): string {
  switch (pr.kind) {
    case 'weight':
      return `🎉 New ${exerciseName} PR — ${formatWeight(pr.value)} lb!`
    case 'e1rm':
      return `🎉 New ${exerciseName} e1RM PR — ${Math.round(pr.value)} lb!`
    case 'reps-at-weight':
      return `🎉 New ${exerciseName} PR — ${pr.reps} reps at ${formatWeight(pr.weight)} lb!`
    case 'reps':
      return `🎉 New ${exerciseName} PR — ${pr.reps} reps!`
  }
}

const DEFAULT_REST_SECONDS = 90

export function ActiveWorkout() {
  const { id, workoutId } = useParams<{ id: string; workoutId: string }>()
  const athleteId = Number(id)
  const wId = Number(workoutId)
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const { confirm, dialog: confirmDialog } = useConfirm()
  const queryKey = ['workout', athleteId, wId] as const
  const detailPath = `/athletes/${athleteId}/workouts/${wId}`

  usePageTitle('Session')
  useWakeLock()
  const restTimer = useRestTimer(() => {
    navigator.vibrate?.(200)
    toast.success('Rest complete — next set!')
  })

  // setId → prescribed-slot index, recorded as sets are checked this session
  // so out-of-order completion keeps each set on the row that was tapped.
  const [claims, setClaims] = useState<Record<number, number>>({})
  // rowKey → partially-edited inputs; an untouched field falls back to the
  // prescription prefill.
  const [drafts, setDrafts] = useState<Record<string, RowDraft>>({})
  // exerciseId → count of freeform pending rows beyond the prescription.
  const [extraPending, setExtraPending] = useState<Record<number, number>>({})
  const [addedExercises, setAddedExercises] = useState<PickedExercise[]>([])
  const [addingExercise, setAddingExercise] = useState(false)
  const [rpeShown, setRpeShown] = useState<Record<number, boolean>>({})
  // setId → PR kind, for sets that set a personal record this session.
  const [prSets, setPrSets] = useState<Record<number, PRKind>>({})
  const [editingSetId, setEditingSetId] = useState<number | null>(null)
  const [editReps, setEditReps] = useState('')
  const [editWeight, setEditWeight] = useState('')
  const [editRpe, setEditRpe] = useState('')
  const [editSetNotes, setEditSetNotes] = useState('')

  const { data, isLoading, error, refetch } = useQuery({
    queryKey,
    queryFn: () => api.getWorkout(athleteId, wId),
    enabled: !isNaN(athleteId) && !isNaN(wId),
  })
  // 404 = no program for today; the page degrades to a freeform quick-logger.
  const { data: prescription, isLoading: rxLoading } = useQuery({
    queryKey: ['prescription', athleteId],
    queryFn: () => api.getPrescription(athleteId),
    enabled: !isNaN(athleteId),
    retry: false,
  })
  const { data: exercises } = useQuery({
    queryKey: ['exercises'],
    queryFn: () => api.listExercises(),
  })

  const exercisesById = useMemo(() => new Map((exercises ?? []).map(e => [e.id, e])), [exercises])

  const prescribedIds = useMemo(
    () => [...new Set((prescription?.lines ?? []).map(l => l.exercise_id))],
    [prescription],
  )
  const historyQueries = useQueries({
    queries: prescribedIds.map(exId => ({
      queryKey: ['exercise-history', athleteId, exId],
      queryFn: () => api.listExerciseHistory(athleteId, exId),
      enabled: !isNaN(athleteId),
      staleTime: 5 * 60_000,
    })),
  })
  // Latest history day per exercise, skipping this workout's own sets.
  // Recomputed per render — useQueries results are not referentially stable.
  const lastByExercise = new Map<number, ExerciseHistoryDayData>()
  historyQueries.forEach((q, i) => {
    const day = q.data?.days.find(d => d.workout_id !== wId)
    if (day) lastByExercise.set(prescribedIds[i], day)
  })

  const sections = useMemo<Section[]>(() => {
    const groupsByEx = new Map<number, ExerciseGroup>()
    for (const g of data?.groups ?? []) groupsByEx.set(g.exercise_id, g)
    const out: Section[] = []
    const seen = new Set<number>()
    for (const line of prescription?.lines ?? []) {
      // A repeated line for the same exercise merges into one section so the
      // per-exercise logged sets map onto a single ordered row list.
      const existing = out.find(s => s.exerciseId === line.exercise_id)
      if (existing) {
        existing.prescribed = existing.prescribed.concat(line.sets)
        continue
      }
      seen.add(line.exercise_id)
      out.push({
        exerciseId: line.exercise_id,
        name: line.exercise_name,
        trainingMax: line.training_max,
        prescribed: [...line.sets],
        logged: groupsByEx.get(line.exercise_id)?.sets ?? [],
      })
    }
    for (const g of data?.groups ?? []) {
      if (seen.has(g.exercise_id)) continue
      seen.add(g.exercise_id)
      out.push({ exerciseId: g.exercise_id, name: g.exercise_name, prescribed: [], logged: g.sets })
    }
    for (const ex of addedExercises) {
      if (seen.has(ex.id)) continue
      seen.add(ex.id)
      out.push({ exerciseId: ex.id, name: ex.name, prescribed: [], logged: [] })
    }
    return out
  }, [data, prescription, addedExercises])

  // Elapsed clock ticks every second while the page is open.
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    const tid = window.setInterval(() => setNow(Date.now()), 1000)
    return () => window.clearInterval(tid)
  }, [])

  // Best-effort PR check after a set lands: compare against the athlete's
  // full history for the exercise (memoized per session), excluding this
  // workout so earlier sets today don't mask a record.
  async function detectPR(set: WorkoutSet, vars: AddSetVars) {
    // Timed / distance / per-side sets don't have comparable rep counts. The
    // server's rep_type vocabulary is reps|each_side|seconds|distance — plain
    // logged sets (including completed AMRAPs) come back as 'reps'.
    if (set.rep_type === 'seconds' || set.rep_type === 'distance' || set.rep_type === 'each_side') return
    try {
      const days = await queryClient.fetchQuery(fullHistoryQuery(athleteId, vars.exerciseId))
      const prior = days.filter(d => d.workout_id !== wId).flatMap(d => d.sets)
      const pr = classifyPR(prior, set)
      if (!pr) return
      setPrSets(p => ({ ...p, [set.id]: pr.kind }))
      toast.success(prToast(vars.exerciseName, pr), { duration: 5000 })
      celebrate()
    } catch {
      // Never let PR detection surface an error mid-session.
    }
  }

  const addSetMutation = useMutation({
    mutationFn: (vars: AddSetVars) => api.addSet(athleteId, wId, {
      exercise_id: vars.exerciseId,
      reps: vars.reps,
      weight: vars.weight,
      rpe: vars.rpe,
      rep_type: vars.repType,
    }),
    onMutate: async (vars) => {
      await queryClient.cancelQueries({ queryKey })
      const prev = queryClient.getQueryData<WorkoutData>(queryKey)
      const tempId = -Date.now()
      const optimistic: WorkoutSet = {
        id: tempId,
        workout_id: wId,
        exercise_id: vars.exerciseId,
        set_number: 0,
        reps: vars.reps,
        weight: vars.weight ?? null,
        rpe: vars.rpe ?? null,
        rep_type: vars.repType ?? 'standard',
        category: '',
        notes: null,
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
      }
      if (prev) {
        const groups = [...prev.groups]
        const gi = groups.findIndex(g => g.exercise_id === vars.exerciseId)
        if (gi >= 0) {
          const sets = [...groups[gi].sets, { ...optimistic, set_number: groups[gi].sets.length + 1 }]
          groups[gi] = { ...groups[gi], sets }
        } else {
          groups.push({ exercise_id: vars.exerciseId, exercise_name: vars.exerciseName, sets: [{ ...optimistic, set_number: 1 }] })
        }
        queryClient.setQueryData<WorkoutData>(queryKey, { ...prev, groups })
      }
      setClaims(c => ({ ...c, [tempId]: vars.slotIndex }))
      const draft = drafts[vars.rowKey]
      setDrafts(d => {
        const next = { ...d }
        delete next[vars.rowKey]
        return next
      })
      if (vars.freeform) {
        setExtraPending(p => ({ ...p, [vars.exerciseId]: Math.max(0, (p[vars.exerciseId] ?? 0) - 1) }))
      }
      return { prev, draft }
    },
    onError: (_err, vars, ctx) => {
      if (ctx?.prev) queryClient.setQueryData(queryKey, ctx.prev)
      if (ctx?.draft) setDrafts(d => ({ ...d, [vars.rowKey]: ctx.draft as RowDraft }))
      if (vars.freeform) {
        setExtraPending(p => ({ ...p, [vars.exerciseId]: (p[vars.exerciseId] ?? 0) + 1 }))
      }
    },
    onSuccess: (set, vars) => {
      // Re-claim under the real id (the optimistic temp id disappears on refetch).
      setClaims(c => ({ ...c, [set.id]: vars.slotIndex }))
      const rest = exercisesById.get(vars.exerciseId)?.rest_seconds
      restTimer.start(rest && rest > 0 ? rest : DEFAULT_REST_SECONDS)
      void detectPR(set, vars)
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey }),
  })

  const updateSetMutation = useMutation({
    // Send explicit values (not undefined) so an intentionally-cleared field is
    // cleared — the API treats an omitted field as "leave unchanged".
    mutationFn: (setId: number) => api.updateSet(athleteId, wId, setId, {
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
      setEditingSetId(cur => cur === setId ? null : cur)
      return { prev }
    },
    onError: (_err, _vars, ctx) => {
      if (ctx?.prev) queryClient.setQueryData(queryKey, ctx.prev)
    },
    onSuccess: () => toast.success('Set deleted'),
    onSettled: () => queryClient.invalidateQueries({ queryKey }),
  })

  function draftValue(key: string, field: keyof RowDraft, fallback: string): string {
    const v = drafts[key]?.[field]
    return v !== undefined ? v : fallback
  }

  function setDraft(key: string, field: keyof RowDraft, value: string) {
    setDrafts(prev => ({ ...prev, [key]: { ...prev[key], [field]: value } }))
  }

  function beginEdit(set: WorkoutSet) {
    setEditingSetId(set.id)
    setEditReps(set.reps.toString())
    setEditWeight(set.weight?.toString() ?? '')
    setEditRpe(set.rpe?.toString() ?? '')
    setEditSetNotes(set.notes ?? '')
  }

  function addExercise(ex: PickedExercise) {
    setAddingExercise(false)
    if (!sections.some(s => s.exerciseId === ex.id)) {
      setAddedExercises(a => a.some(e => e.id === ex.id) ? a : [...a, ex])
    }
    setExtraPending(p => ({ ...p, [ex.id]: (p[ex.id] ?? 0) + 1 }))
  }

  if (isLoading || rxLoading) {
    return (
      <div className="mx-auto max-w-2xl space-y-4">
        <Skeleton className="h-14 w-full" />
        <Skeleton className="h-40 w-full" />
        <Skeleton className="h-40 w-full" />
      </div>
    )
  }
  if (error) return <QueryError error={error} onRetry={refetch} resource="workout" />
  if (!data) return <EmptyState title="Workout not found" description="It may have been deleted." />

  const { workout } = data
  // A 404 (or any prescription failure) degrades to freeform logging.
  const noProgram = !prescription

  const totalPrescribed = sections.reduce((n, s) => n + s.prescribed.length, 0)
  const donePrescribed = sections.reduce((n, s) => n + Math.min(s.logged.length, s.prescribed.length), 0)
  const totalLogged = sections.reduce((n, s) => n + s.logged.length, 0)
  const elapsedSec = Math.max(0, Math.floor((now - new Date(workout.created_at).getTime()) / 1000))

  return (
    <div className={cn('mx-auto max-w-2xl', restTimer.running ? 'pb-36' : 'pb-8')}>
      {/* Sticky session header — sits below the fixed mobile nav (top-16). */}
      <div className="sticky top-16 z-20 -mx-4 mb-4 border-b border-border bg-background/95 px-4 py-2 backdrop-blur md:top-0 md:-mx-6 md:px-6">
        <div className="flex items-center justify-between gap-2">
          <Link
            to={detailPath}
            className="-ml-2 flex h-11 shrink-0 items-center gap-0.5 rounded-lg px-2 text-sm text-muted-foreground outline-none transition-colors hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/50"
            aria-label="Workout details"
          >
            <ChevronLeft className="size-4" aria-hidden="true" />
            Details
          </Link>
          <div className="min-w-0 text-center">
            <p className="truncate text-sm font-semibold">
              {totalPrescribed > 0
                ? donePrescribed >= totalPrescribed
                  ? 'All sets done'
                  : `Set ${donePrescribed + 1} of ${totalPrescribed}`
                : `${totalLogged} set${totalLogged === 1 ? '' : 's'} logged`}
            </p>
            <p className="text-xs tabular-nums text-muted-foreground">{formatElapsed(elapsedSec)}</p>
          </div>
          <Button size="touch" onClick={() => navigate(detailPath)}>Finish</Button>
        </div>
        {totalPrescribed > 0 && (
          <div
            className="mt-2 h-1 overflow-hidden rounded-full bg-muted"
            role="progressbar"
            aria-label="Sets completed"
            aria-valuemin={0}
            aria-valuemax={totalPrescribed}
            aria-valuenow={donePrescribed}
          >
            <div
              className="h-full bg-primary transition-all"
              style={{ width: `${(donePrescribed / totalPrescribed) * 100}%` }}
            />
          </div>
        )}
      </div>

      {!noProgram && (
        <p className="mb-4 text-sm text-muted-foreground">
          {prescription.program_name} — Week {prescription.current_week}, Day {prescription.current_day}
          {prescription.cycle_number > 1 && ` (Cycle ${prescription.cycle_number})`}
        </p>
      )}

      {sections.length === 0 ? (
        <EmptyState
          icon="🏋️"
          title={noProgram ? 'No program for today' : 'Rest day'}
          description="Add an exercise below to log freeform sets."
        />
      ) : (
        <div className="space-y-4">
          {sections.map(section => {
            const rows = buildRows(section, claims, extraPending[section.exerciseId] ?? 0)
            const prevDay = lastByExercise.get(section.exerciseId)
            const showRpe = rpeShown[section.exerciseId] ?? false
            return (
              <section key={section.exerciseId} className="overflow-hidden rounded-xl border border-border bg-card">
                <div className="border-b border-border bg-muted/50 px-3 py-2">
                  <div className="flex items-center justify-between gap-2">
                    <h2 className="min-w-0 truncate font-semibold">{section.name}</h2>
                    <div className="flex shrink-0 items-center gap-1">
                      {section.trainingMax != null && (
                        <span className="text-xs text-muted-foreground">TM {formatWeight(section.trainingMax)}</span>
                      )}
                      <Button
                        variant="ghost"
                        size="icon-touch"
                        className="w-auto px-2 text-xs text-muted-foreground"
                        aria-pressed={showRpe}
                        aria-label={`${showRpe ? 'Hide' : 'Show'} RPE inputs for ${section.name}`}
                        onClick={() => setRpeShown(p => ({ ...p, [section.exerciseId]: !showRpe }))}
                      >
                        RPE
                      </Button>
                    </div>
                  </div>
                  {prevDay && (
                    <p className="text-xs text-muted-foreground">Last: {lastSummary(prevDay)}</p>
                  )}
                </div>

                <div className="divide-y divide-border">
                  {rows.map(row => {
                    const key = `${section.exerciseId}:${row.index}`
                    const prevSet = prevDay?.sets.find(s => s.set_number === row.index + 1)

                    if (row.kind === 'done') {
                      const set = row.set
                      if (editingSetId === set.id) {
                        return (
                          <div key={key} className="space-y-2 bg-muted/30 px-3 py-2">
                            <div className="grid grid-cols-[1.75rem_minmax(0,1fr)_4rem_3.5rem_2.75rem] items-center gap-2">
                              <span className="text-sm tabular-nums text-muted-foreground">{row.index + 1}</span>
                              <Input type="number" inputMode="decimal" enterKeyHint="done" step="0.5" value={editWeight} onChange={e => setEditWeight(e.target.value)} className="h-11" aria-label="Weight" />
                              <Input type="number" inputMode="numeric" enterKeyHint="done" min={1} value={editReps} onChange={e => setEditReps(e.target.value)} className="h-11" aria-label="Reps" />
                              <Input type="number" inputMode="decimal" enterKeyHint="done" step="0.5" min={1} max={10} value={editRpe} onChange={e => setEditRpe(e.target.value)} className="h-11" aria-label="RPE" />
                              <Button
                                size="icon-touch"
                                aria-label="Save set"
                                disabled={updateSetMutation.isPending || !editReps}
                                onClick={() => updateSetMutation.mutate(set.id)}
                              ><Check aria-hidden="true" /></Button>
                            </div>
                            <div className="flex items-center gap-2">
                              <Input type="text" value={editSetNotes} onChange={e => setEditSetNotes(e.target.value)} placeholder="Notes" className="h-11 flex-1" aria-label="Set notes" />
                              <Button variant="ghost" size="icon-touch" aria-label="Cancel edit" onClick={() => setEditingSetId(null)}>
                                <X aria-hidden="true" />
                              </Button>
                              <Button
                                variant="ghost"
                                size="icon-touch"
                                aria-label={`Delete set ${row.index + 1}`}
                                className="text-muted-foreground hover:text-destructive"
                                onClick={async () => {
                                  if (await confirm({ title: 'Delete Set', description: 'Remove this set?', confirmLabel: 'Delete', variant: 'danger' }))
                                    deleteSetMutation.mutate(set.id)
                                }}
                              ><Trash2 aria-hidden="true" /></Button>
                            </div>
                          </div>
                        )
                      }
                      return (
                        <button
                          key={key}
                          type="button"
                          onClick={() => beginEdit(set)}
                          aria-label={`Edit set ${row.index + 1}: ${set.weight != null ? `${formatWeight(set.weight)} for ` : ''}${set.reps_label ?? set.reps} reps${prSets[set.id] ? ' — personal record' : ''}`}
                          className="grid w-full grid-cols-[1.75rem_minmax(0,1fr)_2.75rem] items-center gap-2 px-3 py-2 text-left outline-none transition-colors hover:bg-muted/40 focus-visible:ring-2 focus-visible:ring-ring/50"
                        >
                          <span className="text-sm tabular-nums text-muted-foreground">{row.index + 1}</span>
                          <span className="min-w-0 truncate">
                            <span className="text-sm font-medium">
                              {set.weight != null ? formatWeight(set.weight) : 'BW'} × {set.reps_label ?? set.reps}
                            </span>
                            {set.rpe != null && <span className="text-xs text-muted-foreground"> @ RPE {set.rpe}</span>}
                            {set.notes && <span className="text-xs text-muted-foreground"> · {set.notes}</span>}
                            {prSets[set.id] && (
                              <Badge className="ml-1.5 bg-warning/15 text-warning" aria-label="Personal record">PR</Badge>
                            )}
                          </span>
                          <span className="flex size-11 items-center justify-center rounded-full bg-success/15 text-success" aria-hidden="true">
                            <Check className="size-5" />
                          </span>
                        </button>
                      )
                    }

                    // Pending row: inputs prefilled from the prescription;
                    // AMRAP targets start empty and gate the check button.
                    const pset = row.prescribed
                    const isAmrap = pset != null && (pset.reps == null || pset.rep_type === 'amrap')
                    const defaultReps = pset && !isAmrap && pset.reps != null ? String(pset.reps) : ''
                    const target = pset ? (pset.target_weight ?? pset.absolute_weight) : null
                    const defaultWeight = target != null ? String(target) : ''
                    const repsStr = draftValue(key, 'reps', defaultReps)
                    const weightStr = draftValue(key, 'weight', defaultWeight)
                    const rpeStr = draftValue(key, 'rpe', '')
                    const repsNum = parseInt(repsStr)
                    const canCheck = repsStr.trim() !== '' && !isNaN(repsNum) && repsNum >= 1 && !addSetMutation.isPending
                    return (
                      <div key={key} className="px-3 py-2">
                        <div className="grid grid-cols-[1.75rem_minmax(0,1fr)_4.5rem_3.75rem_2.75rem] items-center gap-2">
                          <span className="text-sm tabular-nums text-muted-foreground">{row.index + 1}</span>
                          <div className="min-w-0">
                            <TargetLabel pset={pset} />
                            {pset?.percentage != null && (
                              <p className="text-[11px] text-muted-foreground">@ {pset.percentage}%</p>
                            )}
                            {prevSet && (
                              <p className="text-[11px] text-muted-foreground">
                                prev {prevSet.weight != null ? `${formatWeight(prevSet.weight)}×` : ''}{prevSet.reps}
                              </p>
                            )}
                          </div>
                          <Input
                            type="number" inputMode="decimal" enterKeyHint="done" step="0.5"
                            value={weightStr}
                            onChange={e => setDraft(key, 'weight', e.target.value)}
                            placeholder={prevSet?.weight != null ? formatWeight(prevSet.weight) : 'BW'}
                            className="h-11"
                            aria-label={`Set ${row.index + 1} weight`}
                          />
                          <Input
                            type="number" inputMode="numeric" enterKeyHint="done" min={1}
                            value={repsStr}
                            onChange={e => setDraft(key, 'reps', e.target.value)}
                            placeholder={prevSet ? String(prevSet.reps) : isAmrap && pset?.reps != null ? `${pset.reps}+` : undefined}
                            required={isAmrap}
                            className="h-11"
                            aria-label={`Set ${row.index + 1} reps${isAmrap ? ' (AMRAP — enter reps achieved)' : ''}`}
                          />
                          <Button
                            size="icon-touch"
                            variant={canCheck ? 'default' : 'outline'}
                            disabled={!canCheck}
                            aria-label={`Log set ${row.index + 1}`}
                            onClick={() => addSetMutation.mutate({
                              exerciseId: section.exerciseId,
                              exerciseName: section.name,
                              rowKey: key,
                              slotIndex: row.index,
                              reps: repsNum,
                              weight: weightStr ? parseFloat(weightStr) : undefined,
                              rpe: rpeStr ? parseFloat(rpeStr) : undefined,
                              repType: pset && pset.rep_type !== 'standard' ? pset.rep_type : undefined,
                              freeform: pset == null,
                            })}
                          ><Check aria-hidden="true" /></Button>
                        </div>
                        {showRpe && (
                          <div className="mt-2 flex items-center justify-end gap-2">
                            <Label htmlFor={`rpe-${key}`} className="text-xs text-muted-foreground">RPE</Label>
                            <Input
                              id={`rpe-${key}`}
                              type="number" inputMode="decimal" enterKeyHint="done" step="0.5" min={1} max={10}
                              value={rpeStr}
                              onChange={e => setDraft(key, 'rpe', e.target.value)}
                              className="h-11 w-20"
                            />
                          </div>
                        )}
                        {pset?.notes && (
                          <p className="mt-1 text-xs text-muted-foreground">{pset.notes}</p>
                        )}
                      </div>
                    )
                  })}
                </div>

                <button
                  type="button"
                  onClick={() => setExtraPending(p => ({ ...p, [section.exerciseId]: (p[section.exerciseId] ?? 0) + 1 }))}
                  className="flex h-11 w-full items-center justify-center gap-1 border-t border-border text-sm text-muted-foreground outline-none transition-colors hover:bg-muted/40 hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/50"
                >
                  <Plus className="size-4" aria-hidden="true" /> Add set
                </button>
              </section>
            )
          })}
        </div>
      )}

      {/* Non-prescribed logging */}
      <div className="mt-4">
        {addingExercise ? (
          <div className="rounded-xl border border-border bg-card p-3">
            <Label id="session-add-exercise">Add exercise</Label>
            <ExercisePicker athleteId={athleteId} value={null} onSelect={addExercise} triggerLabelId="session-add-exercise" />
            <Button variant="ghost" size="touch" className="mt-2" onClick={() => setAddingExercise(false)}>
              Cancel
            </Button>
          </div>
        ) : (
          <Button
            variant="outline"
            size="touch"
            onClick={() => setAddingExercise(true)}
            className="w-full border-dashed text-muted-foreground hover:text-foreground"
          >
            <Plus aria-hidden="true" /> Add exercise
          </Button>
        )}
      </div>

      <RestTimerBar timer={restTimer} />
      {confirmDialog()}
    </div>
  )
}
