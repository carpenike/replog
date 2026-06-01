import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { api, ApiError } from '@/api/client'
import type { SeasonPhaseInput } from '@/api/client'
import { Spinner } from '@/components/ui'
import { useConfirm } from '@/lib/useConfirm'
import { EnumSelect } from '@/components/EnumSelect'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

const PHASES = [
  { value: 'off', label: 'Off-season' },
  { value: 'pre', label: 'Pre-season' },
  { value: 'in', label: 'In-season' },
]

const PHASE_LABELS: Record<string, string> = { off: 'Off-season', pre: 'Pre-season', in: 'In-season' }

export function SeasonPhases() {
  const { id } = useParams<{ id: string }>()
  const athleteId = Number(id)
  const queryClient = useQueryClient()
  const { confirm, dialog: confirmDialog } = useConfirm()

  const [showForm, setShowForm] = useState(false)
  const [phase, setPhase] = useState('off')
  const [sport, setSport] = useState('')
  const [startDate, setStartDate] = useState(new Date().toISOString().slice(0, 10))
  const [endDate, setEndDate] = useState('')
  const [notes, setNotes] = useState('')

  const { data: me } = useQuery({ queryKey: ['me'], queryFn: () => api.me() })
  const isCoach = me?.is_coach || me?.is_admin

  const { data: athlete } = useQuery({
    queryKey: ['athlete', athleteId],
    queryFn: () => api.getAthlete(athleteId),
    enabled: !isNaN(athleteId),
  })

  const { data: phases, isLoading, error } = useQuery({
    queryKey: ['season-phases', athleteId],
    queryFn: () => api.listSeasonPhases(athleteId),
    enabled: !isNaN(athleteId),
  })

  function resetForm() {
    setPhase('off')
    setSport('')
    setStartDate(new Date().toISOString().slice(0, 10))
    setEndDate('')
    setNotes('')
  }

  const createMutation = useMutation({
    mutationFn: () => {
      const data: SeasonPhaseInput = {
        phase,
        sport: sport.trim() || undefined,
        start_date: startDate,
        end_date: endDate.trim() || undefined,
        notes: notes.trim() || undefined,
      }
      return api.createSeasonPhase(athleteId, data)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['season-phases', athleteId] })
      resetForm()
      setShowForm(false)
    },
    onError: (err) => toast.error(err instanceof ApiError ? err.message : 'Failed to record season phase'),
  })

  const deleteMutation = useMutation({
    mutationFn: (phaseId: number) => api.deleteSeasonPhase(athleteId, phaseId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['season-phases', athleteId] }),
    onError: (err) => toast.error(err instanceof ApiError ? err.message : 'Failed to delete season phase'),
  })

  async function handleDelete(phaseId: number) {
    if (await confirm({ title: 'Delete season phase?', description: 'This removes the recorded phase window.', variant: 'danger', confirmLabel: 'Delete' })) {
      deleteMutation.mutate(phaseId)
    }
  }

  if (isLoading) return <Spinner />
  if (error) return <p className="text-destructive">Failed to load season phases.</p>

  return (
    <div>
      {confirmDialog()}
      <div className="flex items-center justify-between mb-6">
        <div>
          <p className="text-sm text-muted-foreground">
            <Link to={`/athletes/${athleteId}`} className="hover:text-foreground">
              {athlete?.name ?? 'Athlete'}
            </Link>
            {' / Season phases'}
          </p>
          <h1 className="text-2xl font-bold">Season phases</h1>
        </div>
        {isCoach && (
          <Button onClick={() => setShowForm(s => !s)}>{showForm ? 'Cancel' : '+ Record phase'}</Button>
        )}
      </div>

      {isCoach && showForm && (
        <form
          onSubmit={(e) => { e.preventDefault(); createMutation.mutate() }}
          className="space-y-4 max-w-lg mb-8 rounded-lg border p-4"
        >
          <div className="grid grid-cols-2 gap-3">
            <div>
              <Label>Phase</Label>
              <EnumSelect value={phase} onChange={setPhase} options={PHASES} required />
            </div>
            <div>
              <Label htmlFor="sport">Sport</Label>
              <Input id="sport" value={sport} onChange={e => setSport(e.target.value)} placeholder="Optional" />
            </div>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <Label htmlFor="start_date">Start date</Label>
              <Input id="start_date" type="date" value={startDate} onChange={e => setStartDate(e.target.value)} required />
            </div>
            <div>
              <Label htmlFor="end_date">End date</Label>
              <Input id="end_date" type="date" value={endDate} onChange={e => setEndDate(e.target.value)} />
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

      {phases && phases.length === 0 ? (
        <p className="text-muted-foreground">No season phases recorded yet.</p>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Phase</TableHead>
              <TableHead>Sport</TableHead>
              <TableHead>Start</TableHead>
              <TableHead>End</TableHead>
              {isCoach && <TableHead></TableHead>}
            </TableRow>
          </TableHeader>
          <TableBody>
            {phases?.map(p => (
              <TableRow key={p.id}>
                <TableCell className="font-medium">{PHASE_LABELS[p.phase] ?? p.phase}</TableCell>
                <TableCell>{p.sport ?? '—'}</TableCell>
                <TableCell>{p.start_date}</TableCell>
                <TableCell>{p.end_date ?? '—'}</TableCell>
                {isCoach && (
                  <TableCell className="text-right">
                    <Button variant="ghost" size="sm" onClick={() => handleDelete(p.id)}>Delete</Button>
                  </TableCell>
                )}
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </div>
  )
}
