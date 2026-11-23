import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { createMemoryRouter, RouterProvider } from 'react-router-dom'
import { describe, expect, it, vi } from 'vitest'
import { api } from '@/api/client'
import type { ComplianceDay } from '@/api/types'
import { CompliancePage } from '@/pages/CompliancePage'

vi.mock('@/api/client', async () => {
  const actual = await vi.importActual<typeof import('@/api/client')>('@/api/client')
  return { ...actual, api: { getCoachCompliance: vi.fn(), getPreferences: vi.fn().mockResolvedValue({ weight_unit: 'lbs', timezone: 'UTC', date_format: 'Jan 2, 2006' }) } }
})

const statuses = ['done', 'rest', 'skipped', 'extra', 'partial', 'rest', 'rest'] as const

describe('compliance grid', () => {
  it('reads the digest week param and links logged days to the session', async () => {
    const days: ComplianceDay[] = statuses.map((status, i) => ({
      date: `2026-11-${String(9 + i).padStart(2, '0')}`,
      weekday: i + 1,
      status,
      session_id: status === 'done' ? 41 : status === 'extra' ? 42 : null,
      program_name: status === 'rest' || status === 'extra' ? null : 'Base',
      program_day_label: null,
    }))
    vi.mocked(api.getCoachCompliance).mockResolvedValue({
      week_start: '2026-11-09', week_end: '2026-11-15', timezone: 'UTC',
      athletes: [{
        athlete_id: 7, name: 'Dana', planned_count: 3, done_count: 1, partial_count: 1, skipped_count: 1,
        prescribed_count: 0, extra_count: 1, summary: '1 of 3', days,
      }],
    })
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const router = createMemoryRouter([{ path: '/coach/compliance', element: <CompliancePage /> }], { initialEntries: ['/coach/compliance?week=2026-11-09'] })
    render(<QueryClientProvider client={queryClient}><RouterProvider router={router} /></QueryClientProvider>)

    expect(await screen.findByText(/1 of 3 planned sessions done/)).toBeTruthy()
    expect(api.getCoachCompliance).toHaveBeenCalledWith('2026-11-09')
    const done = screen.getAllByRole('link', { name: /Dana, .*: Done/ })
    expect(done[0].getAttribute('href')).toBe('/athletes/7/workouts/41')
    expect(screen.getAllByRole('link', { name: /Dana, .*: Extra/ })[0].getAttribute('href')).toBe('/athletes/7/workouts/42')
    expect(screen.getAllByRole('img', { name: /Dana, .*: Skipped/ }).length).toBeGreaterThan(0)
  })
})
