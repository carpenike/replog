import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { api, ApiError } from '@/api/client'
import type { ThrowingSessionInput } from '@/api/client'
import { Plus, Trash2, X } from 'lucide-react'
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
  const [date, setDate] = useState(localDateISO())
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

  const friendlyDate = useFriendlyDate()
  const listQuery = useQuery({
    queryKey: ['throwing-sessions', athleteId],
    queryFn: () => api.listThrowingSessions(athleteId),
    enabled: !isNaN(athleteId),
  })

  function resetForm() {
    setDate(localDateISO())
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


  return (
    <div>
      {confirmDialog()}
      <PageHeader
        title="Throwing"
        back={{ to: `/athletes/${athleteId}`, label: athlete?.name ?? 'Athlete' }}
        actions={
          <Button variant={showForm ? 'ghost' : 'default'} onClick={() => setShowForm(s => !s)} aria-expanded={showForm}>
            {showForm ? <><X aria-hidden="true" /> Cancel</> : <><Plus aria-hidden="true" /> Log Throwing</>}
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
          <div>
            <Label htmlFor="throw-throw-type">Throw type <span aria-hidden="true" className="text-destructive">*</span></Label>
            <EnumSelect id="throw-throw-type" value={throwType} onChange={setThrowType} options={THROW_TYPES} required />
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
            <Label className="flex min-h-11 items-center gap-2 md:min-h-0">
              <Checkbox checked={fatigue} onCheckedChange={(c) => setFatigue(c)} />
              Fatigue
            </Label>
            <Label className="flex min-h-11 items-center gap-2 md:min-h-0">
              <Checkbox checked={pain} onCheckedChange={(c) => setPain(c)} />
              Pain
            </Label>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <Label htmlFor="throw-source">Source <span aria-hidden="true" className="text-destructive">*</span></Label>
              <EnumSelect id="throw-source" value={source} onChange={setSource} options={SOURCES} required />
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

      <QueryState
        query={listQuery}
        resource="throwing sessions"
        empty={{ icon: '⚾', title: 'No throwing sessions logged yet', description: 'Log throws to track volume and Pitch Smart rest.', action: !showForm ? 'Log Throwing' : undefined, onAction: () => setShowForm(true) }}
      >
        {sessions => (
          <>
            <CardList label="Throwing">
              {sessions.map(s => (
                <CardListItem
                  key={s.id}
                  title={friendlyDate(s.date)}
                  subtitle={[s.throw_type, s.throw_count != null ? `${s.throw_count} throws` : null, s.velocity != null ? `${s.velocity} velo` : null].filter(Boolean).join(' · ')}
                  trailing={(s.fatigue || s.pain) ? <span className="inline-flex gap-1">{s.fatigue && <Badge variant="secondary">fatigue</Badge>}{s.pain && <Badge variant="destructive">pain</Badge>}</span> : undefined}
                  actions={<Button variant="ghost" size="sm" aria-label={`Delete ${'throwing session ' + friendlyDate(s.date)}`} className="text-muted-foreground hover:text-destructive" onClick={() => handleDelete(s.id)}><Trash2 aria-hidden="true" /> Delete</Button>}
                />
              ))}
            </CardList>
            <DesktopOnly>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Date</TableHead>
                  <TableHead>Type</TableHead>
                  <TableHead>Throws</TableHead>
                  <TableHead>Velo</TableHead>
                  <TableHead>Flags</TableHead>
                  <TableHead><span className="sr-only">Actions</span></TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {sessions.map(s => (
                  <TableRow key={s.id}>
                    <TableCell className="font-medium">{friendlyDate(s.date)}</TableCell>
                    <TableCell>{s.throw_type}</TableCell>
                    <TableCell>{s.throw_count ?? '—'}</TableCell>
                    <TableCell>{s.velocity ?? '—'}</TableCell>
                    <TableCell className="space-x-1">
                      {s.fatigue && <Badge variant="secondary">fatigue</Badge>}
                      {s.pain && <Badge variant="destructive">pain</Badge>}
                    </TableCell>
                    <TableCell className="text-right">
                      <Button variant="ghost" size="icon-sm" aria-label={`Delete throwing session ${friendlyDate(s.date)}`} className="text-muted-foreground hover:text-destructive" onClick={() => handleDelete(s.id)}><Trash2 aria-hidden="true" /></Button>
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
