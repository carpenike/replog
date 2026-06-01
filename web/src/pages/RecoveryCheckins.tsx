import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { api, ApiError } from '@/api/client'
import type { RecoveryCheckinInput } from '@/api/client'
import { Spinner } from '@/components/ui'
import { useConfirm } from '@/lib/useConfirm'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

function num(v: string): number | undefined {
  if (v.trim() === '') return undefined
  const n = Number(v)
  return isNaN(n) ? undefined : n
}

export function RecoveryCheckins() {
  const { id } = useParams<{ id: string }>()
  const athleteId = Number(id)
  const queryClient = useQueryClient()
  const { confirm, dialog: confirmDialog } = useConfirm()

  const [showForm, setShowForm] = useState(false)
  const [date, setDate] = useState(new Date().toISOString().slice(0, 10))
  const [sleepHours, setSleepHours] = useState('')
  const [soreness, setSoreness] = useState('')
  const [energy, setEnergy] = useState('')
  const [notes, setNotes] = useState('')

  const { data: athlete } = useQuery({
    queryKey: ['athlete', athleteId],
    queryFn: () => api.getAthlete(athleteId),
    enabled: !isNaN(athleteId),
  })

  const { data: checkins, isLoading, error } = useQuery({
    queryKey: ['recovery-checkins', athleteId],
    queryFn: () => api.listRecoveryCheckins(athleteId),
    enabled: !isNaN(athleteId),
  })

  function resetForm() {
    setDate(new Date().toISOString().slice(0, 10))
    setSleepHours('')
    setSoreness('')
    setEnergy('')
    setNotes('')
  }

  const createMutation = useMutation({
    mutationFn: () => {
      const data: RecoveryCheckinInput = {
        date,
        sleep_hours: num(sleepHours),
        soreness: num(soreness),
        energy: num(energy),
        notes: notes.trim() || undefined,
      }
      return api.createRecoveryCheckin(athleteId, data)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['recovery-checkins', athleteId] })
      resetForm()
      setShowForm(false)
    },
    onError: (err) => toast.error(err instanceof ApiError ? err.message : 'Failed to log recovery check-in'),
  })

  const deleteMutation = useMutation({
    mutationFn: (checkinId: number) => api.deleteRecoveryCheckin(athleteId, checkinId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['recovery-checkins', athleteId] }),
    onError: (err) => toast.error(err instanceof ApiError ? err.message : 'Failed to delete recovery check-in'),
  })

  async function handleDelete(checkinId: number) {
    if (await confirm({ title: 'Delete recovery check-in?', description: 'This removes the logged check-in.', variant: 'danger', confirmLabel: 'Delete' })) {
      deleteMutation.mutate(checkinId)
    }
  }

  if (isLoading) return <Spinner />
  if (error) return <p className="text-destructive">Failed to load recovery check-ins.</p>

  return (
    <div>
      {confirmDialog()}
      <div className="flex items-center justify-between mb-6">
        <div>
          <p className="text-sm text-muted-foreground">
            <Link to={`/athletes/${athleteId}`} className="hover:text-foreground">
              {athlete?.name ?? 'Athlete'}
            </Link>
            {' / Recovery'}
          </p>
          <h1 className="text-2xl font-bold">Recovery</h1>
        </div>
        <Button onClick={() => setShowForm(s => !s)}>{showForm ? 'Cancel' : '+ Log Check-in'}</Button>
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
          <div className="grid grid-cols-3 gap-3">
            <div>
              <Label htmlFor="sleep_hours">Sleep (hrs)</Label>
              <Input id="sleep_hours" type="number" step="0.1" min="0" max="24" value={sleepHours} onChange={e => setSleepHours(e.target.value)} />
            </div>
            <div>
              <Label htmlFor="soreness">Soreness (1–10)</Label>
              <Input id="soreness" type="number" min="1" max="10" value={soreness} onChange={e => setSoreness(e.target.value)} />
            </div>
            <div>
              <Label htmlFor="energy">Energy (1–10)</Label>
              <Input id="energy" type="number" min="1" max="10" value={energy} onChange={e => setEnergy(e.target.value)} />
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

      {checkins && checkins.length === 0 ? (
        <p className="text-muted-foreground">No recovery check-ins logged yet.</p>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Date</TableHead>
              <TableHead>Sleep</TableHead>
              <TableHead>Soreness</TableHead>
              <TableHead>Energy</TableHead>
              <TableHead></TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {checkins?.map(c => (
              <TableRow key={c.id}>
                <TableCell className="font-medium">{c.date}</TableCell>
                <TableCell>{c.sleep_hours ?? '—'}</TableCell>
                <TableCell>{c.soreness ?? '—'}</TableCell>
                <TableCell>{c.energy ?? '—'}</TableCell>
                <TableCell className="text-right">
                  <Button variant="ghost" size="sm" onClick={() => handleDelete(c.id)}>Delete</Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </div>
  )
}
