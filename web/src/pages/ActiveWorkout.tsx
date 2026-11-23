import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { useQuery, useQueries, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Check, ChevronLeft, Flame, Link2, Plus, Timer, Trash2, Undo2, X } from 'lucide-react'
import { api, ApiError } from '@/api/client'
import { EmptyState } from '@/components/ui'
import { PageSkeleton, QueryState } from '@/components/QueryState'
import { useFriendlyDate } from '@/lib/dates'
import { ExercisePicker, type PickedExercise } from '@/components/ExercisePicker'
import { ExerciseCuesSheet } from '@/components/ExerciseCuesSheet'
import { CueNote } from '@/components/CueNote'
import { PlateCalculator } from '@/components/PlateCalculator'
import { ReadinessPrompt, type ReadinessAnswers } from '@/components/ReadinessPrompt'
import { SetVideoControl } from '@/components/SetVideoSheet'
import { SwapExerciseSheet } from '@/components/SwapExerciseSheet'
import { celebrate } from '@/lib/confetti'
import { fullHistoryQuery } from '@/lib/fullHistory'
import { groupBlocks, groupKind, memberLabel } from '@/lib/groups'
import { emptyBarKg, fromKg, inventoryFromSetup, KG_COMPETITION, normalizeUnit, type PlateInventory, type WeightUnit } from '@/lib/plates'
import { classifyPR, isWarmup, type PRKind, type PRResult } from '@/lib/records'
import {
  buildRows,
  buildSections,
  buildWarmupRows,
  effortLabel,
  firstWorkingTarget,
  interleaveRounds,
  repTargetLabel,
  roundComplete,
  sessionProgress,
  type Row,
  type Section,
  type WarmupRow,
  type WarmupTarget,
} from '@/lib/session'
import { useConfirm } from '@/lib/useConfirm'
import { usePageTitle } from '@/lib/usePageTitle'
import { useRestTimer, type RestTimer } from '@/lib/useRestTimer'
import { useRestAlertSink } from '@/lib/useRestAlerts'
import { withRestAlerts } from '@/lib/push'
import { useWakeLock } from '@/lib/useWakeLock'
import { generateWarmupRamp } from '@/lib/warmups'
import { distinctNotes } from '@/lib/setSummary'
import { formatWeight, cn, localDateISO } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import type { ExerciseHistoryDayData, ExerciseHistoryEntryData, PrescriptionSetData, WorkoutDetailData, WorkoutSet } from '@/api/types'

/**
 * The coach's cue when every prescribed set of this exercise carries the same
 * one (imports repeat it per set). Shown once under the exercise name instead
 * of under every row. Null when there is none or the sets differ.
 */
function sharedCue(section: Section): string | null {
  const notes = distinctNotes([...section.prescribedWarmups, ...section.prescribed])
  return notes.length === 1 ? notes[0] : null
}

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
  restSeconds?: number | null
  /** Row came from extraPending (freeform) rather than a prescribed slot. */
  freeform: boolean
  warmup: boolean
  /** Superset/circuit: another member still has a set in this round. */
  skipRest: boolean
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

/** Previous session's working sets, in order. Warm-ups never count as "prev". */
function workingSets(day: ExerciseHistoryDayData): ExerciseHistoryEntryData[] {
  return [...day.sets].filter(s => !isWarmup(s)).sort((a, b) => a.set_number - b.set_number)
}

/** One-line "Last: 115 × 8, 8, 7 · Jun 28" summary of the previous session. */
function lastSummary(day: ExerciseHistoryDayData): string {
  const sets = workingSets(day)
  if (sets.length === 0) return ''
  const weights = new Set(sets.map(s => s.weight ?? 0))
  const body = weights.size === 1
    ? `${sets[0].weight != null ? `${formatWeight(sets[0].weight)} × ` : ''}${sets.map(s => s.reps).join(', ')}`
    : sets.map(s => `${s.weight != null ? formatWeight(s.weight) : 'BW'}×${s.reps}`).join(', ')
  return `${body} · ${shortDate(day.workout_date)}`
}

/** Rep-type chip / target-reps label for a prescribed set, with any RIR/RPE target. */
function TargetLabel({ pset }: { pset?: PrescriptionSetData }) {
  if (!pset) return <span className="text-xs text-muted-foreground">any</span>
  const reps = repTargetLabel(pset)
  const effort = effortLabel(pset)
  let main: ReactNode
  switch (pset.rep_type) {
    case 'amrap':
      main = <Badge className="bg-warning/15 text-warning-strong">{reps ?? 'AMRAP'}</Badge>
      break
    case 'each_side':
    case 'seconds':
    case 'distance':
      main = (
        <span className="flex items-center gap-1">
          {reps != null && <span className="text-sm font-medium">{reps}</span>}
          <Badge variant="secondary">{pset.rep_type === 'each_side' ? '/side' : pset.rep_type === 'seconds' ? 'sec' : 'dist'}</Badge>
        </span>
      )
      break
    default:
      main = <span className="text-sm text-muted-foreground">× {reps ?? '—'}</span>
  }
  return (
    <span className="flex flex-wrap items-center gap-1">
      {main}
      {effort && <Badge variant="outline" className="text-[11px]">{effort}</Badge>}
    </span>
  )
}

function RestTimerBar({ timer }: { timer: RestTimer }) {
  // Flag the bar on <html> so desktop toasts lift above it (index.css).
  useEffect(() => {
    if (!timer.running) return
    const root = document.documentElement
    root.setAttribute('data-rest-timer', '')
    return () => root.removeAttribute('data-rest-timer')
  }, [timer.running])
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

// Per-workout "skip the readiness check-in" memory. Browser storage can be
// unavailable (private mode); then the prompt just shows again next time.
function readinessDismissedKey(workoutId: number): string {
  return `replog.readiness-dismissed.${workoutId}`
}
function readReadinessDismissed(workoutId: number): boolean {
  try {
    return localStorage.getItem(readinessDismissedKey(workoutId)) === '1'
  } catch {
    return false
  }
}
function writeReadinessDismissed(workoutId: number) {
  try {
    localStorage.setItem(readinessDismissedKey(workoutId), '1')
  } catch {
    // Not persisted; the in-memory flag still hides the prompt.
  }
}

/** Row label for a superset member's set: "A1 · 2". */
function roundLabel(member: string | null, index: number): string {
  return member ? `${member}·${index + 1}` : String(index + 1)
}

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
  const localRestTimer = useRestTimer(() => {
    navigator.vibrate?.(200)
    toast.success('Rest complete — next set!')
  })
  // On devices that opted in, the server also pushes when the rest ends,
  // so a locked phone still buzzes. Elsewhere this is the plain timer.
  const restAlerts = useRestAlertSink(`/athletes/${athleteId}/workouts/${wId}/session`)
  const restTimer = withRestAlerts(localRestTimer, restAlerts)

  // setId → prescribed-slot index, recorded as sets are checked this session
  // so out-of-order completion keeps each set on the row that was tapped.
  const [claims, setClaims] = useState<Record<number, number>>({})
  // rowKey → partially-edited inputs; an untouched field falls back to the
  // prescription prefill.
  const [drafts, setDrafts] = useState<Record<string, RowDraft>>({})
  // exerciseId → count of freeform pending rows beyond the prescription.
  const [extraPending, setExtraPending] = useState<Record<number, number>>({})
  // exerciseId → generated warm-up ramp (suggestions; never saved to the program).
  const [rampTargets, setRampTargets] = useState<Record<number, WarmupTarget[]>>({})
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
  const [readinessDismissed, setReadinessDismissed] = useState(() => readReadinessDismissed(wId))

  const friendlyDate = useFriendlyDate()
  const workoutQuery = useQuery({
    queryKey,
    queryFn: () => api.getWorkout(athleteId, wId),
    enabled: !isNaN(athleteId) && !isNaN(wId),
  })
  // 404 = no program for today; the page degrades to a freeform quick-logger.
  const data = workoutQuery.data
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
  const { data: prefs } = useQuery({
    queryKey: ['preferences'],
    queryFn: () => api.getPreferences(),
    staleTime: 10 * 60_000,
  })
  const { data: plateSetup } = useQuery({
    queryKey: ['plate-setup', athleteId],
    queryFn: () => api.getPlateSetup(athleteId),
    enabled: !isNaN(athleteId),
    staleTime: 10 * 60_000,
  })
  const unit: WeightUnit = normalizeUnit(prefs?.weight_unit)
  const inventory: PlateInventory = useMemo(
    () => (plateSetup ? inventoryFromSetup(plateSetup) : KG_COMPETITION),
    [plateSetup],
  )
  const emptyBar = fromKg(emptyBarKg(inventory), unit)

  const exercisesById = useMemo(() => new Map((exercises ?? []).map(e => [e.id, e])), [exercises])

  const sections = useMemo<Section[]>(
    () => buildSections(prescription?.lines ?? [], data?.groups ?? [], data?.swaps ?? [], addedExercises),
    [data, prescription, addedExercises],
  )

  const prescribedIds = useMemo(
    () => sections.filter(s => s.prescribedExerciseId != null).map(s => s.exerciseId),
    [sections],
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
    const day = q.data?.days.find(d => d.workout_id !== wId && workingSets(d).length > 0)
    if (day) lastByExercise.set(prescribedIds[i], day)
  })

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
    if (vars.warmup || isWarmup(set)) return
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
      set_type: vars.warmup ? 'warmup' : undefined,
    }),
    onMutate: async (vars) => {
      await queryClient.cancelQueries({ queryKey })
      const prev = queryClient.getQueryData<WorkoutDetailData>(queryKey)
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
        set_type: vars.warmup ? 'warmup' : 'working',
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
        queryClient.setQueryData<WorkoutDetailData>(queryKey, { ...prev, groups })
      }
      if (!vars.warmup) setClaims(c => ({ ...c, [tempId]: vars.slotIndex }))
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
      if (!vars.warmup) setClaims(c => ({ ...c, [set.id]: vars.slotIndex }))
      if (vars.warmup) {
        // Warm-ups only start a timer when the coach prescribed a rest.
        if (vars.restSeconds) restTimer.start(vars.restSeconds)
      } else if (!vars.skipRest) {
        const exerciseRest = exercisesById.get(vars.exerciseId)?.rest_seconds
        const rest = vars.restSeconds ?? (exerciseRest && exerciseRest > 0 ? exerciseRest : DEFAULT_REST_SECONDS)
        restTimer.start(rest)
      }
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
      const prev = queryClient.getQueryData<WorkoutDetailData>(queryKey)
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
        queryClient.setQueryData<WorkoutDetailData>(queryKey, { ...prev, groups })
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
      const prev = queryClient.getQueryData<WorkoutDetailData>(queryKey)
      if (prev) {
        const groups = prev.groups
          .map(g => ({ ...g, sets: g.sets.filter(s => s.id !== setId) }))
          .filter(g => g.sets.length > 0)
        queryClient.setQueryData<WorkoutDetailData>(queryKey, { ...prev, groups })
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

  const swapMutation = useMutation({
    mutationFn: (vars: { from: number; to: number; reason: string }) =>
      api.createExerciseSwap(athleteId, wId, { from_exercise_id: vars.from, to_exercise_id: vars.to, reason: vars.reason || undefined }),
    meta: { skipGlobalError: true },
    onSuccess: (swap) => toast.success(`Swapped to ${swap.to_exercise_name}. Your coach will see it in review.`),
    onError: (err) => toast.error(err instanceof ApiError ? err.message : 'Failed to swap exercise'),
    onSettled: () => queryClient.invalidateQueries({ queryKey }),
  })

  const undoSwapMutation = useMutation({
    mutationFn: (swapId: number) => api.deleteExerciseSwap(athleteId, wId, swapId),
    meta: { skipGlobalError: true },
    onSuccess: () => toast.success('Swap undone'),
    onError: (err) => toast.error(err instanceof ApiError ? err.message : 'Failed to undo swap'),
    onSettled: () => queryClient.invalidateQueries({ queryKey }),
  })

  const readinessMutation = useMutation({
    mutationFn: (answers: ReadinessAnswers) => api.createRecoveryCheckin(athleteId, {
      date: data?.workout.date.split('T')[0] ?? localDateISO(),
      ...answers,
      notes: 'Pre-session check-in',
    }),
    meta: { skipGlobalError: true },
    onSuccess: (checkin) => {
      dismissReadiness()
      toast.success(checkin.low_readiness
        ? 'Check-in saved. It\'s flagged for your coach; today\'s plan is unchanged.'
        : 'Check-in saved for your coach.')
      queryClient.invalidateQueries({ queryKey: ['recovery-checkins', athleteId] })
    },
    onError: (err) => {
      if (err instanceof ApiError && err.code === 409) {
        // Already checked in today (e.g. from the Recovery page).
        dismissReadiness()
        return
      }
      toast.error('Failed to save check-in')
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey }),
  })

  function dismissReadiness() {
    writeReadinessDismissed(wId)
    setReadinessDismissed(true)
  }

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

  function generateRamp(section: Section) {
    const firstKey = `${section.exerciseId}:0`
    const typed = parseFloat(drafts[firstKey]?.weight ?? '')
    const working = firstWorkingTarget(section) ?? (isNaN(typed) ? null : typed)
    if (working == null) {
      toast.info('Enter the first working weight, then tap Warm-ups.')
      return
    }
    const ramp = generateWarmupRamp(working, inventory, unit)
    if (ramp.length === 0) {
      toast.info('No warm-up ramp needed at or below the empty bar.')
      return
    }
    setRampTargets(r => ({ ...r, [section.exerciseId]: ramp.map(s => ({ label: s.label, weight: s.weight, reps: s.reps })) }))
  }

  // Rendered inline (not as nested components) so inputs keep focus.
  function renderEditRow(key: string, label: string, set: WorkoutSet, exercise?: string) {
    return (
      <div key={key} className="space-y-2 bg-muted/30 px-3 py-2">
        <div className="grid grid-cols-[2.25rem_minmax(0,1fr)_4rem_3.5rem_2.75rem] items-center gap-2">
          <span className="text-sm tabular-nums text-muted-foreground">{label}</span>
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
          {/* Form check: attach a short clip to this logged set for the coach's review. */}
          <SetVideoControl
            athleteId={athleteId}
            workoutId={wId}
            setId={set.id}
            setNumber={set.set_number}
            exerciseName={exercise ?? set.exercise_name ?? ''}
            video={set.video}
            canComment={false}
          />
          <Button variant="ghost" size="icon-touch" aria-label="Cancel edit" onClick={() => setEditingSetId(null)}>
            <X aria-hidden="true" />
          </Button>
          <Button
            variant="ghost"
            size="icon-touch"
            aria-label={`Delete set ${label}`}
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

  function renderDoneRow(key: string, label: string, set: WorkoutSet, opts: { warmup?: boolean; name?: string; exercise?: string } = {}) {
    if (editingSetId === set.id) return renderEditRow(key, label, set, opts.exercise)
    const pr = !opts.warmup && prSets[set.id]
    return (
      <button
        key={key}
        type="button"
        onClick={() => beginEdit(set)}
        aria-label={`Edit ${opts.exercise ? `${opts.exercise} ` : ''}${opts.warmup ? 'warm-up' : 'set'} ${label}: ${set.weight != null ? `${formatWeight(set.weight)} for ` : ''}${set.reps_label ?? set.reps} reps${pr ? ' — personal record' : ''}`}
        className={cn(
          'grid w-full grid-cols-[2.25rem_minmax(0,1fr)_2.75rem] items-center gap-2 px-3 py-2 text-left outline-none transition-colors hover:bg-muted/40 focus-visible:ring-2 focus-visible:ring-ring/50',
          opts.warmup && 'bg-muted/30',
        )}
      >
        <span className="text-sm tabular-nums text-muted-foreground">{label}</span>
        <span className="min-w-0 truncate">
          {opts.name && <span className="block truncate text-xs text-muted-foreground">{opts.name}</span>}
          <span className={cn('text-sm font-medium', opts.warmup && 'text-muted-foreground')}>
            {set.weight != null ? formatWeight(set.weight) : 'BW'} × {set.reps_label ?? set.reps}
          </span>
          {set.rpe != null && <span className="text-xs text-muted-foreground"> @ RPE {set.rpe}</span>}
          {set.notes && <span className="text-xs text-muted-foreground"> · {set.notes}</span>}
          {pr && (
            <Badge className="ml-1.5 bg-warning/15 text-warning-strong" aria-label="Personal record">PR</Badge>
          )}
        </span>
        <span
          className={cn(
            'flex size-11 items-center justify-center rounded-full',
            opts.warmup ? 'bg-muted text-muted-foreground' : 'bg-success/15 text-success-strong',
          )}
          aria-hidden="true"
        >
          <Check className="size-5" />
        </span>
      </button>
    )
  }

  function plateLine(key: string, label: string, weightStr: string, barbellBefore = false) {
    const w = parseFloat(weightStr)
    const hasTarget = !isNaN(w) && w > 0 && w >= emptyBar - 0.05
    // With no weight typed yet, still offer the plate counter on a lift that
    // was loaded on the bar last time, so the weight can come from the plates
    // actually stacked. Bodyweight rows stay clean.
    if (!hasTarget && !(barbellBefore && weightStr.trim() === '')) return null
    return (
      <PlateCalculator
        target={hasTarget ? w : undefined}
        inventory={inventory}
        unit={unit}
        label={label}
        onUse={weight => setDraft(key, 'weight', String(weight))}
      />
    )
  }

  function renderWarmupRow(section: Section, row: WarmupRow, label: string) {
    const key = `${section.exerciseId}:w${row.index}`
    if (row.kind === 'done') return renderDoneRow(key, label, row.set, { warmup: true, exercise: section.name })
    const t = row.target
    const repsStr = draftValue(key, 'reps', t.reps != null ? String(t.reps) : '')
    const weightStr = draftValue(key, 'weight', t.weight != null ? String(t.weight) : '')
    const repsNum = parseInt(repsStr)
    const canCheck = repsStr.trim() !== '' && !isNaN(repsNum) && repsNum >= 1 && !addSetMutation.isPending
    return (
      <div key={key} className="bg-muted/30 px-3 py-2">
        <div className="grid grid-cols-[2.25rem_minmax(0,1fr)_4.5rem_3.75rem_2.75rem] items-center gap-2">
          <Badge variant="outline" className="w-fit text-[11px]" aria-label="Warm-up">W</Badge>
          <div className="min-w-0">
            <span className="text-xs text-muted-foreground">{t.label}{t.reps != null && ` × ${t.reps}`}</span>
            {plateLine(key, `${section.name} warm-up ${label}`, weightStr)}
          </div>
          <Input
            type="number" inputMode="decimal" enterKeyHint="done" step="0.5"
            value={weightStr}
            onChange={e => setDraft(key, 'weight', e.target.value)}
            placeholder="BW"
            className="h-11"
            aria-label={`${section.name} warm-up ${label} weight`}
          />
          <Input
            type="number" inputMode="numeric" enterKeyHint="done" min={1}
            value={repsStr}
            onChange={e => setDraft(key, 'reps', e.target.value)}
            className="h-11"
            aria-label={`${section.name} warm-up ${label} reps`}
          />
          <Button
            size="icon-touch"
            variant="outline"
            disabled={!canCheck}
            aria-label={`Log ${section.name} warm-up ${label}`}
            onClick={() => addSetMutation.mutate({
              exerciseId: section.exerciseId,
              exerciseName: section.name,
              rowKey: key,
              slotIndex: row.index,
              reps: repsNum,
              weight: weightStr ? parseFloat(weightStr) : undefined,
              restSeconds: t.rest_seconds,
              freeform: false,
              warmup: true,
              skipRest: false,
            })}
          ><Check aria-hidden="true" /></Button>
        </div>
        {t.notes && !sharedCue(section) && <p className="mt-1 text-xs text-muted-foreground">{t.notes}</p>}
      </div>
    )
  }

  function renderWorkingRow(
    section: Section,
    row: Row,
    opts: { label: string; name?: string; skipRest?: boolean; restSeconds?: number | null },
  ) {
    const key = `${section.exerciseId}:${row.index}`
    const prevDay = lastByExercise.get(section.exerciseId)
    const prevSet = prevDay ? workingSets(prevDay)[row.index] : undefined
    const showRpe = rpeShown[section.exerciseId] ?? false
    if (row.kind === 'done') return renderDoneRow(key, opts.label, row.set, { name: opts.name, exercise: section.name })

    // Pending row: inputs prefilled from the prescription; AMRAP targets start
    // empty and gate the check button. A rep range prefills its bottom.
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
    const setName = `${section.name} set ${opts.label}`
    return (
      <div key={key} className="px-3 py-2">
        <div className="grid grid-cols-[2.25rem_minmax(0,1fr)_4.5rem_3.75rem_2.75rem] items-center gap-2">
          <span className="text-sm tabular-nums text-muted-foreground">{opts.label}</span>
          <div className="min-w-0">
            {opts.name && <p className="truncate text-xs font-medium">{opts.name}</p>}
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
            aria-label={`${setName} weight`}
          />
          <Input
            type="number" inputMode="numeric" enterKeyHint="done" min={1}
            value={repsStr}
            onChange={e => setDraft(key, 'reps', e.target.value)}
            placeholder={prevSet ? String(prevSet.reps) : isAmrap && pset?.reps != null ? `${pset.reps}+` : undefined}
            required={isAmrap}
            className="h-11"
            aria-label={`${setName} reps${isAmrap ? ' (AMRAP — enter reps achieved)' : ''}`}
          />
          <Button
            size="icon-touch"
            variant={canCheck ? 'default' : 'outline'}
            disabled={!canCheck}
            aria-label={`Log ${setName}`}
            onClick={() => addSetMutation.mutate({
              exerciseId: section.exerciseId,
              exerciseName: section.name,
              rowKey: key,
              slotIndex: row.index,
              reps: repsNum,
              weight: weightStr ? parseFloat(weightStr) : undefined,
              rpe: rpeStr ? parseFloat(rpeStr) : undefined,
              repType: pset && pset.rep_type !== 'standard' ? pset.rep_type : undefined,
              restSeconds: opts.restSeconds !== undefined ? opts.restSeconds : pset?.rest_seconds,
              freeform: pset == null,
              warmup: false,
              skipRest: opts.skipRest ?? false,
            })}
          ><Check aria-hidden="true" /></Button>
        </div>
        {plateLine(key, setName, weightStr, prevSet?.weight != null && prevSet.weight >= emptyBar - 0.05)}
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
        {pset?.notes && !sharedCue(section) && (
          <p className="mt-1 text-xs text-muted-foreground">{pset.notes}</p>
        )}
      </div>
    )
  }

  /** Name row, last-session line, and Warm-ups / Swap actions for one exercise. */
  function renderExerciseHeader(section: Section, member?: string) {
    const prevDay = lastByExercise.get(section.exerciseId)
    const showRpe = rpeShown[section.exerciseId] ?? false
    const swap = section.swap
    const isPrescribed = section.prescribedExerciseId != null
    const hasLogged = section.logged.length + section.loggedWarmups.length > 0
    const canSwap = isPrescribed && !swap && !hasLogged
    const canRamp = isPrescribed && section.prescribedWarmups.length === 0 && section.logged.length === 0
    const generated = rampTargets[section.exerciseId]?.length ?? 0
    return (
      <div className="px-3 py-2">
        <div className="flex items-center justify-between gap-2">
          <div className="flex min-w-0 items-center gap-0.5">
            {member && <Badge variant="secondary" className="mr-1 shrink-0">{member}</Badge>}
            {member ? (
              <h3 className="min-w-0 truncate font-semibold">{section.name}</h3>
            ) : (
              <h2 className="min-w-0 truncate font-semibold">{section.name}</h2>
            )}
            <ExerciseCuesSheet name={section.name} exercise={exercisesById.get(section.exerciseId)} />
          </div>
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
        {swap && (
          <p className="flex flex-wrap items-center gap-x-2 text-xs text-muted-foreground">
            <span>Swapped from {swap.from_exercise_name}{swap.reason && ` · ${swap.reason}`}</span>
            {!hasLogged && (
              <Button
                variant="ghost"
                size="sm"
                className="h-11 px-1 text-xs md:h-7"
                disabled={undoSwapMutation.isPending}
                onClick={() => undoSwapMutation.mutate(swap.id)}
              >
                <Undo2 aria-hidden="true" /> Undo
              </Button>
            )}
          </p>
        )}
        {sharedCue(section) && <CueNote text={sharedCue(section)!} />}
        {prevDay && (
          <p className="text-xs text-muted-foreground">Last: {lastSummary(prevDay)}</p>
        )}
        {(canRamp || canSwap) && (
          <div className="-ml-2 mt-0.5 flex flex-wrap items-center gap-1">
            {canRamp && (
              <Button
                variant="ghost"
                size="sm"
                className="h-11 text-xs text-muted-foreground md:h-8"
                onClick={() => generated > 0
                  ? setRampTargets(r => ({ ...r, [section.exerciseId]: [] }))
                  : generateRamp(section)}
              >
                <Flame aria-hidden="true" /> {generated > 0 ? 'Clear warm-ups' : 'Warm-ups'}
              </Button>
            )}
            {canSwap && (
              <SwapExerciseSheet
                athleteId={athleteId}
                exerciseId={section.exerciseId}
                exerciseName={section.name}
                pending={swapMutation.isPending}
                onSwap={(to, reason) => swapMutation.mutate({ from: section.prescribedExerciseId ?? section.exerciseId, to, reason })}
              />
            )}
          </div>
        )}
      </div>
    )
  }

  function renderWarmupBlock(section: Section, member?: string) {
    const rows = buildWarmupRows(section, rampTargets[section.exerciseId] ?? [])
    if (rows.length === 0) return null
    return (
      <div className="border-t border-dashed border-border" role="group" aria-label={`${member ?? section.name} warm-ups`}>
        <p className="bg-muted/30 px-3 pt-2 text-[11px] font-medium tracking-wide text-muted-foreground uppercase">
          {member ? `${member} warm-ups` : 'Warm-ups'} · not counted
        </p>
        <div className="divide-y divide-dashed divide-border">
          {rows.map(r => renderWarmupRow(section, r, `${member ? `${member} ` : ''}${r.index + 1}`))}
        </div>
      </div>
    )
  }

  function addSetButton(section: Section, label = 'Add set') {
    return (
      <button
        type="button"
        onClick={() => setExtraPending(p => ({ ...p, [section.exerciseId]: (p[section.exerciseId] ?? 0) + 1 }))}
        className="flex h-11 w-full items-center justify-center gap-1 text-sm text-muted-foreground outline-none transition-colors hover:bg-muted/40 hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/50"
      >
        <Plus className="size-4" aria-hidden="true" /> {label}
      </button>
    )
  }

  function renderSingle(section: Section) {
    const rows = buildRows(section, claims, extraPending[section.exerciseId] ?? 0)
    return (
      <section key={section.exerciseId} className="overflow-hidden rounded-xl border border-border bg-card">
        <div className="border-b border-border bg-muted/50">{renderExerciseHeader(section)}</div>
        {renderWarmupBlock(section)}
        <div className={cn('divide-y divide-border', (section.prescribedWarmups.length > 0 || section.loggedWarmups.length > 0 || (rampTargets[section.exerciseId]?.length ?? 0) > 0) && 'border-t border-border')}>
          {rows.map(row => renderWorkingRow(section, row, { label: String(row.index + 1) }))}
        </div>
        <div className="border-t border-border">{addSetButton(section)}</div>
      </section>
    )
  }

  // Superset / circuit: one linked card, rows alternate member by member each
  // round (A1, A2, A1, A2, ...), and the rest timer runs once per round —
  // after the last member's set in that round.
  function renderGroup(key: string, members: Section[]) {
    const labels = members.map((_, i) => memberLabel(key, i))
    const memberRows = members.map(s => buildRows(s, claims, extraPending[s.exerciseId] ?? 0))
    const cells = interleaveRounds(memberRows)
    const kind = groupKind(members.length)
    return (
      <section key={`group-${key}`} className="overflow-hidden rounded-xl border-2 border-primary/30 bg-card" aria-label={`${kind} ${key}`}>
        <div className="flex items-center gap-1.5 border-b border-border bg-primary/5 px-3 py-1.5 text-xs font-medium text-primary">
          <Link2 className="size-3.5" aria-hidden="true" />
          {kind} {key} · alternate sets, rest after each round
        </div>
        <div className="divide-y divide-border border-b border-border bg-muted/50">
          {members.map((s, i) => <div key={s.exerciseId}>{renderExerciseHeader(s, labels[i])}</div>)}
        </div>
        {members.map((s, i) => <div key={`w-${s.exerciseId}`}>{renderWarmupBlock(s, labels[i])}</div>)}
        <div className="divide-y divide-border">
          {cells.map(({ member, round, row }) => {
            const section = members[member]
            const roundRest = Math.max(0, ...memberRows.map(rows => rows[round]?.prescribed?.rest_seconds ?? 0))
            return (
              <div key={`${section.exerciseId}:${row.index}`} className={cn(member === 0 && round > 0 && 'border-t-2 border-t-border')}>
                {renderWorkingRow(section, row, {
                  label: roundLabel(labels[member], round),
                  name: section.name,
                  skipRest: !roundComplete(memberRows, member, round),
                  restSeconds: roundRest > 0 ? roundRest : undefined,
                })}
              </div>
            )
          })}
        </div>
        <div className={cn('grid border-t border-border', members.length === 3 ? 'grid-cols-3' : 'grid-cols-2')}>
          {members.map((s, i) => <div key={s.exerciseId}>{addSetButton(s, `${labels[i]} set`)}</div>)}
        </div>
      </section>
    )
  }

  if (!data || rxLoading) {
    return (
      <div className="mx-auto max-w-2xl">
        <h1 className="sr-only">Session</h1>
        <QueryState
          query={rxLoading && data ? { data: undefined, error: null } : workoutQuery}
          resource="workout"
          skeleton={<PageSkeleton kind="detail" />}
          notFound={<EmptyState icon="🔍" title="Workout not found" description="It may have been deleted." action="All workouts" actionTo={`/athletes/${athleteId}/workouts`} />}
        >
          {() => null}
        </QueryState>
      </div>
    )
  }

  const { workout } = data
  // A 404 (or any prescription failure) degrades to freeform logging.
  const noProgram = !prescription

  // Warm-ups are excluded from "Set X of Y" and the progress bar.
  const { total: totalPrescribed, done: donePrescribed, logged: totalLogged } = sessionProgress(sections)
  const elapsedSec = Math.max(0, Math.floor((now - new Date(workout.created_at).getTime()) / 1000))
  const blocks = groupBlocks(sections, s => s.groupKey)

  // Ask the readiness questions once, when a fresh session for today opens.
  const isToday = workout.date.split('T')[0] === localDateISO()
  const showReadiness = isToday && !data.readiness && !readinessDismissed && totalLogged === 0
    && sections.every(s => s.loggedWarmups.length === 0)

  return (
    <div className={cn('mx-auto max-w-2xl', restTimer.running ? 'pb-36' : 'pb-8')}>
      {/* Sticky session header — sits below the fixed mobile nav and the iOS
          status bar inset (--app-header-offset). */}
      <h1 className="sr-only">Session · {friendlyDate(workout.date)}</h1>
      <div className="sticky top-(--app-header-offset) z-20 -mx-4 mb-4 border-b border-border bg-background/95 px-4 py-2 backdrop-blur md:top-0 md:-mx-6 md:px-6">
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
            aria-label="Working sets completed"
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
          {prescription.program_name} · Week {prescription.current_week}, Day {prescription.current_day}
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
          {blocks.map(block => block.key && block.items.length >= 2
            ? renderGroup(block.key, block.items)
            : block.items.map(renderSingle))}
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

      <ReadinessPrompt
        open={showReadiness}
        pending={readinessMutation.isPending}
        onSubmit={answers => readinessMutation.mutate(answers)}
        onSkip={dismissReadiness}
      />
      <RestTimerBar timer={restTimer} />
      {confirmDialog()}
    </div>
  )
}
