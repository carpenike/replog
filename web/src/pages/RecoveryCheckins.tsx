import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { api, ApiError } from '@/api/client'
import type { RecoveryCheckinInput } from '@/api/client'
import { Plus, Trash2, X } from 'lucide-react'
import { PageHeader } from '@/components/PageHeader'
import { QueryState } from '@/components/QueryState'
import { CardList, CardListItem, DesktopOnly } from '@/components/CardList'
import { useFriendlyDate } from '@/lib/dates'
import { localDateISO } from '@/lib/utils'
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
  const [date, setDate] = useState(localDateISO())
  const [sleepHours, setSleepHours] = useState('')
  const [soreness, setSoreness] = useState('')
  const [energy, setEnergy] = useState('')
  const [notes, setNotes] = useState('')

  const { data: athlete } = useQuery({
    queryKey: ['athlete', athleteId],
    queryFn: () => api.getAthlete(athleteId),
    enabled: !isNaN(athleteId),
  })

  const friendlyDate = useFriendlyDate()
  const listQuery = useQuery({
    queryKey: ['recovery-checkins', athleteId],
    queryFn: () => api.listRecoveryCheckins(athleteId),
    enabled: !isNaN(athleteId),
  })

  function resetForm() {
    setDate(localDateISO())
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


  return (
    <div>
      {confirmDialog()}
      <PageHeader
        title="Recovery"
        back={{ to: `/athletes/${athleteId}`, label: athlete?.name ?? 'Athlete' }}
        actions={
          <Button variant={showForm ? 'ghost' : 'default'} onClick={() => setShowForm(s => !s)} aria-expanded={showForm}>
            {showForm ? <><X aria-hidden="true" /> Cancel</> : <><Plus aria-hidden="true" /> Log Check-in</>}
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

      <QueryState
        query={listQuery}
        resource="recovery check-ins"
        empty={{ icon: '😴', title: 'No recovery check-ins logged yet', description: 'Sleep, soreness and energy help the coach read the week. They never change the plan.', action: !showForm ? 'Log Check-in' : undefined, onAction: () => setShowForm(true) }}
      >
        {checkins => (
          <>
            <CardList label="Recovery">
              {checkins.map(c => (
                <CardListItem
                  key={c.id}
                  title={friendlyDate(c.date)}
                  subtitle={[c.sleep_hours != null ? `Sleep ${c.sleep_hours}h` : null, c.soreness != null ? `Soreness ${c.soreness}` : null, c.energy != null ? `Energy ${c.energy}` : null, c.mood != null ? `Mood ${c.mood}` : null, c.low_readiness ? 'Flagged for coach' : null].filter(Boolean).join(' · ') || 'No scores'}
                  actions={<Button variant="ghost" size="sm" aria-label={`Delete ${'check-in ' + friendlyDate(c.date)}`} className="text-muted-foreground hover:text-destructive" onClick={() => handleDelete(c.id)}><Trash2 aria-hidden="true" /> Delete</Button>}
                />
              ))}
            </CardList>
            <DesktopOnly>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Date</TableHead>
                  <TableHead>Sleep</TableHead>
                  <TableHead>Soreness</TableHead>
                  <TableHead>Energy</TableHead>
                  <TableHead>Mood</TableHead>
                  <TableHead><span className="sr-only">Actions</span></TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {checkins.map(c => (
                  <TableRow key={c.id}>
                    <TableCell className="font-medium">{friendlyDate(c.date)}</TableCell>
                    <TableCell>{c.sleep_hours ?? '—'}</TableCell>
                    <TableCell>{c.soreness ?? '—'}</TableCell>
                    <TableCell>{c.energy ?? '—'}</TableCell>
                    <TableCell>
                      {c.mood ?? '—'}
                      {c.low_readiness && <span className="ml-2 rounded-full bg-warning/10 px-2 py-0.5 text-xs text-warning-strong">Flagged for coach</span>}
                    </TableCell>
                    <TableCell className="text-right">
                      <Button variant="ghost" size="icon-sm" aria-label={`Delete check-in ${friendlyDate(c.date)}`} className="text-muted-foreground hover:text-destructive" onClick={() => handleDelete(c.id)}><Trash2 aria-hidden="true" /></Button>
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
