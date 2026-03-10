import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'
import { Spinner } from '@/components/ui'
import { useConfirm } from '@/lib/useConfirm'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
function formatWeight(w: number): string {
  return w === Math.floor(w) ? w.toString() : w.toFixed(1)
}
export function BodyWeightsList() {
  const { id } = useParams<{ id: string }>()
  const athleteId = Number(id)
  const { confirm, dialog: confirmDialog } = useConfirm()
  const queryClient = useQueryClient()
  const [date, setDate] = useState(new Date().toISOString().slice(0, 10))
  const [weight, setWeight] = useState('')
  const [notes, setNotes] = useState('')
  const [offset, setOffset] = useState(0)
  const { data: page, isLoading } = useQuery({
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
  return (
    <div>
      <p className="text-sm text-muted-foreground mb-1">
        <Link to={`/athletes/${athleteId}`} className="hover:text-foreground">Athlete</Link>
        {' / Body Weight'}
      </p>
      <h1 className="text-2xl font-bold mb-6">Body Weight</h1>
      {/* Add form */}
      <form
        onSubmit={(e) => { e.preventDefault(); addMutation.mutate() }}
        className="flex flex-wrap gap-3 items-end mb-6 rounded-lg border border-border bg-card p-4"
      >
        <div>
          <Label htmlFor="bw-date" >Date</Label>
          <Input id="bw-date" type="date" value={date} onChange={e => setDate(e.target.value)} />
        </div>
        <div>
          <Label htmlFor="bw-weight" >Weight</Label>
          <Input id="bw-weight" type="number" step="0.1" value={weight} onChange={e => setWeight(e.target.value)} placeholder="185.0" required className="w-24" />
        </div>
        <div className="flex-1">
          <Label htmlFor="bw-notes" >Notes</Label>
          <Input id="bw-notes" type="text" value={notes} onChange={e => setNotes(e.target.value)} placeholder="Optional" />
        </div>
        <Button type="submit" disabled={addMutation.isPending || !weight}
          >
          Log
        </Button>
      </form>
      {/* List */}
      {isLoading ? (
        <Spinner />
      ) : page && page.entries.length === 0 ? (
        <p className="text-muted-foreground">No body weight entries yet.</p>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Date</TableHead>
              <TableHead>Weight</TableHead>
              <TableHead>Notes</TableHead>
              <TableHead className="w-12"></TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {page?.entries.map(bw => (
              <TableRow key={bw.id}>
                <TableCell>{bw.date}</TableCell>
                <TableCell className="font-medium">{formatWeight(bw.weight)}</TableCell>
                <TableCell className="text-muted-foreground">{bw.notes ?? ''}</TableCell>
                <TableCell>
                  <Button variant="ghost" size="xs" onClick={async () => { if (await confirm({ title: 'Delete Entry', description: 'Remove this body weight entry?', confirmLabel: 'Delete', variant: 'danger' })) deleteMutation.mutate(bw.id) }}>
                    ×
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
      {/* Pagination */}
      {page && (page.has_more || offset > 0) && (
        <div className="flex items-center justify-between pt-4">
          <Button variant="ghost" onClick={() => setOffset(Math.max(0, offset - 20))}
            disabled={offset === 0}
            
          >
            ← Previous
          </Button>
          {page.has_more && (
            <Button variant="ghost" onClick={() => setOffset(offset + 20)}
              
            >
              Next →
            </Button>
          )}
        </div>
      )}
      {confirmDialog()}
    </div>
  )
}