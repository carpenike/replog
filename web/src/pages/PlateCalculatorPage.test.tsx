import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen } from '@testing-library/react'
import { createMemoryRouter, RouterProvider } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { api } from '@/api/client'
import { PlateCalculatorPage } from '@/pages/PlateCalculatorPage'

vi.mock('@/api/client', async () => {
  const actual = await vi.importActual<typeof import('@/api/client')>('@/api/client')
  return { ...actual, api: { me: vi.fn(), getPreferences: vi.fn(), getPlateSetup: vi.fn() } }
})

function renderPage() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const router = createMemoryRouter([{ path: '/plates', element: <PlateCalculatorPage /> }], { initialEntries: ['/plates'] })
  render(<QueryClientProvider client={queryClient}><RouterProvider router={router} /></QueryClientProvider>)
}

beforeEach(() => {
  sessionStorage.clear()
  vi.mocked(api.me).mockResolvedValue({ id: 1, username: 'ryan', athlete_id: 7, is_coach: true, is_admin: true } as Awaited<ReturnType<typeof api.me>>)
  vi.mocked(api.getPreferences).mockResolvedValue({ id: 1, user_id: 1, weight_unit: 'lbs', timezone: 'UTC', date_format: '2006-01-02', created_at: '', updated_at: '' })
  vi.mocked(api.getPlateSetup).mockResolvedValue({
    athlete_id: 7, bar: { weight: 20, unit: 'kg' }, collar: { weight: 2.5, unit: 'kg' }, use_collars: false, is_default: true,
    plates: [25, 20, 15, 10, 5, 2.5, 2, 1.5, 1, 0.5, 0.25, 0.125].map(w => ({ weight: w, unit: 'kg', pairs: w === 20 ? 2 : 1 })),
  })
})

describe('plate calculator page', () => {
  it('starts at the empty bar and adds up tapped kg plates in lb and kg', async () => {
    renderPage()
    expect(await screen.findByRole('heading', { name: 'Plate calculator' })).toBeTruthy()
    await screen.findByRole('button', { name: 'Add 0.125 kg' })
    expect(screen.getByTestId('plate-total').textContent).toContain('44.1')
    fireEvent.click(screen.getByRole('button', { name: 'Add 20 kg, 2 of 2 pairs left' }))
    fireEvent.click(screen.getByRole('button', { name: 'Add 15 kg' }))
    fireEvent.click(screen.getByRole('button', { name: 'Add 0.5 kg' }))
    // 20 + 2 × 35.5 = 91 kg
    expect(screen.getByTestId('plate-total').textContent).toContain('200.6')
    expect(screen.getByText('91 kg')).toBeTruthy()

    fireEvent.click(screen.getByRole('button', { name: 'Take off 15 kg' }))
    expect(screen.getByText('61 kg')).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: 'Take off the last plate' }))
    expect(screen.getByText('60 kg')).toBeTruthy()
  })

  it('counts collars when switched on', async () => {
    renderPage()
    await screen.findByRole('button', { name: 'Add 0.125 kg' })
    fireEvent.click(screen.getByRole('checkbox'))
    expect(screen.getByText('25 kg')).toBeTruthy()
  })
})
