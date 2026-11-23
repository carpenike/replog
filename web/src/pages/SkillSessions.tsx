import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { api, ApiError } from '@/api/client'
import type { SkillSessionInput } from '@/api/client'
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
  const [date, setDate] = useState(localDateISO())
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

  const friendlyDate = useFriendlyDate()
  const listQuery = useQuery({
    queryKey: ['skill-sessions', athleteId],
    queryFn: () => api.listSkillSessions(athleteId),
    enabled: !isNaN(athleteId),
  })

  function resetForm() {
    setDate(localDateISO())
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


  return (
    <div>
      {confirmDialog()}
      <PageHeader
        title="Skill"
        back={{ to: `/athletes/${athleteId}`, label: athlete?.name ?? 'Athlete' }}
        actions={
          <Button variant={showForm ? 'ghost' : 'default'} onClick={() => setShowForm(s => !s)} aria-expanded={showForm}>
            {showForm ? <><X aria-hidden="true" /> Cancel</> : <><Plus aria-hidden="true" /> Log Skill</>}
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
            <Label htmlFor="skill-skill-type">Skill type <span aria-hidden="true" className="text-destructive">*</span></Label>
            <EnumSelect id="skill-skill-type" value={skillType} onChange={setSkillType} options={SKILL_TYPES} required />
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

      <QueryState
        query={listQuery}
        resource="skill sessions"
        empty={{ icon: '🎯', title: 'No skill sessions logged yet', description: 'Batting, fielding, sprints and other skill work show up here.', action: !showForm ? 'Log Skill' : undefined, onAction: () => setShowForm(true) }}
      >
        {sessions => (
          <>
            <CardList label="Skill">
              {sessions.map(s => (
                <CardListItem
                  key={s.id}
                  title={friendlyDate(s.date)}
                  subtitle={SKILL_TYPES.find(t => t.value === s.skill_type)?.label ?? s.skill_type}
                  trailing={[s.rep_count != null ? `${s.rep_count} reps` : null, s.load_kg != null ? `${s.load_kg} kg` : null].filter(Boolean).join(' · ') || undefined}
                  actions={<Button variant="ghost" size="sm" aria-label={`Delete ${'skill session ' + friendlyDate(s.date)}`} className="text-muted-foreground hover:text-destructive" onClick={() => handleDelete(s.id)}><Trash2 aria-hidden="true" /> Delete</Button>}
                />
              ))}
            </CardList>
            <DesktopOnly>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Date</TableHead>
                  <TableHead>Skill</TableHead>
                  <TableHead>Reps</TableHead>
                  <TableHead>Load</TableHead>
                  <TableHead><span className="sr-only">Actions</span></TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {sessions.map(s => (
                  <TableRow key={s.id}>
                    <TableCell className="font-medium">{friendlyDate(s.date)}</TableCell>
                    <TableCell>{s.skill_type}</TableCell>
                    <TableCell>{s.rep_count ?? '—'}</TableCell>
                    <TableCell>{s.load_kg ?? '—'}</TableCell>
                    <TableCell className="text-right">
                      <Button variant="ghost" size="icon-sm" aria-label={`Delete skill session ${friendlyDate(s.date)}`} className="text-muted-foreground hover:text-destructive" onClick={() => handleDelete(s.id)}><Trash2 aria-hidden="true" /></Button>
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
