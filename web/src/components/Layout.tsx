import type { ReactNode } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { api, ApiError } from '@/api/client'
import type { User } from '@/api/types'

const navItems = [
  { href: '/', label: 'Dashboard', icon: '🏠' },
  { href: '/athletes', label: 'Athletes', icon: '🏋️' },
  { href: '/exercises', label: 'Exercises', icon: '📋' },
  { href: '/notifications', label: 'Notifications', icon: '🔔' },
]

const coachItems = [
  { href: '/programs', label: 'Programs', icon: '📊' },
  { href: '/reviews/pending', label: 'Reviews', icon: '✅' },
]

const adminItems = [
  { href: '/users', label: 'Users', icon: '👥' },
  { href: '/admin/settings', label: 'Settings', icon: '⚙️' },
]

interface LayoutProps {
  user: User
  children: ReactNode
}

export function Layout({ user, children }: LayoutProps) {
  const location = useLocation()
  const queryClient = useQueryClient()

  const { data: unread } = useQuery({
    queryKey: ['unread-count'],
    queryFn: () => api.unreadCount(),
    refetchInterval: 60_000,
  })

  function isActive(href: string) {
    if (href === '/') return location.pathname === '/'
    return location.pathname.startsWith(href)
  }

  async function handleLogout() {
    try {
      await api.logout()
    } catch (e) {
      if (!(e instanceof ApiError)) throw e
    }
    queryClient.clear()
    window.location.href = '/'
  }

  return (
    <div className="flex min-h-screen bg-background text-foreground">
      {/* Sidebar */}
      <aside className="w-56 border-r border-border bg-card flex flex-col">
        <div className="p-4 border-b border-border">
          <Link to="/" className="text-lg font-bold text-primary">RepLog</Link>
        </div>

        <nav className="flex-1 p-2 space-y-1">
          {navItems.map(item => (
            <Link
              key={item.href}
              to={item.href}
              className={`flex items-center gap-2 px-3 py-2 rounded-md text-sm transition-colors ${
                isActive(item.href)
                  ? 'bg-primary/10 text-primary font-medium'
                  : 'text-muted-foreground hover:text-foreground hover:bg-accent'
              }`}
            >
              <span>{item.icon}</span>
              {item.label}
              {item.href === '/notifications' && unread && unread.count > 0 && (
                <span className="ml-auto bg-primary text-primary-foreground text-xs rounded-full w-5 h-5 flex items-center justify-center">
                  {unread.count}
                </span>
              )}
            </Link>
          ))}

          {(user.is_coach || user.is_admin) && (
            <>
              <div className="pt-4 pb-1 px-3 text-xs font-semibold uppercase text-muted-foreground tracking-wider">
                Coaching
              </div>
              {coachItems.map(item => (
                <Link
                  key={item.href}
                  to={item.href}
                  className={`flex items-center gap-2 px-3 py-2 rounded-md text-sm transition-colors ${
                    isActive(item.href)
                      ? 'bg-primary/10 text-primary font-medium'
                      : 'text-muted-foreground hover:text-foreground hover:bg-accent'
                  }`}
                >
                  <span>{item.icon}</span>
                  {item.label}
                </Link>
              ))}
            </>
          )}

          {user.is_admin && (
            <>
              <div className="pt-4 pb-1 px-3 text-xs font-semibold uppercase text-muted-foreground tracking-wider">
                Admin
              </div>
              {adminItems.map(item => (
                <Link
                  key={item.href}
                  to={item.href}
                  className={`flex items-center gap-2 px-3 py-2 rounded-md text-sm transition-colors ${
                    isActive(item.href)
                      ? 'bg-primary/10 text-primary font-medium'
                      : 'text-muted-foreground hover:text-foreground hover:bg-accent'
                  }`}
                >
                  <span>{item.icon}</span>
                  {item.label}
                </Link>
              ))}
            </>
          )}
        </nav>

        <div className="p-4 border-t border-border">
          <div className="flex items-center gap-2">
            <div className="w-8 h-8 rounded-full bg-primary/20 flex items-center justify-center text-xs font-bold text-primary">
              {user.username.slice(0, 2).toUpperCase()}
            </div>
            <div className="flex-1 min-w-0">
              <p className="text-sm font-medium truncate">{user.name ?? user.username}</p>
              <p className="text-xs text-muted-foreground">
                {user.is_admin ? 'Admin' : user.is_coach ? 'Coach' : 'Athlete'}
              </p>
            </div>
          </div>
          <button
            onClick={handleLogout}
            className="mt-2 w-full text-left px-3 py-1.5 rounded-md text-xs text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"
          >
            Sign out
          </button>
        </div>
      </aside>

      {/* Main content */}
      <main className="flex-1 p-6">
        {children}
      </main>
    </div>
  )
}
