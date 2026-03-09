import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'
import { Spinner } from '@/components/ui'
import { useConfirm } from '@/lib/useConfirm'

export function EquipmentList() {
  const queryClient = useQueryClient()
  const { confirm, dialog: confirmDialog } = useConfirm()
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [showForm, setShowForm] = useState(false)

  const { data: equipment, isLoading } = useQuery({
    queryKey: ['equipment'],
    queryFn: () => api.listEquipment(),
  })

  const createMutation = useMutation({
    mutationFn: () => api.createEquipment(name, description),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['equipment'] })
      setName('')
      setDescription('')
      setShowForm(false)
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => api.deleteEquipmentItem(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['equipment'] }),
  })

  if (isLoading) return <Spinner />

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">Equipment</h1>
        <button onClick={() => setShowForm(!showForm)}
          className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90">
          {showForm ? 'Cancel' : '+ New'}
        </button>
      </div>

      {showForm && (
        <form onSubmit={(e) => { e.preventDefault(); createMutation.mutate() }}
          className="rounded-lg border border-border bg-card p-4 mb-6 flex flex-wrap gap-3 items-end">
          <div className="flex-1 min-w-50">
            <label className="block text-xs text-muted-foreground mb-1">Name</label>
            <input type="text" value={name} onChange={e => setName(e.target.value)} required
              className="w-full rounded-md border border-border bg-background px-3 py-1.5 text-sm" />
          </div>
          <div className="flex-1 min-w-50">
            <label className="block text-xs text-muted-foreground mb-1">Description</label>
            <input type="text" value={description} onChange={e => setDescription(e.target.value)}
              className="w-full rounded-md border border-border bg-background px-3 py-1.5 text-sm" />
          </div>
          <button type="submit" disabled={createMutation.isPending || !name}
            className="rounded-md bg-primary px-4 py-1.5 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50">
            Add
          </button>
        </form>
      )}

      {equipment && equipment.length === 0 ? (
        <p className="text-muted-foreground">No equipment defined.</p>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
          {equipment?.map(e => (
            <div key={e.id} className="flex items-center justify-between rounded-lg border border-border bg-card p-3">
              <div>
                <p className="text-sm font-medium">{e.name}</p>
                {e.description && <p className="text-xs text-muted-foreground">{e.description}</p>}
              </div>
              <button onClick={async () => {
                if (await confirm({ title: 'Delete Equipment', description: `Delete ${e.name}?`, confirmLabel: 'Delete', variant: 'danger' }))
                  deleteMutation.mutate(e.id)
              }} className="text-xs text-destructive hover:text-destructive/80">×</button>
            </div>
          ))}
        </div>
      )}
      {confirmDialog()}
    </div>
  )
}
