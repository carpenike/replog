import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { Eye, Pencil, Plus, Trash2 } from 'lucide-react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { UserWithAthlete } from '@/api/types'
import { PageHeader } from '@/components/PageHeader'
import { QueryState } from '@/components/QueryState'
import { CardList, CardListItem, DesktopOnly } from '@/components/CardList'
import { useConfirm } from '@/lib/useConfirm'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

function RoleBadges({ u }: { u: UserWithAthlete }) {
  return (
    <div className="flex gap-1 flex-wrap">
      {u.is_admin && <Badge variant="destructive">Admin</Badge>}
      {u.is_coach && <Badge>Coach</Badge>}
      {!u.is_admin && !u.is_coach && <Badge variant="secondary">Athlete</Badge>}
    </div>
  )
}

interface UserActionsProps {
  u: UserWithAthlete
  onImpersonate: (id: number) => void
  onDelete: (u: UserWithAthlete) => void
  impersonating: boolean
}

function UserActions({ u, onImpersonate, onDelete, impersonating }: UserActionsProps) {
  return (
    <div className="flex gap-1">
      <Button variant="ghost" size="icon-sm" aria-label={`Edit ${u.username}`} render={<Link to={`/users/${u.id}/edit`} />}>
        <Pencil aria-hidden="true" />
      </Button>
      {!u.is_admin && (
        <Button variant="ghost" size="icon-sm" aria-label={`View as ${u.username}`} disabled={impersonating} onClick={() => onImpersonate(u.id)}>
          <Eye aria-hidden="true" />
        </Button>
      )}
      <Button variant="ghost" size="icon-sm" aria-label={`Delete ${u.username}`} className="text-muted-foreground hover:text-destructive" onClick={() => onDelete(u)}>
        <Trash2 aria-hidden="true" />
      </Button>
    </div>
  )
}

export function UsersList() {
  const { confirm, dialog: confirmDialog } = useConfirm()
  const queryClient = useQueryClient()

  const query = useQuery({
    queryKey: ['users'],
    queryFn: () => api.listUsers(),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => api.deleteUser(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] })
      toast.success('User deleted')
    },
  })
  // Failures toast via the global mutation cache.
  const impersonateMutation = useMutation({
    mutationFn: (id: number) => api.startImpersonation(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['me'] })
      window.location.href = '/'
    },
  })

  const handleDelete = async (u: UserWithAthlete) => {
    if (await confirm({ title: 'Delete User', description: `Delete user ${u.username}?`, confirmLabel: 'Delete', variant: 'danger' })) {
      deleteMutation.mutate(u.id)
    }
  }
  const actions = (u: UserWithAthlete) => (
    <UserActions u={u} onImpersonate={id => impersonateMutation.mutate(id)} onDelete={handleDelete} impersonating={impersonateMutation.isPending} />
  )

  return (
    <div>
      <PageHeader
        title="Users"
        actions={<Button render={<Link to="/users/new" />}><Plus aria-hidden="true" /> New User</Button>}
      />

      <QueryState query={query} resource="users" empty={{ icon: '👥', title: 'No users yet', action: 'New User', actionTo: '/users/new' }}>
        {users => (
          <>
            <CardList label="Users">
              {users.map(u => (
                <CardListItem
                  key={u.id}
                  title={u.name ? `${u.name} (${u.username})` : u.username}
                  subtitle={[u.email, u.athlete_name ? `Athlete: ${u.athlete_name}` : null].filter(Boolean).join(' · ') || undefined}
                  trailing={<RoleBadges u={u} />}
                  actions={actions(u)}
                />
              ))}
            </CardList>
            <DesktopOnly>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Username</TableHead>
                    <TableHead>Name</TableHead>
                    <TableHead>Email</TableHead>
                    <TableHead>Linked Athlete</TableHead>
                    <TableHead>Roles</TableHead>
                    <TableHead className="w-28"><span className="sr-only">Actions</span></TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {users.map(u => (
                    <TableRow key={u.id}>
                      <TableCell className="font-medium">
                        <Link to={`/users/${u.id}/edit`} className="hover:text-primary">{u.username}</Link>
                      </TableCell>
                      <TableCell>{u.name ?? '—'}</TableCell>
                      <TableCell className="text-muted-foreground">{u.email ?? '—'}</TableCell>
                      <TableCell>{u.athlete_name ?? '—'}</TableCell>
                      <TableCell><RoleBadges u={u} /></TableCell>
                      <TableCell>{actions(u)}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </DesktopOnly>
          </>
        )}
      </QueryState>
      {confirmDialog()}
    </div>
  )
}
