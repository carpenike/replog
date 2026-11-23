import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { ChevronLeft, ChevronRight, Trash2 } from 'lucide-react'
import { api } from '@/api/client'
import { PageHeader } from '@/components/PageHeader'
import { QueryState } from '@/components/QueryState'
import { useFriendlyDate } from '@/lib/dates'
import { useConfirm } from '@/lib/useConfirm'
import { formatWeight, localDateISO } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Sparkline } from '@/components/ui/sparkline'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

export function BodyWeightsList() {
  const { id } = useParams<{ id: string }>()
  const athleteId = Number(id)
  const { confirm, dialog: confirmDialog } = useConfirm()
  const queryClient = useQueryClient()
  const [date, setDate] = useState(localDateISO)
  const [weight, setWeight] = useState('')
  const [notes, setNotes] = useState('')
  const [offset, setOffset] = useState(0)

  const friendlyDate = useFriendlyDate()

  const { data: athlete } = useQuery({
    queryKey: ['athlete', athleteId],
    queryFn: () => api.getAthlete(athleteId),
    enabled: !isNaN(athleteId),
  })
  const query = useQuery({
    queryKey: ['body-weights', athleteId, offset],
    queryFn: () => api.listBodyWeights(athleteId, offset),
    enabled: !isNaN(athleteId),
  })
  const addMutation = useMutation({
    mutationFn: () => api.createBodyWeight(athleteId, date, parseFloat(weight), notes),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['body-weights', athleteId] })
      setWeight('')
      setNotes('')
    },
  })
  const deleteMutation = useMutation({
    mutationFn: (bwId: number) => api.deleteBodyWeight(athleteId, bwId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['body-weights', athleteId] })
    },
  })

  // Trend sparkline: oldest → newest (entries come back newest-first).
  const page = query.data
  const trend = page?.entries.map(e => e.weight).slice().reverse() ?? []

  return (
    <div>
      <PageHeader
        title="Body Weight"
        back={{ to: `/athletes/${athleteId}`, label: athlete?.name ?? 'Athlete' }}
        actions={trend.length >= 2 && (
          <Sparkline data={trend} width={140} height={36} ariaLabel="Body weight trend" />
        )}
      />
      {/* Add form */}
      <form
        onSubmit={(e) => { e.preventDefault(); addMutation.mutate() }}
        className="flex flex-wrap gap-3 items-end mb-6 rounded-lg border border-border bg-card p-4"
      >
        <div>
          <Label htmlFor="bw-date" >Date <span aria-hidden="true" className="text-destructive">*</span></Label>
          <Input id="bw-date" type="date" enterKeyHint="done" value={date} onChange={e => setDate(e.target.value)} className="h-11 mt-1" required />
        </div>
        <div>
          <Label htmlFor="bw-weight">Weight <span aria-hidden="true" className="text-destructive">*</span></Label>
          <Input id="bw-weight" type="number" inputMode="decimal" enterKeyHint="done" step="0.1" value={weight} onChange={e => setWeight(e.target.value)} placeholder="185.0" required className="h-11 mt-1 w-24" />
        </div>
        <div className="flex-1">
          <Label htmlFor="bw-notes" >Notes</Label>
          <Input id="bw-notes" type="text" value={notes} onChange={e => setNotes(e.target.value)} placeholder="Optional" className="h-11 mt-1" />
        </div>
        <Button type="submit" size="touch" disabled={addMutation.isPending || !weight}>
          Log
        </Button>
      </form>
      {/* List */}
      <QueryState
        query={query}
        resource="body weights"
        isEmpty={p => p.entries.length === 0 && offset === 0}
        empty={{ icon: '⚖️', title: 'No body weight entries yet', description: 'Log your first entry above.' }}
      >
        {page => (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Date</TableHead>
              <TableHead>Weight</TableHead>
              <TableHead>Notes</TableHead>
              <TableHead className="w-12"><span className="sr-only">Actions</span></TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {page.entries.map(bw => (
              <TableRow key={bw.id}>
                <TableCell>{friendlyDate(bw.date)}</TableCell>
                <TableCell className="font-medium">{formatWeight(bw.weight)}</TableCell>
                <TableCell className="text-muted-foreground">{bw.notes ?? ''}</TableCell>
                <TableCell>
                  <Button variant="ghost" size="icon-sm" aria-label={`Delete entry from ${friendlyDate(bw.date)}`} className="text-muted-foreground hover:text-destructive" onClick={async () => { if (await confirm({ title: 'Delete Entry', description: 'Remove this body weight entry?', confirmLabel: 'Delete', variant: 'danger' })) deleteMutation.mutate(bw.id) }}>
                    <Trash2 aria-hidden="true" />
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
        )}
      </QueryState>
      {/* Pagination */}
      {page && (page.has_more || offset > 0) && (
        <div className="flex items-center justify-between pt-4">
          <Button variant="ghost" onClick={() => setOffset(Math.max(0, offset - 20))}
            disabled={offset === 0}
          >
            <ChevronLeft aria-hidden="true" /> Previous
          </Button>
          {page.has_more && (
            <Button variant="ghost" onClick={() => setOffset(offset + 20)}>
              Next <ChevronRight aria-hidden="true" />
            </Button>
          )}
        </div>
      )}
      {confirmDialog()}
    </div>
  )
}
