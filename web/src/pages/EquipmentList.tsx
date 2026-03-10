import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'
import { Spinner } from '@/components/ui'
import { useConfirm } from '@/lib/useConfirm'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Card, CardContent } from '@/components/ui/card'
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
        <Button variant="ghost" onClick={() => setShowForm(!showForm)}
          >
          {showForm ? 'Cancel' : '+ New'}
        </Button>
      </div>
      {showForm && (
        <form onSubmit={(e) => { e.preventDefault(); createMutation.mutate() }}
          className="rounded-lg border border-border bg-card p-4 mb-6 flex flex-wrap gap-3 items-end">
          <div className="flex-1 min-w-50">
            <Label >Name</Label>
            <Input type="text" value={name} onChange={e => setName(e.target.value)} required />
          </div>
          <div className="flex-1 min-w-50">
            <Label >Description</Label>
            <Input type="text" value={description} onChange={e => setDescription(e.target.value)} />
          </div>
          <Button type="submit" disabled={createMutation.isPending || !name}
            >
            Add
          </Button>
        </form>
      )}
      {equipment && equipment.length === 0 ? (
        <p className="text-muted-foreground">No equipment defined.</p>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
          {equipment?.map(e => (
            <Card size="sm" className="flex items-center justify-between">
              <CardContent>
              <div>
                <p className="text-sm font-medium">{e.name}</p>
                {e.description && <p className="text-xs text-muted-foreground">{e.description}</p>}
              </div>
              <Button variant="ghost" onClick={async () => {
                if (await confirm({ title: 'Delete Equipment', description: `Delete ${e.name}?`, confirmLabel: 'Delete', variant: 'danger' }))
                  deleteMutation.mutate(e.id)
              }} className="text-xs text-destructive hover:text-destructive/80">×</Button>
              </CardContent>
            </Card>
          ))}
        </div>
      )}
      {confirmDialog()}
    </div>
  )
}