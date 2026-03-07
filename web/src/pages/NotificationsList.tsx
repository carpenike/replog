import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'

export function NotificationsList() {
  const { data: notifications, isLoading, error } = useQuery({
    queryKey: ['notifications'],
    queryFn: () => api.listNotifications(),
  })

  if (isLoading) return <p className="text-muted-foreground">Loading notifications...</p>
  if (error) return <p className="text-destructive">Failed to load notifications.</p>

  return (
    <div>
      <h1 className="text-2xl font-bold mb-6">Notifications</h1>

      {notifications && notifications.length === 0 ? (
        <p className="text-muted-foreground">No notifications.</p>
      ) : (
        <div className="space-y-2">
          {notifications?.map(n => (
            <div
              key={n.id}
              className={`rounded-lg border bg-card p-4 ${
                n.read ? 'border-border' : 'border-primary/30 bg-primary/5'
              }`}
            >
              <div className="flex items-start justify-between">
                <div>
                  <p className={`text-sm font-medium ${n.read ? '' : 'text-primary'}`}>
                    {n.title}
                  </p>
                  {n.message && (
                    <p className="text-sm text-muted-foreground mt-0.5">{n.message}</p>
                  )}
                </div>
                <span className="text-xs text-muted-foreground whitespace-nowrap ml-4">
                  {new Date(n.created_at).toLocaleDateString()}
                </span>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
