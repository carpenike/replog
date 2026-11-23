import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createMemoryRouter, RouterProvider } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { api } from '@/api/client'
import type { ProgramTemplate } from '@/api/types'
import { ProgramDetail } from '@/pages/ProgramDetail'

vi.mock('@/api/client', async () => {
  const actual = await vi.importActual<typeof import('@/api/client')>('@/api/client')
  return {
    ...actual,
    api: {
      getProgramTemplate: vi.fn(),
      listExercises: vi.fn(),
      listProgressionRules: vi.fn(),
      updatePrescribedSet: vi.fn(),
    },
  }
})

function program(partial: Partial<ProgramTemplate> = {}): ProgramTemplate {
  return {
    id: 4, name: 'Hypertrophy', num_weeks: 1, num_days: 1, is_loop: false, status: 'active', version: 1,
    created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z', ...partial,
  }
}

let nextId = 1
function set(exercise_id: number, exercise_name: string, extra: Record<string, unknown> = {}) {
  return {
    id: nextId++, exercise_id, exercise_name, week: 1, day: 1, set_number: 1, reps: 8, percentage: null,
    absolute_weight: null, rest_seconds: null, rep_type: 'reps', notes: null, sort_order: 3, non_working: false, ...extra,
  }
}

function renderPage() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  const router = createMemoryRouter([{ path: '/programs/:id', element: <ProgramDetail /> }], { initialEntries: ['/programs/4'] })
  render(<QueryClientProvider client={queryClient}><RouterProvider router={router} /></QueryClientProvider>)
}

beforeEach(() => {
  vi.clearAllMocks()
  nextId = 1
  vi.mocked(api.listExercises).mockResolvedValue([])
  vi.mocked(api.listProgressionRules).mockResolvedValue([])
  vi.mocked(api.updatePrescribedSet).mockResolvedValue()
})

describe('program groups', () => {
  it('renders exercises sharing a group key as one labelled superset', async () => {
    vi.mocked(api.getProgramTemplate).mockResolvedValue({
      program: program(),
      sets: [
        set(1, 'Back Squat', { reps: 5, reps_max: 8, rir_min: 1, rir_max: 2 }),
        set(2, 'Lateral Raise', { group_key: 'B' }),
        set(3, 'Hammer Curl', { group_key: 'B' }),
        set(4, 'Cat to Cow', { non_working: true }),
      ],
    } as unknown as Awaited<ReturnType<typeof api.getProgramTemplate>>)
    renderPage()

    const superset = await screen.findByRole('region', { name: 'Superset B' })
    expect(within(superset).getByText('Lateral Raise')).toBeTruthy()
    expect(within(superset).getByText('Hammer Curl')).toBeTruthy()
    expect(within(superset).getByText('B1')).toBeTruthy()
    expect(within(superset).getByText('B2')).toBeTruthy()
    expect(screen.getByText(/5–8 reps .*RIR 1–2/)).toBeTruthy()
    expect(screen.getByText(/Warm-up 8 reps/)).toBeTruthy()
  })

  it('shows a group picker per exercise while editing', async () => {
    vi.mocked(api.getProgramTemplate).mockResolvedValue({
      program: program(),
      sets: [set(1, 'Back Squat'), set(2, 'Lateral Raise', { group_key: 'A' })],
    } as unknown as Awaited<ReturnType<typeof api.getProgramTemplate>>)
    renderPage()

    await userEvent.click(await screen.findByRole('button', { name: 'Edit' }))
    expect(screen.getByLabelText('Superset group for Back Squat')).toBeTruthy()
    expect(screen.getByLabelText('Superset group for Lateral Raise').textContent).toContain('Group A')
  })

  it('saves a half-filled RIR range as a single value and keeps the other fields', async () => {
    vi.mocked(api.getProgramTemplate).mockResolvedValue({
      program: program(),
      sets: [set(1, 'Back Squat', { prescribed_rpe: 8, group_key: 'A' })],
    } as unknown as Awaited<ReturnType<typeof api.getProgramTemplate>>)
    renderPage()

    await userEvent.click(await screen.findByRole('button', { name: 'Edit' }))
    await userEvent.click(screen.getByRole('button', { name: 'Edit set 1' }))
    await userEvent.type(document.getElementById('prog-edit-rir-min') as HTMLInputElement, '2')
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    expect(api.updatePrescribedSet).toHaveBeenCalledWith(4, 1, expect.objectContaining({
      rir_min: 2, rir_max: 2, sort_order: 3, prescribed_rpe: 8, group_key: 'A',
    }))
  })
})

describe('locked imports', () => {
  it('shows the Locked · vN badge, hides edit controls and explains re-importing', async () => {
    vi.mocked(api.getProgramTemplate).mockResolvedValue({
      program: program({ locked: true, import_version: 3 }),
      sets: [set(1, 'Back Squat')],
    } as unknown as Awaited<ReturnType<typeof api.getProgramTemplate>>)
    renderPage()

    expect(await screen.findByText(/Locked · v3/)).toBeTruthy()
    expect(screen.queryByRole('button', { name: 'Edit' })).toBeNull()
    expect(screen.getByText(/re-import a new version/)).toBeTruthy()
  })
})
