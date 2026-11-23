import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { api } from '@/api/client'
import type { User } from '@/api/types'
import { Layout } from '@/components/Layout'

vi.mock('@/api/client', async () => {
  const actual = await vi.importActual<typeof import('@/api/client')>('@/api/client')
  return {
    ...actual,
    api: {
      unreadCount: vi.fn(),
      logout: vi.fn(),
      stopImpersonation: vi.fn(),
    },
  }
})

const me: User = {
  id: 1,
  username: 'ryan',
  name: 'Ryan',
  athlete_id: null,
  is_coach: true,
  is_admin: false,
  mcp_enabled: false,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}

function renderLayout(onToggleTheme = vi.fn()) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <Layout user={me} theme="dark" onToggleTheme={onToggleTheme}>
          <p>page</p>
        </Layout>
      </MemoryRouter>
    </QueryClientProvider>,
  )
  return onToggleTheme
}

beforeEach(() => {
  vi.mocked(api.unreadCount).mockResolvedValue({ count: 3 })
})

describe('mobile header', () => {
  it('names the hamburger and shows a bell with the unread count', async () => {
    renderLayout()

    expect(screen.getByRole('button', { name: 'Open navigation menu' })).toBeTruthy()
    const bell = await screen.findByRole('link', { name: 'Notifications, 3 unread' })
    expect(bell.getAttribute('href')).toBe('/notifications')
  })

  it('opens the account menu with theme, Preferences and Sign out', async () => {
    const onToggleTheme = renderLayout()

    await userEvent.click(screen.getByRole('button', { name: 'Account menu for Ryan' }))

    expect(await screen.findByRole('menuitem', { name: 'Preferences' })).toBeTruthy()
    expect(screen.getByRole('menuitem', { name: 'Sign out' })).toBeTruthy()
    await userEvent.click(screen.getByRole('menuitem', { name: 'Light mode' }))
    expect(onToggleTheme).toHaveBeenCalledTimes(1)
  })
})
