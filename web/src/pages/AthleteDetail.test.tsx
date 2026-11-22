import { QueryClient, QueryClientProvider, MutationCache } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createMemoryRouter, RouterProvider } from 'react-router-dom'
import { toast } from 'sonner'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { api, ApiError } from '@/api/client'
import type { Athlete, AthleteProgram, User } from '@/api/types'
import { AthleteDetail } from '@/pages/AthleteDetail'

vi.mock('@/api/client', async () => {
  const actual = await vi.importActual<typeof import('@/api/client')>('@/api/client')
  return {
    ...actual,
    api: {
      me: vi.fn(),
      getAthlete: vi.fn(),
      listWorkouts: vi.fn(),
      listTrainingMaxes: vi.fn(),
      listAthletePrograms: vi.fn(),
      listAthleteEquipment: vi.fn(),
      listEquipment: vi.fn(),
      promoteAthlete: vi.fn(),
      reactivateProgram: vi.fn(),
      deleteAthleteProgram: vi.fn(),
      startImpersonation: vi.fn(),
    },
  }
})

vi.mock('sonner', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}))

const coach: User = {
  id: 1,
  username: 'coach',
  name: 'Coach',
  athlete_id: null,
  is_coach: true,
  is_admin: false,
  mcp_enabled: false,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}

function athlete(tier: string | null): Athlete {
  return {
    id: 7,
    name: 'Sam',
    tier,
    track_body_weight: false,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
  }
}

const oldProgram: AthleteProgram = {
  id: 31,
  athlete_id: 7,
  template_id: 4,
  template_name: 'Base Builder',
  start_date: '2026-01-01',
  active: false,
  role: 'primary',
  deactivated_at: '2026-02-01T00:00:00Z',
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-02-01T00:00:00Z',
}

function renderPage() {
  // Mirror main.tsx: failed mutations toast unless they opt out.
  const mutationCache = new MutationCache({
    onError: (error, _v, _c, mutation) => {
      if (mutation.meta?.skipGlobalError) return
      toast.error(error instanceof Error ? error.message : 'Something went wrong.')
    },
  })
  const queryClient = new QueryClient({
    mutationCache,
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  const router = createMemoryRouter(
    [{ path: '/athletes/:id', element: <AthleteDetail /> }],
    { initialEntries: ['/athletes/7'] },
  )
  render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  )
}

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(api.me).mockResolvedValue(coach)
  vi.mocked(api.getAthlete).mockResolvedValue(athlete('foundational'))
  vi.mocked(api.listWorkouts).mockResolvedValue({ workouts: [], has_more: false })
  vi.mocked(api.listTrainingMaxes).mockResolvedValue([])
  vi.mocked(api.listAthletePrograms).mockResolvedValue([oldProgram] as unknown as Awaited<ReturnType<typeof api.listAthletePrograms>>)
  vi.mocked(api.listAthleteEquipment).mockResolvedValue([])
  vi.mocked(api.listEquipment).mockResolvedValue([])
})

describe('promote', () => {
  it('confirms with from → to copy before promoting, then toasts success', async () => {
    vi.mocked(api.promoteAthlete).mockResolvedValue(athlete('intermediate'))
    renderPage()

    await userEvent.click(await screen.findByRole('button', { name: 'Promote' }))
    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByText('Promote to Intermediate?')).toBeTruthy()
    expect(within(dialog).getByText(/Sam moves from Foundational to Intermediate/)).toBeTruthy()
    expect(api.promoteAthlete).not.toHaveBeenCalled()

    await userEvent.click(within(dialog).getByRole('button', { name: 'Promote to Intermediate' }))

    await waitFor(() => expect(api.promoteAthlete).toHaveBeenCalledWith(7))
    await waitFor(() => expect(toast.success).toHaveBeenCalledWith('Sam promoted to Intermediate.'))
  })

  it('does nothing when the coach cancels', async () => {
    renderPage()

    await userEvent.click(await screen.findByRole('button', { name: 'Promote' }))
    await userEvent.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Cancel' }))

    expect(api.promoteAthlete).not.toHaveBeenCalled()
  })

  it('toasts the error when the promote fails', async () => {
    vi.mocked(api.promoteAthlete).mockRejectedValue(new ApiError('already at highest tier', 400))
    renderPage()

    await userEvent.click(await screen.findByRole('button', { name: 'Promote' }))
    await userEvent.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Promote to Intermediate' }))

    await waitFor(() => expect(toast.error).toHaveBeenCalledWith("Couldn't promote: already at highest tier"))
    expect(toast.error).toHaveBeenCalledTimes(1)
  })

  it('hides Promote at the top tier', async () => {
    vi.mocked(api.getAthlete).mockResolvedValue(athlete('sport_performance'))
    renderPage()

    expect(await screen.findByRole('heading', { name: 'Sam' })).toBeTruthy()
    expect(screen.queryByRole('button', { name: 'Promote' })).toBeNull()
  })
})

describe('previous program actions', () => {
  it('names the icon buttons and toasts a failed reactivate', async () => {
    vi.mocked(api.reactivateProgram).mockRejectedValue(new ApiError('Role already taken', 409))
    renderPage()

    await userEvent.click(await screen.findByText('1 previous program'))
    expect(screen.getByRole('button', { name: 'Delete Base Builder assignment' })).toBeTruthy()
    await userEvent.click(screen.getByRole('button', { name: 'Reactivate Base Builder' }))
    await userEvent.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Reactivate' }))

    await waitFor(() => expect(api.reactivateProgram).toHaveBeenCalledWith(7, 31))
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith('Role already taken'))
  })
})
