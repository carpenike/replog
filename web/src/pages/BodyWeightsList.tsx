import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'
import { Spinner } from '@/components/ui'
import { useConfirm } from '@/lib/useConfirm'

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

  const { data: page, isLoading } = useQuery({
    queryKey: ['body-weights', athleteId],
    queryFn: () => api.listBodyWeights(athleteId),
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
          <label htmlFor="bw-date" className="block text-xs text-muted-foreground mb-1">Date</label>
          <input id="bw-date" type="date" value={date} onChange={e => setDate(e.target.value)}
            className="rounded-md border border-border bg-background px-3 py-1.5 text-sm" />
        </div>
        <div>
          <label htmlFor="bw-weight" className="block text-xs text-muted-foreground mb-1">Weight</label>
          <input id="bw-weight" type="number" step="0.1" value={weight} onChange={e => setWeight(e.target.value)}
            placeholder="185.0" required
            className="rounded-md border border-border bg-background px-3 py-1.5 text-sm w-24" />
        </div>
        <div className="flex-1">
          <label htmlFor="bw-notes" className="block text-xs text-muted-foreground mb-1">Notes</label>
          <input id="bw-notes" type="text" value={notes} onChange={e => setNotes(e.target.value)}
            placeholder="Optional"
            className="rounded-md border border-border bg-background px-3 py-1.5 text-sm w-full" />
        </div>
        <button type="submit" disabled={addMutation.isPending || !weight}
          className="rounded-md bg-primary px-4 py-1.5 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50">
          Log
        </button>
      </form>

      {/* List */}
      {isLoading ? (
        <Spinner />
      ) : page && page.entries.length === 0 ? (
        <p className="text-muted-foreground">No body weight entries yet.</p>
      ) : (
        <div className="rounded-lg border border-border overflow-hidden">
          <table className="w-full">
            <thead>
              <tr className="border-b border-border bg-muted/50">
                <th className="text-left p-3 text-sm font-medium text-muted-foreground">Date</th>
                <th className="text-left p-3 text-sm font-medium text-muted-foreground">Weight</th>
                <th className="text-left p-3 text-sm font-medium text-muted-foreground">Notes</th>
                <th className="p-3 w-12"></th>
              </tr>
            </thead>
            <tbody>
              {page?.entries.map(bw => (
                <tr key={bw.id} className="border-b border-border last:border-0">
                  <td className="p-3 text-sm">{bw.date}</td>
                  <td className="p-3 text-sm font-medium">{formatWeight(bw.weight)}</td>
                  <td className="p-3 text-sm text-muted-foreground">{bw.notes ?? ''}</td>
                  <td className="p-3">
                    <button
                      onClick={async () => { if (await confirm({ title: 'Delete Entry', description: 'Remove this body weight entry?', confirmLabel: 'Delete', variant: 'danger' })) deleteMutation.mutate(bw.id) }}
                      className="text-xs text-destructive hover:text-destructive/80"
                    >
                      ×
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      {confirmDialog()}
    </div>
  )
}
