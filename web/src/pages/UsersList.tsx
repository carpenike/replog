import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'

export function UsersList() {
  const { data: users, isLoading, error } = useQuery({
    queryKey: ['users'],
    queryFn: () => api.listUsers(),
  })

  if (isLoading) return <p className="text-muted-foreground">Loading users...</p>
  if (error) return <p className="text-destructive">Failed to load users.</p>

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">Users</h1>
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
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
