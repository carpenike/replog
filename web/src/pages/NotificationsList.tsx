import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { api } from '@/api/client'
import { Spinner } from '@/components/ui'
import { Button } from '@/components/ui/button'

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
          <Button variant="ghost" onClick={() => markAllMutation.mutate()}
            disabled={markAllMutation.isPending}
            className="text-sm text-primary hover:text-primary/80 disabled:opacity-50">
            Mark all read
          </Button>
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
              <div className="flex items-start gap-4">
                {n.link ? (
                  <Link
                    to={n.link}
                    onClick={() => {
                      if (!n.read && !markReadMutation.isPending) markReadMutation.mutate(n.id)
                    }}
                    className="min-w-0 flex-1 rounded-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
                  >
                    <p className={`text-sm font-medium ${n.read ? '' : 'text-primary'}`}>
                      {n.title}
                    </p>
                    {n.message && (
                      <p className="text-sm text-muted-foreground mt-0.5">{n.message}</p>
                    )}
                  </Link>
                ) : (
                  <div className="min-w-0 flex-1">
                    <p className={`text-sm font-medium ${n.read ? '' : 'text-primary'}`}>
                      {n.title}
                    </p>
                    {n.message && (
                      <p className="text-sm text-muted-foreground mt-0.5">{n.message}</p>
                    )}
                  </div>
                )}
                <div className="flex shrink-0 items-center gap-2">
                  <span className="text-xs text-muted-foreground whitespace-nowrap">
                    {new Date(n.created_at).toLocaleDateString()}
                  </span>
                  {!n.read && (
                    <Button
                      variant="ghost"
                      aria-label={`Mark ${n.title} as read`}
                      onClick={() => markReadMutation.mutate(n.id)}
                      disabled={markReadMutation.isPending}
                    >
                      ✓
                    </Button>
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