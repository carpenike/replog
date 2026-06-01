import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { api, ApiError } from '@/api/client'
import type { ConditioningSessionInput, ConditioningIntervalInput } from '@/api/client'
import { Spinner } from '@/components/ui'
import { useConfirm } from '@/lib/useConfirm'
import { EnumSelect } from '@/components/EnumSelect'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

const MODALITIES = [
  { value: 'run', label: 'Run' },
  { value: 'row', label: 'Row' },
  { value: 'bike', label: 'Bike' },
  { value: 'sprint', label: 'Sprint' },
  { value: 'circuit', label: 'Circuit' },
  { value: 'swim', label: 'Swim' },
  { value: 'other', label: 'Other' },
]

const SESSION_TYPES = [
  { value: 'steady', label: 'Steady' },
  { value: 'interval', label: 'Interval' },
  { value: 'sprint', label: 'Sprint' },
  { value: 'tempo', label: 'Tempo' },
]

const DISTANCE_UNITS = [
  { value: 'm', label: 'm' },
  { value: 'km', label: 'km' },
  { value: 'yd', label: 'yd' },
  { value: 'mi', label: 'mi' },
]

function num(v: string): number | undefined {
  if (v.trim() === '') return undefined
  const n = Number(v)
  return isNaN(n) ? undefined : n
}

interface IntervalRow {
  work_seconds: string
  work_distance: string
  rest_seconds: string
  notes: string
}

const emptyInterval: IntervalRow = { work_seconds: '', work_distance: '', rest_seconds: '', notes: '' }

export function ConditioningSessions() {
  const { id } = useParams<{ id: string }>()
  const athleteId = Number(id)
  const queryClient = useQueryClient()
  const { confirm, dialog: confirmDialog } = useConfirm()

  const [showForm, setShowForm] = useState(false)
  const [date, setDate] = useState(new Date().toISOString().slice(0, 10))
  const [modality, setModality] = useState('run')
  const [sessionType, setSessionType] = useState('steady')
  const [totalDistance, setTotalDistance] = useState('')
  const [distanceUnit, setDistanceUnit] = useState('m')
  const [durationSeconds, setDurationSeconds] = useState('')
  const [avgHr, setAvgHr] = useState('')
  const [rpe, setRpe] = useState('')
  const [notes, setNotes] = useState('')
  const [intervals, setIntervals] = useState<IntervalRow[]>([])

  const { data: athlete } = useQuery({
    queryKey: ['athlete', athleteId],
    queryFn: () => api.getAthlete(athleteId),
    enabled: !isNaN(athleteId),
  })

  const { data: sessions, isLoading, error } = useQuery({
    queryKey: ['conditioning-sessions', athleteId],
    queryFn: () => api.listConditioningSessions(athleteId),
    enabled: !isNaN(athleteId),
  })

  function resetForm() {
    setDate(new Date().toISOString().slice(0, 10))
    setModality('run')
    setSessionType('steady')
    setTotalDistance('')
    setDistanceUnit('m')
    setDurationSeconds('')
    setAvgHr('')
    setRpe('')
    setNotes('')
    setIntervals([])
  }

  const createMutation = useMutation({
    mutationFn: () => {
      const intervalInputs: ConditioningIntervalInput[] = intervals.map((iv, idx) => ({
        interval_number: idx + 1,
        work_seconds: num(iv.work_seconds),
        work_distance: num(iv.work_distance),
        rest_seconds: num(iv.rest_seconds),
        notes: iv.notes.trim() || undefined,
      }))
      const data: ConditioningSessionInput = {
        date,
        modality,
        session_type: sessionType,
        total_distance: num(totalDistance),
        distance_unit: totalDistance.trim() ? distanceUnit : undefined,
        duration_seconds: num(durationSeconds),
        avg_hr: num(avgHr),
        rpe: num(rpe),
        notes: notes.trim() || undefined,
        intervals: intervalInputs.length ? intervalInputs : undefined,
      }
      return api.createConditioningSession(athleteId, data)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['conditioning-sessions', athleteId] })
      resetForm()
      setShowForm(false)
    },
    onError: (err) => toast.error(err instanceof ApiError ? err.message : 'Failed to log conditioning session'),
  })

  const deleteMutation = useMutation({
    mutationFn: (sessionId: number) => api.deleteConditioningSession(athleteId, sessionId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['conditioning-sessions', athleteId] }),
    onError: (err) => toast.error(err instanceof ApiError ? err.message : 'Failed to delete conditioning session'),
  })

  async function handleDelete(sessionId: number) {
    if (await confirm({ title: 'Delete conditioning session?', description: 'This removes the logged session and its parent workout.', variant: 'danger', confirmLabel: 'Delete' })) {
      deleteMutation.mutate(sessionId)
    }
  }

  function updateInterval(idx: number, field: keyof IntervalRow, value: string) {
    setIntervals(prev => prev.map((iv, i) => (i === idx ? { ...iv, [field]: value } : iv)))
  }

  if (isLoading) return <Spinner />
  if (error) return <p className="text-destructive">Failed to load conditioning sessions.</p>

  return (
    <div>
      {confirmDialog()}
      <div className="flex items-center justify-between mb-6">
        <div>
          <p className="text-sm text-muted-foreground">
            <Link to={`/athletes/${athleteId}`} className="hover:text-foreground">
              {athlete?.name ?? 'Athlete'}
            </Link>
            {' / Conditioning'}
          </p>
          <h1 className="text-2xl font-bold">Conditioning</h1>
        </div>
        <Button onClick={() => setShowForm(s => !s)}>{showForm ? 'Cancel' : '+ Log Conditioning'}</Button>
      </div>

      {showForm && (
        <form
          onSubmit={(e) => { e.preventDefault(); createMutation.mutate() }}
          className="space-y-4 max-w-lg mb-8 rounded-lg border p-4"
        >
          <div>
            <Label htmlFor="date">Date</Label>
            <Input id="date" type="date" value={date} onChange={e => setDate(e.target.value)} required />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <Label>Modality</Label>
              <EnumSelect value={modality} onChange={setModality} options={MODALITIES} required />
            </div>
            <div>
              <Label>Session type</Label>
              <EnumSelect value={sessionType} onChange={setSessionType} options={SESSION_TYPES} required />
            </div>
          </div>
          <div className="grid grid-cols-3 gap-3">
            <div>
              <Label htmlFor="total_distance">Distance</Label>
              <Input id="total_distance" type="number" step="0.1" min="0" value={totalDistance} onChange={e => setTotalDistance(e.target.value)} />
            </div>
            <div>
              <Label>Unit</Label>
              <EnumSelect value={distanceUnit} onChange={setDistanceUnit} options={DISTANCE_UNITS} />
            </div>
            <div>
              <Label htmlFor="duration_seconds">Duration (s)</Label>
              <Input id="duration_seconds" type="number" min="0" value={durationSeconds} onChange={e => setDurationSeconds(e.target.value)} />
            </div>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <Label htmlFor="avg_hr">Avg HR</Label>
              <Input id="avg_hr" type="number" min="0" value={avgHr} onChange={e => setAvgHr(e.target.value)} />
            </div>
            <div>
              <Label htmlFor="rpe">RPE</Label>
              <Input id="rpe" type="number" step="0.5" min="0" max="10" value={rpe} onChange={e => setRpe(e.target.value)} />
            </div>
          </div>

          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <Label>Intervals</Label>
              <Button type="button" variant="outline" size="sm" onClick={() => setIntervals(prev => [...prev, { ...emptyInterval }])}>
                + Add interval
              </Button>
            </div>
            {intervals.map((iv, idx) => (
              <div key={idx} className="grid grid-cols-[auto_1fr_1fr_1fr_auto] items-center gap-2">
                <span className="text-sm text-muted-foreground w-6">#{idx + 1}</span>
                <Input type="number" min="0" placeholder="Work s" value={iv.work_seconds} onChange={e => updateInterval(idx, 'work_seconds', e.target.value)} />
                <Input type="number" step="0.1" min="0" placeholder="Work dist" value={iv.work_distance} onChange={e => updateInterval(idx, 'work_distance', e.target.value)} />
                <Input type="number" min="0" placeholder="Rest s" value={iv.rest_seconds} onChange={e => updateInterval(idx, 'rest_seconds', e.target.value)} />
                <Button type="button" variant="ghost" size="sm" onClick={() => setIntervals(prev => prev.filter((_, i) => i !== idx))}>✕</Button>
              </div>
            ))}
          </div>

          <div>
            <Label htmlFor="notes">Notes</Label>
            <Textarea id="notes" value={notes} onChange={e => setNotes(e.target.value)} rows={2} placeholder="Optional notes..." />
          </div>
          <Button type="submit" disabled={createMutation.isPending}>
            {createMutation.isPending ? 'Saving...' : 'Save'}
          </Button>
        </form>
      )}

      {sessions && sessions.length === 0 ? (
        <p className="text-muted-foreground">No conditioning sessions logged yet.</p>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Date</TableHead>
              <TableHead>Modality</TableHead>
              <TableHead>Type</TableHead>
              <TableHead>Distance</TableHead>
              <TableHead>Intervals</TableHead>
              <TableHead></TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {sessions?.map(s => (
              <TableRow key={s.id}>
                <TableCell className="font-medium">{s.date}</TableCell>
                <TableCell>{s.modality}</TableCell>
                <TableCell>{s.session_type}</TableCell>
                <TableCell>{s.total_distance != null ? `${s.total_distance} ${s.distance_unit ?? ''}` : '—'}</TableCell>
                <TableCell>{s.intervals.length || '—'}</TableCell>
                <TableCell className="text-right">
                  <Button variant="ghost" size="sm" onClick={() => handleDelete(s.id)}>Delete</Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </div>
  )
}
