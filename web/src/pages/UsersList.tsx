import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate } from 'react-router-dom'
import { api } from '@/api/client'
import { Spinner } from '@/components/ui'
import { useConfirm } from '@/lib/useConfirm'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

export function UsersList() {
  const navigate = useNavigate()
  const { confirm, dialog: confirmDialog } = useConfirm()
  const queryClient = useQueryClient()

  const { data: users, isLoading, error } = useQuery({
    queryKey: ['users'],
    queryFn: () => api.listUsers(),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => api.deleteUser(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['users'] }),
  })

  if (isLoading) return <Spinner />
  if (error) return <p className="text-destructive">Failed to load users.</p>

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">Users</h1>
        <Button onClick={() => navigate('/users/new')}>
          + New User
        </Button>
      </div>

      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Username</TableHead>
            <TableHead>Name</TableHead>
            <TableHead>Email</TableHead>
            <TableHead>Linked Athlete</TableHead>
            <TableHead>Roles</TableHead>
            <TableHead className="w-12"></TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {users?.map(u => (
            <TableRow key={u.id}>
              <TableCell className="font-medium">{u.username}</TableCell>
              <TableCell>{u.name ?? '—'}</TableCell>
              <TableCell className="text-muted-foreground">{u.email ?? '—'}</TableCell>
              <TableCell>{u.athlete_name ?? '—'}</TableCell>
              <TableCell>
                <div className="flex gap-1 flex-wrap">
                  {u.is_admin && <Badge variant="destructive">Admin</Badge>}
                  {u.is_coach && <Badge>Coach</Badge>}
                  {!u.is_admin && !u.is_coach && <Badge variant="secondary">Athlete</Badge>}
                  {u.mcp_enabled && <Badge variant="outline" title="MCP access enabled">MCP</Badge>}
                </div>
              </TableCell>
              <TableCell>
                <div className="flex gap-2">
                  <Link to={`/users/${u.id}/edit`} className="text-xs text-primary hover:text-primary/80">Edit</Link>
                  {!u.is_admin && (
                    <Button variant="ghost" size="xs" onClick={async () => {
                      await api.startImpersonation(u.id)
                      queryClient.invalidateQueries({ queryKey: ['me'] })
                      window.location.href = '/'
                    }}>
                      👁️
                    </Button>
                  )}
                  <Button variant="ghost" size="xs" onClick={async () => { if (await confirm({ title: 'Delete User', description: `Delete user ${u.username}?`, confirmLabel: 'Delete', variant: 'danger' })) deleteMutation.mutate(u.id) }}>
                    ×
                  </Button>
                </div>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
      {confirmDialog()}
    </div>
  )
}
