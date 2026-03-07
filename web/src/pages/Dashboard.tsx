import type { User } from '@/api/types'

interface DashboardProps {
  user: User
}

export function Dashboard({ user }: DashboardProps) {
  return (
    <div>
      <h1 className="text-2xl font-bold mb-6">
        Welcome, {user.name ?? user.username}
      </h1>
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        <div className="rounded-lg border border-border bg-card p-4">
          <h2 className="text-sm font-medium text-muted-foreground mb-1">Quick Actions</h2>
          <p className="text-foreground">Dashboard content coming soon.</p>
        </div>
      </div>
    </div>
  )
}
