import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { CreatedVideoShare, VideoShare, VideoShareList } from '@/api/types'

const listVideoShares = vi.fn<(a: number, w: number, s: number, inactive?: boolean) => Promise<VideoShareList>>()
const createVideoShare = vi.fn<(a: number, w: number, s: number, body: { expires_in_hours: number; label?: string }) => Promise<CreatedVideoShare>>()
const revokeVideoShare = vi.fn<(a: number, w: number, s: number, id: number) => Promise<VideoShare>>()

vi.mock('@/api/client', () => ({
  api: {
    listVideoShares: (...args: Parameters<typeof listVideoShares>) => listVideoShares(...args),
    createVideoShare: (...args: Parameters<typeof createVideoShare>) => createVideoShare(...args),
    revokeVideoShare: (...args: Parameters<typeof revokeVideoShare>) => revokeVideoShare(...args),
  },
}))

const { VideoShareSheet } = await import('./VideoShareSheet')

const activeShare: VideoShare = {
  id: 7, label: 'Grandpa', created_by: 'coach', created_at: '2026-11-23T09:00:00Z',
  expires_at: '2026-11-30T09:00:00Z', access_count: 3, status: 'active',
}

function renderSheet() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <VideoShareSheet open onOpenChange={() => {}} athleteId={1} workoutId={2} setId={3} setNumber={1} exerciseName="Back Squat" />
    </QueryClientProvider>,
  )
}

describe('VideoShareSheet', () => {
  beforeEach(() => {
    listVideoShares.mockResolvedValue({ sharing_enabled: true, shares: [] })
  })
  afterEach(() => {
    vi.clearAllMocks()
  })

  it('creates a link with the chosen expiry and label, and shows it once with Copy', async () => {
    const user = userEvent.setup()
    createVideoShare.mockResolvedValue({
      share: { ...activeShare, id: 9 }, token: 'tok_123', url: '/s/tok_123',
    })
    renderSheet()
    expect(await screen.findByText('No active links.')).toBeTruthy()
    expect(screen.getByRole('radio', { name: '7 days' }).getAttribute('aria-checked')).toBe('true')

    await user.click(screen.getByRole('radio', { name: '24 hours' }))
    await user.type(screen.getByLabelText('Label (optional)'), '  Grandpa ')
    await user.click(screen.getByRole('button', { name: /Create link/ }))

    await waitFor(() => expect(createVideoShare).toHaveBeenCalledWith(1, 2, 3, { expires_in_hours: 24, label: 'Grandpa' }))
    const link = await screen.findByLabelText('Your new link') as HTMLInputElement
    expect(link.value).toBe(`${window.location.origin}/s/tok_123`)
    expect(screen.getByText(/won't be shown again/)).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Copy share link' })).toBeTruthy()

    // Moving on hides the token for good.
    await user.click(screen.getByRole('button', { name: /Create another link/ }))
    expect(screen.queryByLabelText('Your new link')).toBeNull()
    expect(screen.queryByDisplayValue(/tok_123/)).toBeNull()
  })

  it('lists active links and revokes one after confirming', async () => {
    const user = userEvent.setup()
    listVideoShares.mockResolvedValue({ sharing_enabled: true, shares: [activeShare] })
    revokeVideoShare.mockResolvedValue({ ...activeShare, status: 'revoked', revoked_at: '2026-11-24T09:00:00Z' })
    renderSheet()
    expect(await screen.findByText('Grandpa')).toBeTruthy()
    expect(screen.getByText(/3 views · by coach/)).toBeTruthy()

    await user.click(screen.getByRole('button', { name: 'Revoke Grandpa' }))
    expect(revokeVideoShare).not.toHaveBeenCalled()
    expect(await screen.findByText(/stops working right away/)).toBeTruthy()
    const buttons = screen.getAllByRole('button', { name: 'Revoke' })
    await user.click(buttons[buttons.length - 1])
    await waitFor(() => expect(revokeVideoShare).toHaveBeenCalledWith(1, 2, 3, 7))
  })

  it('asks for inactive links when the toggle is on', async () => {
    const user = userEvent.setup()
    renderSheet()
    await screen.findByText('No active links.')
    await user.click(screen.getByLabelText('Show expired and revoked'))
    await waitFor(() => expect(listVideoShares).toHaveBeenCalledWith(1, 2, 3, true))
  })

  it('disables creating a link when sharing is turned off', async () => {
    listVideoShares.mockResolvedValue({ sharing_enabled: false, shares: [] })
    renderSheet()
    expect(await screen.findByText(/Sharing is turned off/)).toBeTruthy()
    expect((screen.getByRole('button', { name: /Create link/ }) as HTMLButtonElement).disabled).toBe(true)
  })
})
