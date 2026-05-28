import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { api, ApiError } from '@/api/client'
import type { EquipmentData } from '@/api/types'
import { Spinner } from '@/components/ui'
import { useConfirm } from '@/lib/useConfirm'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

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
    onError: (err) => toast.error(err instanceof ApiError ? err.message : 'Failed to create equipment'),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => api.deleteEquipmentItem(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['equipment'] }),
    onError: (err) => toast.error(err instanceof ApiError ? err.message : 'Failed to delete equipment'),
  })

  if (isLoading) return <Spinner />

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">Equipment</h1>
        <Button variant="ghost" onClick={() => setShowForm(!showForm)}>
          {showForm ? 'Cancel' : '+ New'}
        </Button>
      </div>

      {showForm && (
        <form onSubmit={(e) => { e.preventDefault(); createMutation.mutate() }}
          className="rounded-lg border border-border bg-card p-4 mb-6 flex flex-wrap gap-3 items-end">
          <div className="flex-1 min-w-50">
            <Label>Name</Label>
            <Input type="text" value={name} onChange={e => setName(e.target.value)} required />
          </div>
          <div className="flex-1 min-w-50">
            <Label>Description</Label>
            <Input type="text" value={description} onChange={e => setDescription(e.target.value)} />
          </div>
          <Button type="submit" disabled={createMutation.isPending || !name}>
            Add
          </Button>
        </form>
      )}

      {equipment && equipment.length === 0 ? (
        <p className="text-muted-foreground">No equipment defined.</p>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Description</TableHead>
              <TableHead className="w-24"></TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {equipment?.map(e => (
              <EquipmentRow
                key={e.id}
                item={e}
                onDelete={async () => {
                  if (await confirm({ title: 'Delete Equipment', description: `Delete ${e.name}?`, confirmLabel: 'Delete', variant: 'danger' }))
                    deleteMutation.mutate(e.id)
                }}
              />
            ))}
          </TableBody>
        </Table>
      )}
      {confirmDialog()}
    </div>
  )
}

function EquipmentRow({ item, onDelete }: { item: EquipmentData; onDelete: () => void }) {
  const queryClient = useQueryClient()
  const [editing, setEditing] = useState(false)
  const [editName, setEditName] = useState(item.name)
  const [editDesc, setEditDesc] = useState(item.description ?? '')

  const updateMutation = useMutation({
    mutationFn: () => api.updateEquipment(item.id, editName.trim(), editDesc),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['equipment'] })
      setEditing(false)
    },
    onError: (err) => toast.error(err instanceof ApiError ? err.message : 'Failed to update equipment'),
  })

  if (editing) {
    return (
      <TableRow>
        <TableCell>
          <Input value={editName} onChange={e => setEditName(e.target.value)} />
        </TableCell>
        <TableCell>
          <Input value={editDesc} onChange={e => setEditDesc(e.target.value)} />
        </TableCell>
        <TableCell>
          <div className="flex gap-1">
            <Button size="xs" disabled={updateMutation.isPending || !editName.trim()} onClick={() => updateMutation.mutate()}>
              {updateMutation.isPending ? '…' : 'Save'}
            </Button>
            <Button size="xs" variant="ghost" onClick={() => { setEditing(false); setEditName(item.name); setEditDesc(item.description ?? '') }}>
              Cancel
            </Button>
          </div>
        </TableCell>
      </TableRow>
    )
  }

  return (
    <TableRow>
      <TableCell className="font-medium">{item.name}</TableCell>
      <TableCell className="text-muted-foreground">{item.description ?? '—'}</TableCell>
      <TableCell>
        <div className="flex gap-1">
          <Button variant="ghost" size="xs" onClick={() => setEditing(true)}>✎</Button>
          <Button variant="ghost" size="xs" onClick={onDelete}>×</Button>
        </div>
      </TableCell>
    </TableRow>
  )
}