import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { api, ApiError } from '@/api/client'
import type { ThrowingSessionInput } from '@/api/client'
import { Spinner } from '@/components/ui'
import { useConfirm } from '@/lib/useConfirm'
import { EnumSelect } from '@/components/EnumSelect'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Checkbox } from '@/components/ui/checkbox'
import { Badge } from '@/components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

const THROW_TYPES = [
  { value: 'game', label: 'Game' },
  { value: 'bullpen', label: 'Bullpen' },
  { value: 'lesson', label: 'Lesson' },
  { value: 'long_toss', label: 'Long toss' },
  { value: 'catch', label: 'Catch' },
  { value: 'flat_ground', label: 'Flat ground' },
  { value: 'position', label: 'Position' },
]

const SOURCES = [
  { value: 'program', label: 'Program' },
  { value: 'external', label: 'External' },
]

function num(v: string): number | undefined {
  if (v.trim() === '') return undefined
  const n = Number(v)
  return isNaN(n) ? undefined : n
}

export function ThrowingSessions() {
  const { id } = useParams<{ id: string }>()
  const athleteId = Number(id)
  const queryClient = useQueryClient()
  const { confirm, dialog: confirmDialog } = useConfirm()

  const [showForm, setShowForm] = useState(false)
  const [date, setDate] = useState(new Date().toISOString().slice(0, 10))
  const [throwType, setThrowType] = useState('bullpen')
  const [throwCount, setThrowCount] = useState('')
  const [maxIntent, setMaxIntent] = useState('')
  const [velocity, setVelocity] = useState('')
  const [fatigue, setFatigue] = useState(false)
  const [pain, setPain] = useState(false)
  const [source, setSource] = useState('program')
  const [team, setTeam] = useState('')
  const [notes, setNotes] = useState('')

  const { data: athlete } = useQuery({
    queryKey: ['athlete', athleteId],
    queryFn: () => api.getAthlete(athleteId),
    enabled: !isNaN(athleteId),
  })

  const { data: sessions, isLoading, error } = useQuery({
    queryKey: ['throwing-sessions', athleteId],
    queryFn: () => api.listThrowingSessions(athleteId),
    enabled: !isNaN(athleteId),
  })

  function resetForm() {
    setDate(new Date().toISOString().slice(0, 10))
    setThrowType('bullpen')
    setThrowCount('')
    setMaxIntent('')
    setVelocity('')
    setFatigue(false)
    setPain(false)
    setSource('program')
    setTeam('')
    setNotes('')
  }

  const createMutation = useMutation({
    mutationFn: () => {
      const data: ThrowingSessionInput = {
        date,
        throw_type: throwType,
        throw_count: num(throwCount),
        max_intent: num(maxIntent),
        velocity: num(velocity),
        fatigue,
        pain,
        source,
        team: team.trim() || undefined,
        notes: notes.trim() || undefined,
      }
      return api.createThrowingSession(athleteId, data)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['throwing-sessions', athleteId] })
      resetForm()
      setShowForm(false)
    },
    onError: (err) => toast.error(err instanceof ApiError ? err.message : 'Failed to log throwing session'),
  })

  const deleteMutation = useMutation({
    mutationFn: (sessionId: number) => api.deleteThrowingSession(athleteId, sessionId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['throwing-sessions', athleteId] }),
    onError: (err) => toast.error(err instanceof ApiError ? err.message : 'Failed to delete throwing session'),
  })

  async function handleDelete(sessionId: number) {
    if (await confirm({ title: 'Delete throwing session?', description: 'This removes the logged session and its parent workout.', variant: 'danger', confirmLabel: 'Delete' })) {
      deleteMutation.mutate(sessionId)
    }
  }

  if (isLoading) return <Spinner />
  if (error) return <p className="text-destructive">Failed to load throwing sessions.</p>

  return (
    <div>
      {confirmDialog()}
      <div className="flex items-center justify-between mb-6">
        <div>
          <p className="text-sm text-muted-foreground">
            <Link to={`/athletes/${athleteId}`} className="hover:text-foreground">
              {athlete?.name ?? 'Athlete'}
            </Link>
            {' / Throwing'}
          </p>
          <h1 className="text-2xl font-bold">Throwing</h1>
        </div>
        <Button onClick={() => setShowForm(s => !s)}>{showForm ? 'Cancel' : '+ Log Throwing'}</Button>
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
          <div>
            <Label>Throw type</Label>
            <EnumSelect value={throwType} onChange={setThrowType} options={THROW_TYPES} required />
          </div>
          <div className="grid grid-cols-3 gap-3">
            <div>
              <Label htmlFor="throw_count">Throw count</Label>
              <Input id="throw_count" type="number" min="0" value={throwCount} onChange={e => setThrowCount(e.target.value)} />
            </div>
            <div>
              <Label htmlFor="max_intent">Max intent %</Label>
              <Input id="max_intent" type="number" min="0" max="100" value={maxIntent} onChange={e => setMaxIntent(e.target.value)} />
            </div>
            <div>
              <Label htmlFor="velocity">Velocity</Label>
              <Input id="velocity" type="number" step="0.1" min="0" value={velocity} onChange={e => setVelocity(e.target.value)} />
            </div>
          </div>
          <div className="flex gap-6">
            <Label className="flex items-center gap-2">
              <Checkbox checked={fatigue} onCheckedChange={(c) => setFatigue(c)} />
              Fatigue
            </Label>
            <Label className="flex items-center gap-2">
              <Checkbox checked={pain} onCheckedChange={(c) => setPain(c)} />
              Pain
            </Label>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <Label>Source</Label>
              <EnumSelect value={source} onChange={setSource} options={SOURCES} required />
            </div>
            <div>
              <Label htmlFor="team">Team</Label>
              <Input id="team" value={team} onChange={e => setTeam(e.target.value)} placeholder="Optional" />
            </div>
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
        <p className="text-muted-foreground">No throwing sessions logged yet.</p>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Date</TableHead>
              <TableHead>Type</TableHead>
              <TableHead>Throws</TableHead>
              <TableHead>Velo</TableHead>
              <TableHead>Flags</TableHead>
              <TableHead></TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {sessions?.map(s => (
              <TableRow key={s.id}>
                <TableCell className="font-medium">{s.date}</TableCell>
                <TableCell>{s.throw_type}</TableCell>
                <TableCell>{s.throw_count ?? '—'}</TableCell>
                <TableCell>{s.velocity ?? '—'}</TableCell>
                <TableCell className="space-x-1">
                  {s.fatigue && <Badge variant="secondary">fatigue</Badge>}
                  {s.pain && <Badge variant="destructive">pain</Badge>}
                </TableCell>
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
