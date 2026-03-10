import { useState, type ReactNode } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Menu, Sun, Moon, Settings, LogOut, User as UserIcon } from 'lucide-react'
import { api, ApiError } from '@/api/client'
import type { User } from '@/api/types'
import { Button } from '@/components/ui/button'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { Badge } from '@/components/ui/badge'
import { Separator } from '@/components/ui/separator'
import { Sheet, SheetTrigger, SheetContent, SheetTitle } from '@/components/ui/sheet'
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuLabel,
} from '@/components/ui/dropdown-menu'

const navItems = [
  { href: '/', label: 'Dashboard', icon: '🏠' },
  { href: '/athletes', label: 'Athletes', icon: '🏋️' },
  { href: '/exercises', label: 'Exercises', icon: '📋' },
  { href: '/notifications', label: 'Notifications', icon: '🔔' },
]

const coachItems = [
  { href: '/programs', label: 'Programs', icon: '📊' },
  { href: '/equipment', label: 'Equipment', icon: '🏗️' },
  { href: '/reviews/pending', label: 'Reviews', icon: '✅' },
]

const adminItems = [
  { href: '/users', label: 'Users', icon: '👥' },
  { href: '/admin/settings', label: 'Settings', icon: '⚙️' },
  { href: '/admin/catalog', label: 'Catalog', icon: '📚' },
]

interface LayoutProps {
  user: User
  children: ReactNode
  theme: 'dark' | 'light'
  onToggleTheme: () => void
}

export function Layout({ user, children, theme, onToggleTheme }: LayoutProps) {
  const location = useLocation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [sheetOpen, setSheetOpen] = useState(false)

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
    window.location.href = '/app/'
  }

  const initials = user.username.slice(0, 2).toUpperCase()
  const displayName = user.name ?? user.username
  const roleLabel = user.is_admin ? 'Admin' : user.is_coach ? 'Coach' : 'Athlete'

  function NavLinks({ onNavigate }: { onNavigate?: () => void }) {
    return (
      <>
        {navItems.map(item => (
          <Link
            key={item.href}
            to={item.href}
            onClick={onNavigate}
            className={`flex items-center gap-2 px-3 py-2 rounded-md text-sm transition-colors ${
              isActive(item.href)
                ? 'bg-primary/10 text-primary font-medium'
                : 'text-muted-foreground hover:text-foreground hover:bg-accent'
            }`}
          >
            <span>{item.icon}</span>
            {item.label}
            {item.href === '/notifications' && unread && unread.count > 0 && (
              <Badge variant="default" className="ml-auto h-5 w-5 rounded-full p-0 flex items-center justify-center text-xs">
                {unread.count}
              </Badge>
            )}
          </Link>
        ))}

        {(user.is_coach || user.is_admin) && (
          <>
            <Separator className="my-2" />
            <p className="px-3 py-1 text-xs font-semibold uppercase text-muted-foreground tracking-wider">
              Coaching
            </p>
            {coachItems.map(item => (
              <Link
                key={item.href}
                to={item.href}
                onClick={onNavigate}
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
            <Separator className="my-2" />
            <p className="px-3 py-1 text-xs font-semibold uppercase text-muted-foreground tracking-wider">
              Admin
            </p>
            {adminItems.map(item => (
              <Link
                key={item.href}
                to={item.href}
                onClick={onNavigate}
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
      </>
    )
  }

  return (
    <div className="flex min-h-screen bg-background text-foreground">
      <a href="#main-content" className="skip-link">Skip to content</a>

      {/* Mobile header */}
      <div className="fixed top-0 left-0 right-0 z-30 flex items-center justify-between border-b border-border bg-card px-4 py-3 md:hidden">
        <Sheet open={sheetOpen} onOpenChange={setSheetOpen}>
          <SheetTrigger>
            <Button variant="ghost" size="icon">
              <Menu className="h-5 w-5" />
            </Button>
          </SheetTrigger>
          <SheetContent side="left" className="w-56 p-0">
            <SheetTitle className="p-4 border-b border-border text-lg font-bold text-primary">
              RepLog
            </SheetTitle>
            <nav className="flex-1 p-2 space-y-1 overflow-y-auto">
              <NavLinks onNavigate={() => setSheetOpen(false)} />
            </nav>
          </SheetContent>
        </Sheet>
        <Link to="/" className="text-lg font-bold text-primary">RepLog</Link>
        <div className="w-10" />
      </div>

      {/* Desktop sidebar */}
      <aside className="hidden md:flex md:sticky md:top-0 md:h-screen md:w-56 md:flex-col md:border-r md:border-border md:bg-card">
        <div className="p-4 border-b border-border">
          <Link to="/" className="text-lg font-bold text-primary">RepLog</Link>
        </div>

        <nav className="flex-1 p-2 space-y-1 overflow-y-auto">
          <NavLinks />
        </nav>

        <div className="p-3 border-t border-border">
          <DropdownMenu>
            <DropdownMenuTrigger className="flex items-center gap-2 w-full rounded-md px-2 py-1.5 hover:bg-accent transition-colors text-left">
                <Avatar className="h-8 w-8">
                  <AvatarFallback className="bg-primary/20 text-primary text-xs font-bold">
                    {initials}
                  </AvatarFallback>
                </Avatar>
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-medium truncate">{displayName}</p>
                  <p className="text-xs text-muted-foreground">{roleLabel}</p>
                </div>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start" className="w-48">
              <DropdownMenuLabel>{displayName}</DropdownMenuLabel>
              <DropdownMenuSeparator />
              <DropdownMenuItem onClick={onToggleTheme}>
                {theme === 'dark' ? <Sun className="mr-2 h-4 w-4" /> : <Moon className="mr-2 h-4 w-4" />}
                {theme === 'dark' ? 'Light mode' : 'Dark mode'}
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => navigate('/preferences')}>
                <Settings className="mr-2 h-4 w-4" />
                Preferences
              </DropdownMenuItem>
              {user.athlete_id && (
                <DropdownMenuItem onClick={() => navigate(`/athletes/${user.athlete_id}`)}>
                  <UserIcon className="mr-2 h-4 w-4" />
                  My Profile
                </DropdownMenuItem>
              )}
              <DropdownMenuSeparator />
              <DropdownMenuItem onClick={handleLogout} className="text-destructive">
                <LogOut className="mr-2 h-4 w-4" />
                Sign out
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </aside>

      {/* Main content */}
      <main id="main-content" className="flex-1 p-4 pt-16 md:p-6 md:pt-6">
        {children}
      </main>
    </div>
  )
}
