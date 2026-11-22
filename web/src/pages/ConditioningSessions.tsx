import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Plus, Trash2, X } from 'lucide-react'
import { api, ApiError } from '@/api/client'
import type { ConditioningSessionInput, ConditioningIntervalInput } from '@/api/client'
import { PageHeader } from '@/components/PageHeader'
import { QueryState } from '@/components/QueryState'
import { CardList, CardListItem, DesktopOnly } from '@/components/CardList'
import { useFriendlyDate } from '@/lib/dates'
import { localDateISO } from '@/lib/utils'
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
  const [date, setDate] = useState(localDateISO())
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

  const friendlyDate = useFriendlyDate()
  const listQuery = useQuery({
    queryKey: ['conditioning-sessions', athleteId],
    queryFn: () => api.listConditioningSessions(athleteId),
    enabled: !isNaN(athleteId),
  })

  function resetForm() {
    setDate(localDateISO())
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


  return (
    <div>
      {confirmDialog()}
      <PageHeader
        title="Conditioning"
        back={{ to: `/athletes/${athleteId}`, label: athlete?.name ?? 'Athlete' }}
        actions={
          <Button variant={showForm ? 'ghost' : 'default'} onClick={() => setShowForm(s => !s)} aria-expanded={showForm}>
            {showForm ? <><X aria-hidden="true" /> Cancel</> : <><Plus aria-hidden="true" /> Log Conditioning</>}
          </Button>
        }
      />

      {showForm && (
        <form
          onSubmit={(e) => { e.preventDefault(); createMutation.mutate() }}
          className="space-y-4 max-w-lg mb-8 rounded-lg border p-4"
        >
          <div>
            <Label htmlFor="date">Date <span aria-hidden="true" className="text-destructive">*</span></Label>
            <Input id="date" type="date" value={date} onChange={e => setDate(e.target.value)} required />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <Label htmlFor="cond-modality">Modality <span aria-hidden="true" className="text-destructive">*</span></Label>
              <EnumSelect id="cond-modality" value={modality} onChange={setModality} options={MODALITIES} required />
            </div>
            <div>
              <Label htmlFor="cond-session-type">Session type <span aria-hidden="true" className="text-destructive">*</span></Label>
              <EnumSelect id="cond-session-type" value={sessionType} onChange={setSessionType} options={SESSION_TYPES} required />
            </div>
          </div>
          <div className="grid grid-cols-3 gap-3">
            <div>
              <Label htmlFor="total_distance">Distance</Label>
              <Input id="total_distance" type="number" step="0.1" min="0" value={totalDistance} onChange={e => setTotalDistance(e.target.value)} />
            </div>
            <div>
              <Label htmlFor="cond-unit">Unit</Label>
              <EnumSelect id="cond-unit" value={distanceUnit} onChange={setDistanceUnit} options={DISTANCE_UNITS} />
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
              <h2 id="cond-intervals" className="text-sm font-medium">Intervals</h2>
              <Button type="button" variant="outline" size="sm" onClick={() => setIntervals(prev => [...prev, { ...emptyInterval }])}>
                <Plus aria-hidden="true" /> Add interval
              </Button>
            </div>
            {intervals.map((iv, idx) => (
              <div key={idx} className="grid grid-cols-[auto_1fr_1fr_1fr_auto] items-center gap-2">
                <span className="text-sm text-muted-foreground w-6">#{idx + 1}</span>
                <Input type="number" inputMode="numeric" min="0" aria-label={`Interval ${idx + 1} work seconds`} placeholder="Work s" value={iv.work_seconds} onChange={e => updateInterval(idx, 'work_seconds', e.target.value)} />
                <Input type="number" inputMode="decimal" step="0.1" min="0" aria-label={`Interval ${idx + 1} work distance`} placeholder="Work dist" value={iv.work_distance} onChange={e => updateInterval(idx, 'work_distance', e.target.value)} />
                <Input type="number" inputMode="numeric" min="0" aria-label={`Interval ${idx + 1} rest seconds`} placeholder="Rest s" value={iv.rest_seconds} onChange={e => updateInterval(idx, 'rest_seconds', e.target.value)} />
                <Button type="button" variant="ghost" size="icon-sm" aria-label={`Remove interval ${idx + 1}`} onClick={() => setIntervals(prev => prev.filter((_, i) => i !== idx))}>
                  <X aria-hidden="true" />
                </Button>
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

      <QueryState
        query={listQuery}
        resource="conditioning sessions"
        empty={{ icon: '🏃', title: 'No conditioning sessions logged yet', description: 'Runs, rows, rides and interval work show up here.', action: !showForm ? 'Log Conditioning' : undefined, onAction: () => setShowForm(true) }}
      >
        {sessions => (
          <>
            <CardList label="Conditioning">
              {sessions.map(s => (
                <CardListItem
                  key={s.id}
                  title={friendlyDate(s.date)}
                  subtitle={`${s.modality} · ${s.session_type}`}
                  trailing={[s.total_distance != null ? `${s.total_distance} ${s.distance_unit ?? ''}`.trim() : null, s.intervals.length ? `${s.intervals.length} intervals` : null].filter(Boolean).join(' · ') || undefined}
                  actions={<Button variant="ghost" size="sm" aria-label={`Delete ${'conditioning session ' + friendlyDate(s.date)}`} className="text-muted-foreground hover:text-destructive" onClick={() => handleDelete(s.id)}><Trash2 aria-hidden="true" /> Delete</Button>}
                />
              ))}
            </CardList>
            <DesktopOnly>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Date</TableHead>
                  <TableHead>Modality</TableHead>
                  <TableHead>Type</TableHead>
                  <TableHead>Distance</TableHead>
                  <TableHead>Intervals</TableHead>
                  <TableHead><span className="sr-only">Actions</span></TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {sessions.map(s => (
                  <TableRow key={s.id}>
                    <TableCell className="font-medium">{friendlyDate(s.date)}</TableCell>
                    <TableCell>{s.modality}</TableCell>
                    <TableCell>{s.session_type}</TableCell>
                    <TableCell>{s.total_distance != null ? `${s.total_distance} ${s.distance_unit ?? ''}` : '—'}</TableCell>
                    <TableCell>{s.intervals.length || '—'}</TableCell>
                    <TableCell className="text-right">
                      <Button variant="ghost" size="icon-sm" aria-label={`Delete conditioning session ${friendlyDate(s.date)}`} className="text-muted-foreground hover:text-destructive" onClick={() => handleDelete(s.id)}><Trash2 aria-hidden="true" /></Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
        </Table>
            </DesktopOnly>
          </>
        )}
      </QueryState>
    </div>
  )
}
