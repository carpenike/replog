import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, within } from '@testing-library/react'
import { createMemoryRouter, RouterProvider } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { api } from '@/api/client'
import type { PrescriptionData, WorkoutDetailData } from '@/api/types'
import { localDateISO } from '@/lib/utils'
import { ActiveWorkout } from '@/pages/ActiveWorkout'

vi.mock('@/api/client', async () => {
  const actual = await vi.importActual<typeof import('@/api/client')>('@/api/client')
  return {
    ...actual,
    api: {
      getWorkout: vi.fn(),
      getPrescription: vi.fn(),
      listExercises: vi.fn(),
      getPreferences: vi.fn(),
      getPlateSetup: vi.fn(),
      listExerciseHistory: vi.fn(),
      addSet: vi.fn(),
      createRecoveryCheckin: vi.fn(),
    },
  }
})

const today = localDateISO()

function workout(extra: Partial<WorkoutDetailData> = {}): WorkoutDetailData {
  return {
    workout: { id: 5, athlete_id: 7, date: today, set_count: 0, created_at: new Date().toISOString(), updated_at: '' },
    groups: [],
    swaps: [],
    ...extra,
  }
}

const prescription: PrescriptionData = {
  program_name: 'Block A',
  current_week: 1,
  current_day: 1,
  cycle_number: 1,
  lines: [
    {
      exercise_id: 1,
      exercise_name: 'Back Squat',
      sets: [
        { set_number: 1, reps: 5, rep_type: 'standard', non_working: true, absolute_weight: 135 },
        { set_number: 2, reps: 5, rep_type: 'standard', absolute_weight: 225 },
      ],
    },
    {
      exercise_id: 2, exercise_name: 'Lateral Raise', group_key: 'B',
      sets: [
        { set_number: 1, reps: 8, reps_max: 12, rir_min: 2, rir_max: 3, rep_type: 'standard', group_key: 'B' },
        { set_number: 2, reps: 8, reps_max: 12, rep_type: 'standard', group_key: 'B' },
      ],
    },
    {
      exercise_id: 3, exercise_name: 'Hammer Curl', group_key: 'B',
      sets: [
        { set_number: 1, reps: 10, rep_type: 'standard', group_key: 'B' },
        { set_number: 2, reps: 10, rep_type: 'standard', group_key: 'B' },
      ],
    },
    {
      exercise_id: 4, exercise_name: 'Deadlift',
      sets: [{ set_number: 1, reps: 3, rep_type: 'standard', absolute_weight: 315 }],
    },
  ],
}

function renderSession({ askReadiness = false } = {}) {
  // The readiness dialog is modal; most tests start with it already skipped.
  if (!askReadiness) localStorage.setItem('replog.readiness-dismissed.5', '1')
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const router = createMemoryRouter(
    [{ path: '/athletes/:id/workouts/:workoutId/session', element: <ActiveWorkout /> }],
    { initialEntries: ['/athletes/7/workouts/5/session'] },
  )
  render(<QueryClientProvider client={queryClient}><RouterProvider router={router} /></QueryClientProvider>)
}

beforeEach(() => {
  localStorage.clear()
  vi.mocked(api.getWorkout).mockResolvedValue(workout())
  vi.mocked(api.getPrescription).mockResolvedValue(prescription)
  vi.mocked(api.listExercises).mockResolvedValue([
    { id: 1, name: 'Back Squat', form_notes: 'Brace, then sit between the hips.', demo_url: 'https://youtu.be/dQw4w9WgXcQ', featured: false, created_at: '', updated_at: '' },
  ])
  vi.mocked(api.getPreferences).mockResolvedValue({ id: 1, user_id: 1, weight_unit: 'lbs', timezone: 'UTC', date_format: '2006-01-02', created_at: '', updated_at: '' })
  vi.mocked(api.getPlateSetup).mockResolvedValue({
    athlete_id: 7, bar: { weight: 20, unit: 'kg' }, collar: { weight: 2.5, unit: 'kg' }, use_collars: false, is_default: true,
    plates: [25, 20, 15, 10, 5, 2.5, 2, 1.5, 1, 0.5].map(w => ({ weight: w, unit: 'kg', pairs: w === 25 ? 4 : 1 })),
  })
  vi.mocked(api.listExerciseHistory).mockResolvedValue({ days: [], has_more: false })
})

describe('ActiveWorkout session', () => {
  it('counts only working sets and shows imported warm-ups apart', async () => {
    renderSession()
    // 1 squat + 2 + 2 superset + 1 deadlift working sets; the warm-up is not counted.
    expect(await screen.findByText('Set 1 of 6')).toBeTruthy()
    expect(screen.getByRole('progressbar', { name: 'Working sets completed' }).getAttribute('aria-valuemax')).toBe('6')
    const warmups = screen.getByRole('group', { name: 'Back Squat warm-ups' })
    expect(within(warmups).getByLabelText('Warm-up')).toBeTruthy()
    expect(within(warmups).getByText(/not counted/)).toBeTruthy()
  })

  it('renders a superset as one card alternating members each round, with rep ranges and RIR', async () => {
    renderSession()
    const card = await screen.findByRole('region', { name: 'Superset B' })
    const labels = within(card).getAllByText(/^B[12]·[12]$/).map(el => el.textContent)
    expect(labels).toEqual(['B1·1', 'B2·1', 'B1·2', 'B2·2'])
    expect(within(card).getAllByText('× 8–12').length).toBe(2)
    expect(within(card).getByText('RIR 2–3')).toBeTruthy()
  })

  it('shows kg plates per side for a lb target and offers the real loaded weight', async () => {
    renderSession()
    const trigger = await screen.findByRole('button', { name: /Plates for Back Squat set 1: 25 \+ 15 \+ 1 kg per side, 224.9 lb loaded/ })
    fireEvent.click(trigger)
    const use = await screen.findByRole('button', { name: 'Use 224.9 lb' })
    fireEvent.click(use)
    expect((screen.getByLabelText('Back Squat set 1 weight') as HTMLInputElement).value).toBe('224.9')
  })

  it('generates a loadable warm-up ramp on request', async () => {
    renderSession()
    const deadlift = (await screen.findByRole('heading', { name: 'Deadlift' })).closest('section') as HTMLElement
    fireEvent.click(within(deadlift).getByRole('button', { name: /Warm-ups/ }))
    const ramp = within(deadlift).getByRole('group', { name: 'Deadlift warm-ups' })
    expect(within(ramp).getByText('Bar × 10')).toBeTruthy()
    expect(within(ramp).getByText('80% × 2')).toBeTruthy()
    // Still 6 working sets: the ramp does not count.
    expect(screen.getByText('Set 1 of 6')).toBeTruthy()
  })

  it('asks the readiness questions once and remembers a skip', async () => {
    renderSession({ askReadiness: true })
    expect(await screen.findByRole('dialog', { name: 'Quick check-in' })).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: 'Skip' }))
    expect(screen.queryByRole('dialog', { name: 'Quick check-in' })).toBeNull()
    expect(localStorage.getItem('replog.readiness-dismissed.5')).toBe('1')
  })

  it('does not ask when the day already has a check-in', async () => {
    vi.mocked(api.getWorkout).mockResolvedValue(workout({
      readiness: { id: 1, workout_id: 9, athlete_id: 7, date: today, sleep_hours: 8, created_at: '', updated_at: '' },
    }))
    renderSession({ askReadiness: true })
    await screen.findByText('Set 1 of 6')
    expect(screen.queryByRole('dialog', { name: 'Quick check-in' })).toBeNull()
  })

  it('shows a swapped lift in place of the prescribed one, keeping the reps', async () => {
    vi.mocked(api.getWorkout).mockResolvedValue(workout({
      swaps: [{ id: 3, workout_id: 5, from_exercise_id: 4, from_exercise_name: 'Deadlift', to_exercise_id: 9, to_exercise_name: 'Trap Bar Deadlift', reason: 'Back tight', created_at: '' }],
    }))
    renderSession()
    const card = (await screen.findByRole('heading', { name: 'Trap Bar Deadlift' })).closest('section') as HTMLElement
    expect(within(card).getByText('Swapped from Deadlift · Back tight')).toBeTruthy()
    expect(within(card).getByRole('button', { name: /Undo/ })).toBeTruthy()
    // The 315 lb load belonged to the prescribed lift; the substitute starts empty.
    expect((within(card).getByLabelText('Trap Bar Deadlift set 1 weight') as HTMLInputElement).value).toBe('')
    expect((within(card).getByLabelText('Trap Bar Deadlift set 1 reps') as HTMLInputElement).value).toBe('3')
    expect(screen.queryByRole('heading', { name: 'Deadlift' })).toBeNull()
  })
})
