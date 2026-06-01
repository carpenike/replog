import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { api, ApiError } from '@/api/client'
import type { SkillSessionInput } from '@/api/client'
import { Spinner } from '@/components/ui'
import { useConfirm } from '@/lib/useConfirm'
import { EnumSelect } from '@/components/EnumSelect'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

const SKILL_TYPES = [
  { value: 'batting', label: 'Batting' },
  { value: 'fielding', label: 'Fielding' },
  { value: 'throwing_accuracy', label: 'Throwing accuracy' },
  { value: 'agility', label: 'Agility' },
  { value: 'medball', label: 'Med ball' },
  { value: 'sprint', label: 'Sprint' },
  { value: 'other', label: 'Other' },
]

function num(v: string): number | undefined {
  if (v.trim() === '') return undefined
  const n = Number(v)
  return isNaN(n) ? undefined : n
}

export function SkillSessions() {
  const { id } = useParams<{ id: string }>()
  const athleteId = Number(id)
  const queryClient = useQueryClient()
  const { confirm, dialog: confirmDialog } = useConfirm()

  const [showForm, setShowForm] = useState(false)
  const [date, setDate] = useState(new Date().toISOString().slice(0, 10))
  const [skillType, setSkillType] = useState('batting')
  const [repCount, setRepCount] = useState('')
  const [loadKg, setLoadKg] = useState('')
  const [velocity, setVelocity] = useState('')
  const [durationSeconds, setDurationSeconds] = useState('')
  const [notes, setNotes] = useState('')

  const { data: athlete } = useQuery({
    queryKey: ['athlete', athleteId],
    queryFn: () => api.getAthlete(athleteId),
    enabled: !isNaN(athleteId),
  })

  const { data: sessions, isLoading, error } = useQuery({
    queryKey: ['skill-sessions', athleteId],
    queryFn: () => api.listSkillSessions(athleteId),
    enabled: !isNaN(athleteId),
  })

  function resetForm() {
    setDate(new Date().toISOString().slice(0, 10))
    setSkillType('batting')
    setRepCount('')
    setLoadKg('')
    setVelocity('')
    setDurationSeconds('')
    setNotes('')
  }

  const createMutation = useMutation({
    mutationFn: () => {
      const data: SkillSessionInput = {
        date,
        skill_type: skillType,
        rep_count: num(repCount),
        load_kg: num(loadKg),
        velocity: num(velocity),
        duration_seconds: num(durationSeconds),
        notes: notes.trim() || undefined,
      }
      return api.createSkillSession(athleteId, data)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['skill-sessions', athleteId] })
      resetForm()
      setShowForm(false)
    },
    onError: (err) => toast.error(err instanceof ApiError ? err.message : 'Failed to log skill session'),
  })

  const deleteMutation = useMutation({
    mutationFn: (sessionId: number) => api.deleteSkillSession(athleteId, sessionId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['skill-sessions', athleteId] }),
    onError: (err) => toast.error(err instanceof ApiError ? err.message : 'Failed to delete skill session'),
  })

  async function handleDelete(sessionId: number) {
    if (await confirm({ title: 'Delete skill session?', description: 'This removes the logged session and its parent workout.', variant: 'danger', confirmLabel: 'Delete' })) {
      deleteMutation.mutate(sessionId)
    }
  }

  if (isLoading) return <Spinner />
  if (error) return <p className="text-destructive">Failed to load skill sessions.</p>

  return (
    <div>
      {confirmDialog()}
      <div className="flex items-center justify-between mb-6">
        <div>
          <p className="text-sm text-muted-foreground">
            <Link to={`/athletes/${athleteId}`} className="hover:text-foreground">
              {athlete?.name ?? 'Athlete'}
            </Link>
            {' / Skill'}
          </p>
          <h1 className="text-2xl font-bold">Skill</h1>
        </div>
        <Button onClick={() => setShowForm(s => !s)}>{showForm ? 'Cancel' : '+ Log Skill'}</Button>
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
            <Label>Skill type</Label>
            <EnumSelect value={skillType} onChange={setSkillType} options={SKILL_TYPES} required />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <Label htmlFor="rep_count">Reps</Label>
              <Input id="rep_count" type="number" min="0" value={repCount} onChange={e => setRepCount(e.target.value)} />
            </div>
            <div>
              <Label htmlFor="load_kg">Load (kg)</Label>
              <Input id="load_kg" type="number" step="0.1" min="0" value={loadKg} onChange={e => setLoadKg(e.target.value)} />
            </div>
            <div>
              <Label htmlFor="velocity">Velocity</Label>
              <Input id="velocity" type="number" step="0.1" min="0" value={velocity} onChange={e => setVelocity(e.target.value)} />
            </div>
            <div>
              <Label htmlFor="duration_seconds">Duration (s)</Label>
              <Input id="duration_seconds" type="number" min="0" value={durationSeconds} onChange={e => setDurationSeconds(e.target.value)} />
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
        <p className="text-muted-foreground">No skill sessions logged yet.</p>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Date</TableHead>
              <TableHead>Skill</TableHead>
              <TableHead>Reps</TableHead>
              <TableHead>Load</TableHead>
              <TableHead></TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {sessions?.map(s => (
              <TableRow key={s.id}>
                <TableCell className="font-medium">{s.date}</TableCell>
                <TableCell>{s.skill_type}</TableCell>
                <TableCell>{s.rep_count ?? '—'}</TableCell>
                <TableCell>{s.load_kg ?? '—'}</TableCell>
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
