import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Pencil, Plus, X } from 'lucide-react'
import { api, ApiError } from '@/api/client'
import type { EquipmentData } from '@/api/types'
import { PageHeader } from '@/components/PageHeader'
import { QueryState } from '@/components/QueryState'
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

  const query = useQuery({
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

  return (
    <div>
      <PageHeader
        title="Equipment"
        description="What each gym has, so programs only prescribe lifts an athlete can do."
        actions={
          <Button variant={showForm ? 'ghost' : 'default'} onClick={() => setShowForm(!showForm)} aria-expanded={showForm}>
            {showForm ? <><X aria-hidden="true" /> Cancel</> : <><Plus aria-hidden="true" /> New</>}
          </Button>
        }
      />

      {showForm && (
        <form onSubmit={(e) => { e.preventDefault(); createMutation.mutate() }}
          className="rounded-lg border border-border bg-card p-4 mb-6 flex flex-wrap gap-3 items-end">
          <div className="flex-1 min-w-50">
            <Label htmlFor="equipment-name">Name <span aria-hidden="true" className="text-destructive">*</span></Label>
            <Input id="equipment-name" type="text" value={name} onChange={e => setName(e.target.value)} required />
          </div>
          <div className="flex-1 min-w-50">
            <Label htmlFor="equipment-desc">Description</Label>
            <Input id="equipment-desc" type="text" value={description} onChange={e => setDescription(e.target.value)} />
          </div>
          <Button type="submit" disabled={createMutation.isPending || !name}>
            Add
          </Button>
        </form>
      )}

      <QueryState
        query={query}
        resource="equipment"
        empty={{ icon: '🏗️', title: 'No equipment yet', description: 'Add the bars, racks and machines your gyms have.', action: showForm ? undefined : 'Add equipment', onAction: () => setShowForm(true) }}
      >
        {equipment => (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Description</TableHead>
              <TableHead className="w-24"><span className="sr-only">Actions</span></TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {equipment.map(e => (
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
      </QueryState>
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
          <Input aria-label="Name" value={editName} onChange={e => setEditName(e.target.value)} />
        </TableCell>
        <TableCell>
          <Input aria-label="Description" value={editDesc} onChange={e => setEditDesc(e.target.value)} />
        </TableCell>
        <TableCell>
          <div className="flex gap-1">
            <Button size="sm" disabled={updateMutation.isPending || !editName.trim()} onClick={() => updateMutation.mutate()}>
              {updateMutation.isPending ? '…' : 'Save'}
            </Button>
            <Button size="sm" variant="ghost" onClick={() => { setEditing(false); setEditName(item.name); setEditDesc(item.description ?? '') }}>
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
          <Button variant="ghost" size="icon-sm" aria-label={`Edit ${item.name}`} onClick={() => setEditing(true)}>
            <Pencil aria-hidden="true" />
          </Button>
          <Button variant="ghost" size="icon-sm" aria-label={`Delete ${item.name}`} className="hover:text-destructive" onClick={onDelete}>
            <X aria-hidden="true" />
          </Button>
        </div>
      </TableCell>
    </TableRow>
  )
}