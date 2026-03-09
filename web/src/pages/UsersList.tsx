import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { api } from '@/api/client'
import { Spinner } from '@/components/ui'
import { useConfirm } from '@/lib/useConfirm'

export function UsersList() {
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
        <Link to="/users/new"
          className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90">
          + New User
        </Link>
      </div>

      <div className="rounded-lg border border-border overflow-hidden">
        <table className="w-full">
          <thead>
            <tr className="border-b border-border bg-muted/50">
              <th className="text-left p-3 text-sm font-medium text-muted-foreground">Username</th>
              <th className="text-left p-3 text-sm font-medium text-muted-foreground">Name</th>
              <th className="text-left p-3 text-sm font-medium text-muted-foreground">Email</th>
              <th className="text-left p-3 text-sm font-medium text-muted-foreground">Linked Athlete</th>
              <th className="text-left p-3 text-sm font-medium text-muted-foreground">Roles</th>
              <th className="p-3 w-12"></th>
            </tr>
          </thead>
          <tbody>
            {users?.map(u => (
              <tr key={u.id} className="border-b border-border last:border-0 hover:bg-muted/30 transition-colors">
                <td className="p-3 text-sm font-medium">{u.username}</td>
                <td className="p-3 text-sm">{u.name ?? '—'}</td>
                <td className="p-3 text-sm text-muted-foreground">{u.email ?? '—'}</td>
                <td className="p-3 text-sm">{u.athlete_name ?? '—'}</td>
                <td className="p-3 text-sm">
                  <div className="flex gap-1">
                    {u.is_admin && <span className="text-xs px-1.5 py-0.5 rounded bg-destructive/10 text-destructive">Admin</span>}
                    {u.is_coach && <span className="text-xs px-1.5 py-0.5 rounded bg-primary/10 text-primary">Coach</span>}
                    {!u.is_admin && !u.is_coach && <span className="text-xs text-muted-foreground">Athlete</span>}
                  </div>
                </td>
                <td className="p-3">
                  <button
                    onClick={async () => { if (await confirm({ title: 'Delete User', description: `Delete user ${u.username}?`, confirmLabel: 'Delete', variant: 'danger' })) deleteMutation.mutate(u.id) }}
                    className="text-xs text-destructive hover:text-destructive/80">
                    ×
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {confirmDialog()}
    </div>
  )
}
