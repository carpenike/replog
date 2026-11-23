import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, within } from '@testing-library/react'
import { createMemoryRouter, RouterProvider } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { api } from '@/api/client'
import type { AccessoryPlanData } from '@/api/types'
import { AccessoryPlans } from '@/pages/AccessoryPlans'

vi.mock('@/api/client', async () => {
  const actual = await vi.importActual<typeof import('@/api/client')>('@/api/client')
  return {
    ...actual,
    api: {
      getAthlete: vi.fn(),
      listAccessoryPlans: vi.fn(),
      listExercises: vi.fn(),
      updateAccessoryPlan: vi.fn(),
    },
  }
})

function plan(id: number, name: string, group_key: string | null): AccessoryPlanData {
  return {
    id, athlete_id: 7, day: 1, exercise_id: id, exercise_name: name, target_sets: 3, target_rep_min: 10,
    target_rep_max: 12, sort_order: id, group_key, active: true, created_at: '', updated_at: '',
  }
}

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(api.getAthlete).mockResolvedValue({ id: 7, name: 'Sam', track_body_weight: false, created_at: '', updated_at: '' })
  vi.mocked(api.listExercises).mockResolvedValue([])
})

describe('accessory groups', () => {
  it('renders lines sharing a group as a superset with A1/A2 labels and a group picker per line', async () => {
    vi.mocked(api.listAccessoryPlans).mockResolvedValue([
      plan(1, 'Face Pull', 'A'),
      plan(2, 'Plank', null),
      plan(3, 'Curl', 'A'),
    ])
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const router = createMemoryRouter([{ path: '/athletes/:id/accessories', element: <AccessoryPlans /> }], { initialEntries: ['/athletes/7/accessories'] })
    render(<QueryClientProvider client={queryClient}><RouterProvider router={router} /></QueryClientProvider>)

    const superset = await screen.findByRole('region', { name: 'Superset A' })
    expect(within(superset).getByText('A1')).toBeTruthy()
    expect(within(superset).getByText('Face Pull')).toBeTruthy()
    expect(within(superset).getByText('Curl')).toBeTruthy()
    expect(within(superset).queryByText('Plank')).toBeNull()
    expect(screen.getByLabelText('Superset group for Plank').textContent).toContain('No group')
  })
})
