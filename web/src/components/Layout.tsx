import { useState, type ReactNode } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Bell, BookOpen, Boxes, CalendarCheck, CheckSquare, ClipboardList, Dumbbell, Eye, House, Library, ListOrdered, LogOut, Menu,
  Moon, NotebookPen, Settings, Sun, Trophy, User as UserIcon, Users,
} from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { api, ApiError } from '@/api/client'
import type { User } from '@/api/types'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { Badge } from '@/components/ui/badge'
import { Separator } from '@/components/ui/separator'
import { Sheet, SheetTrigger, SheetContent, SheetTitle } from '@/components/ui/sheet'
import { cn } from '@/lib/utils'
import { bellLabel } from '@/lib/chrome'
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
} from '@/components/ui/dropdown-menu'

interface NavItemData { href: string; label: string; icon: LucideIcon }

/** A single sidebar nav link — single source of truth for the link markup/classes. */
function NavItem({
  item, active, onNavigate, children,
}: { item: NavItemData; active: boolean; onNavigate?: () => void; children?: ReactNode }) {
  return (
    <Link
      to={item.href}
      onClick={onNavigate}
      aria-current={active ? 'page' : undefined}
      className={cn(
        'flex items-center gap-2 px-3 py-2 rounded-md text-sm transition-colors',
        active
          ? 'bg-primary/10 text-primary font-medium'
          : 'text-muted-foreground hover:text-foreground hover:bg-accent',
      )}
    >
      <item.icon className="size-4 shrink-0" aria-hidden="true" />
      {item.label}
      {children}
    </Link>
  )
}

const navItems = [
  { href: '/', label: 'Dashboard', icon: House },
  { href: '/notifications', label: 'Notifications', icon: Bell },
]

const coachItems = [
  { href: '/athletes', label: 'Athletes', icon: Dumbbell },
  { href: '/exercises', label: 'Exercises', icon: ListOrdered },
  { href: '/programs', label: 'Programs', icon: ClipboardList },
  { href: '/equipment', label: 'Equipment', icon: Boxes },
  { href: '/reviews/pending', label: 'Reviews', icon: CheckSquare },
  { href: '/coach/compliance', label: 'Compliance', icon: CalendarCheck },
]

const adminItems = [
  { href: '/users', label: 'Users', icon: Users },
  { href: '/admin/settings', label: 'Settings', icon: Settings },
  { href: '/admin/catalog', label: 'Catalog', icon: Library },
]

interface LayoutProps {
  user: User
  children: ReactNode
  theme: 'dark' | 'light'
  onToggleTheme: () => void
}

interface NavLinksProps {
  user: User
  pathname: string
  unreadCount: number
  onNavigate?: () => void
}

function isActive(user: User, pathname: string, href: string) {
  if (href === '/') return pathname === '/'
  // Exact match for list pages and athlete profile to avoid false highlights
  if (
    href === '/athletes' ||
    href === '/exercises' ||
    href === '/programs' ||
    href === '/equipment' ||
    href === '/notifications'
  ) {
    return pathname === href
  }
  // My Profile: exact match only (don't highlight for sub-pages like /prescription)
  if (user.athlete_id && href === `/athletes/${user.athlete_id}`) {
    return pathname === href
  }
  return pathname.startsWith(href)
}

function NavLinks({ user, pathname, unreadCount, onNavigate }: NavLinksProps) {
  return (
    <>
      {user.athlete_id && (
        <>
          <p className="px-3 py-1 text-xs font-semibold uppercase text-muted-foreground tracking-wider">
            My Training
          </p>
          {[
            { href: `/athletes/${user.athlete_id}/prescription`, label: "Today's Workout", icon: ClipboardList },
            { href: `/athletes/${user.athlete_id}/workouts`, label: 'My Workouts', icon: NotebookPen },
            { href: `/athletes/${user.athlete_id}/records`, label: 'My Records', icon: Trophy },
            { href: `/athletes/${user.athlete_id}/journal`, label: 'My Journal', icon: BookOpen },
            { href: `/athletes/${user.athlete_id}`, label: 'My Profile', icon: UserIcon },
          ].map(item => (
            <NavItem key={item.href} item={item} active={isActive(user, pathname, item.href)} onNavigate={onNavigate} />
          ))}
          <Separator className="my-2" />
        </>
      )}

      {navItems.map(item => (
        <NavItem key={item.href} item={item} active={isActive(user, pathname, item.href)} onNavigate={onNavigate}>
          {item.href === '/notifications' && unreadCount > 0 && (
            <Badge variant="default" className="ml-auto h-5 w-5 rounded-full p-0 flex items-center justify-center text-xs">
              {unreadCount}
            </Badge>
          )}
        </NavItem>
      ))}

      {(user.is_coach || user.is_admin) && (
        <>
          <Separator className="my-2" />
          <p className="px-3 py-1 text-xs font-semibold uppercase text-muted-foreground tracking-wider">
            Coaching
          </p>
          {coachItems.map(item => (
            <NavItem key={item.href} item={item} active={isActive(user, pathname, item.href)} onNavigate={onNavigate} />
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
            <NavItem key={item.href} item={item} active={isActive(user, pathname, item.href)} onNavigate={onNavigate} />
          ))}
        </>
      )}
    </>
  )
}

interface AccountMenuProps {
  user: User
  theme: 'dark' | 'light'
  onToggleTheme: () => void
  onLogout: () => void
  /** compact = avatar-only trigger for the mobile header. */
  compact?: boolean
}

/** The account dropdown: theme, Preferences, Sign out. Shared by the desktop
 * sidebar footer and the mobile header. */
function AccountMenu({ user, theme, onToggleTheme, onLogout, compact }: AccountMenuProps) {
  const navigate = useNavigate()
  const initials = user.username.slice(0, 2).toUpperCase()
  const displayName = user.name ?? user.username
  const roleLabel = user.is_admin ? 'Admin' : user.is_coach ? 'Coach' : 'Athlete'
  const avatar = (
    <Avatar className="h-8 w-8">
      {user.avatar_url && <AvatarImage src={user.avatar_url} alt="" />}
      <AvatarFallback className="bg-primary/20 text-primary text-xs font-bold">
        {initials}
      </AvatarFallback>
    </Avatar>
  )
  return (
    <DropdownMenu>
      {compact ? (
        <DropdownMenuTrigger
          aria-label={`Account menu for ${displayName}`}
          className="inline-flex h-10 w-10 items-center justify-center rounded-full hover:bg-accent transition-colors"
        >
          {avatar}
        </DropdownMenuTrigger>
      ) : (
        <DropdownMenuTrigger className="flex items-center gap-2 w-full rounded-md px-2 py-1.5 hover:bg-accent transition-colors text-left">
          {avatar}
          <div className="flex-1 min-w-0">
            <p className="text-sm font-medium truncate">{displayName}</p>
            <p className="text-xs text-muted-foreground">{roleLabel}</p>
          </div>
        </DropdownMenuTrigger>
      )}
      <DropdownMenuContent align={compact ? 'end' : 'start'} className="w-48">
        <div className="px-1.5 py-1 text-xs font-medium text-muted-foreground">
          {displayName}{compact ? ` · ${roleLabel}` : ''}
        </div>
        <DropdownMenuSeparator />
        <DropdownMenuItem onClick={onToggleTheme}>
          {theme === 'dark' ? <Sun className="mr-2 h-4 w-4" /> : <Moon className="mr-2 h-4 w-4" />}
          {theme === 'dark' ? 'Light mode' : 'Dark mode'}
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => navigate('/preferences')}>
          <Settings className="mr-2 h-4 w-4" />
          Preferences
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem onClick={onLogout} className="text-destructive">
          <LogOut className="mr-2 h-4 w-4" />
          Sign out
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

/** Header bell linking to /notifications with the unread count. */
function NotificationBell({ unreadCount }: { unreadCount: number }) {
  return (
    <Link
      to="/notifications"
      aria-label={bellLabel(unreadCount)}
      className="relative inline-flex h-10 w-10 items-center justify-center rounded-lg hover:bg-accent transition-colors"
    >
      <Bell className="h-5 w-5" aria-hidden="true" />
      {unreadCount > 0 && (
        <span
          aria-hidden="true"
          className="absolute right-1 top-1 flex h-4 min-w-4 items-center justify-center rounded-full bg-primary px-1 text-[10px] font-semibold leading-none text-primary-foreground"
        >
          {unreadCount > 99 ? '99+' : unreadCount}
        </span>
      )}
    </Link>
  )
}

export function Layout({ user, children, theme, onToggleTheme }: LayoutProps) {
  const location = useLocation()
  const queryClient = useQueryClient()
  const [sheetOpen, setSheetOpen] = useState(false)

  const { data: unread } = useQuery({
    queryKey: ['unread-count'],
    queryFn: () => api.unreadCount(),
    refetchInterval: 60_000,
  })

  async function handleLogout() {
    try {
      await api.logout()
    } catch (e) {
      if (!(e instanceof ApiError)) throw e
    }
    queryClient.clear()
    window.location.href = '/'
  }

  const stopImpersonation = useMutation({
    mutationFn: () => api.stopImpersonation(),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['me'] })
      window.location.href = '/'
    },
  })

  const displayName = user.name ?? user.username
  const unreadCount = unread?.count ?? 0

  return (
    <div className="flex min-h-screen bg-background text-foreground">
      <a href="#main-content" className="skip-link">Skip to content</a>

      {/* Mobile header */}
      {/* Safe-area padding keeps it clear of the iOS status bar in standalone
          mode (black-translucent + viewport-fit=cover). */}
      <div className="fixed top-0 left-0 right-0 z-30 flex items-center justify-between border-b border-border bg-card pt-[calc(0.75rem+env(safe-area-inset-top))] pb-3 pl-[max(1rem,env(safe-area-inset-left))] pr-[max(1rem,env(safe-area-inset-right))] md:hidden">
        <Sheet open={sheetOpen} onOpenChange={setSheetOpen}>
          <SheetTrigger
            aria-label="Open navigation menu"
            className="inline-flex h-10 w-10 items-center justify-center rounded-lg hover:bg-accent transition-colors"
          >
            <Menu className="h-5 w-5" aria-hidden="true" />
          </SheetTrigger>
          <SheetContent
            side="left"
            className="w-56 p-0 pt-[env(safe-area-inset-top)] [&>[data-slot=sheet-close]]:top-[calc(0.75rem+env(safe-area-inset-top))]"
          >
            <SheetTitle className="p-4 border-b border-border text-lg font-bold text-primary">
              RepLog
            </SheetTitle>
            <nav className="flex-1 p-2 space-y-1 overflow-y-auto">
              <NavLinks
                user={user}
                pathname={location.pathname}
                unreadCount={unreadCount}
                onNavigate={() => setSheetOpen(false)}
              />
            </nav>
          </SheetContent>
        </Sheet>
        <Link to="/" className="text-lg font-bold text-primary">RepLog</Link>
        <div className="flex items-center gap-1">
          <NotificationBell unreadCount={unreadCount} />
          <AccountMenu
            user={user}
            theme={theme}
            onToggleTheme={onToggleTheme}
            onLogout={handleLogout}
            compact
          />
        </div>
      </div>

      {/* Desktop sidebar */}
      <aside className="hidden md:flex md:sticky md:top-0 md:h-screen md:w-56 md:flex-col md:border-r md:border-border md:bg-card">
        <div className="p-4 border-b border-border">
          <Link to="/" className="text-lg font-bold text-primary">RepLog</Link>
        </div>

        <nav className="flex-1 p-2 space-y-1 overflow-y-auto">
          <NavLinks user={user} pathname={location.pathname} unreadCount={unreadCount} />
        </nav>

        <div className="p-3 border-t border-border">
          <AccountMenu user={user} theme={theme} onToggleTheme={onToggleTheme} onLogout={handleLogout} />
        </div>
      </aside>

      {/* Main content */}
      <main
        id="main-content"
        className="flex-1 p-4 pt-(--app-header-offset) pl-[max(1rem,env(safe-area-inset-left))] pr-[max(1rem,env(safe-area-inset-right))] md:p-6 md:pt-6"
      >
        {user.impersonating && (
          <div className="mb-4 flex items-center justify-between rounded-lg bg-warning/20 border border-warning/40 px-4 py-2 text-sm">
            <span className="font-medium text-warning-strong">
              <Eye className="mr-1 inline size-4 align-text-bottom" aria-hidden="true" />
              Viewing as <strong>{displayName}</strong>
            </span>
            <button
              type="button"
              onClick={() => stopImpersonation.mutate()}
              disabled={stopImpersonation.isPending}
              className="min-h-11 rounded-md bg-warning px-3 py-1 text-xs font-medium text-warning-foreground hover:bg-warning/90 transition-colors disabled:opacity-50 md:min-h-0"
            >
              Exit Impersonation
            </button>
          </div>
        )}
        {children}
      </main>
    </div>
  )
}
