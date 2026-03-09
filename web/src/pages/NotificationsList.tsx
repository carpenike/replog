import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'
import { Spinner } from '@/components/ui'

export function NotificationsList() {
  const queryClient = useQueryClient()

  const { data: notifications, isLoading, error } = useQuery({
    queryKey: ['notifications'],
    queryFn: () => api.listNotifications(),
  })

  const markReadMutation = useMutation({
    mutationFn: (id: number) => api.markNotificationRead(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['notifications'] })
      queryClient.invalidateQueries({ queryKey: ['unread-count'] })
    },
  })

  const markAllMutation = useMutation({
    mutationFn: () => api.markAllNotificationsRead(),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['notifications'] })
      queryClient.invalidateQueries({ queryKey: ['unread-count'] })
    },
  })

  const hasUnread = notifications?.some(n => !n.read) ?? false

  if (isLoading) return <Spinner />
  if (error) return <p className="text-destructive">Failed to load notifications.</p>

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">Notifications</h1>
        {hasUnread && (
          <button onClick={() => markAllMutation.mutate()}
            disabled={markAllMutation.isPending}
            className="text-sm text-primary hover:text-primary/80 disabled:opacity-50">
            Mark all read
          </button>
        )}
      </div>

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
                <div className="flex-1">
                  <p className={`text-sm font-medium ${n.read ? '' : 'text-primary'}`}>
                    {n.title}
                  </p>
                  {n.message && (
                    <p className="text-sm text-muted-foreground mt-0.5">{n.message}</p>
                  )}
                </div>
                <div className="flex items-center gap-2 ml-4">
                  <span className="text-xs text-muted-foreground whitespace-nowrap">
                    {new Date(n.created_at).toLocaleDateString()}
                  </span>
                  {!n.read && (
                    <button onClick={() => markReadMutation.mutate(n.id)}
                      className="text-xs text-primary hover:text-primary/80">
                      ✓
                    </button>
                  )}
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
