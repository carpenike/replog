import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { api } from '@/api/client'
import { Check, CheckCheck } from 'lucide-react'
import { PageHeader } from '@/components/PageHeader'
import { QueryState } from '@/components/QueryState'
import { useFriendlyDate } from '@/lib/dates'
import { Button } from '@/components/ui/button'

export function NotificationsList() {
  const queryClient = useQueryClient()
  const friendlyDate = useFriendlyDate()
  const query = useQuery({
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
  const hasUnread = query.data?.some(n => !n.read) ?? false
  return (
    <div>
      <PageHeader
        title="Notifications"
        actions={hasUnread && (
          <Button variant="ghost" onClick={() => markAllMutation.mutate()}
            disabled={markAllMutation.isPending}
            className="text-primary hover:text-primary/80">
            <CheckCheck aria-hidden="true" /> Mark all read
          </Button>
        )}
      />
      <QueryState
        query={query}
        resource="notifications"
        empty={{ icon: '🔔', title: "You're all caught up", description: 'Reviews, notes and program updates show up here.' }}
      >
        {notifications => (
        <div className="space-y-2">
          {notifications.map(n => (
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
                    {friendlyDate(n.created_at, { timestamp: true })}
                  </span>
                  {!n.read && (
                    <Button
                      variant="ghost"
                      size="icon"
                      aria-label={`Mark ${n.title} as read`}
                      onClick={() => markReadMutation.mutate(n.id)}
                      disabled={markReadMutation.isPending}
                    >
                      <Check aria-hidden="true" />
                    </Button>
                  )}
                </div>
              </div>
            </div>
          ))}
        </div>
        )}
      </QueryState>
    </div>
  )
}