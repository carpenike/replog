import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { SharedVideo } from '@/api/types'

const getSharedVideo = vi.fn<(token: string) => Promise<SharedVideo>>()
const me = vi.fn()

vi.mock('@/api/client', () => ({
  api: {
    getSharedVideo: (token: string) => getSharedVideo(token),
    me: () => me(),
  },
}))

const { App } = await import('@/App')

function renderAt(path: string) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[path]}>
        <App />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('shared video viewer', () => {
  afterEach(() => vi.clearAllMocks())

  it('plays the clip with only the exercise, date and expiry, without the auth bootstrap', async () => {
    getSharedVideo.mockResolvedValue({
      exercise_name: 'Back Squat', date: '2026-11-20', expires_at: '2026-11-30T15:00:00Z',
      content_type: 'video/mp4', video_url: '/api/share/tok_abc/video',
    })
    const { container } = renderAt('/s/tok_abc')
    expect(await screen.findByRole('heading', { name: 'Back Squat' })).toBeTruthy()
    expect(getSharedVideo).toHaveBeenCalledWith('tok_abc')
    expect(me).not.toHaveBeenCalled()
    expect(container.querySelector('video')?.getAttribute('src')).toBe('/api/share/tok_abc/video')
    expect(screen.getByText(/^Shared via RepLog · expires /)).toBeTruthy()
    expect(screen.getByText(/Nov 20/)).toBeTruthy()
    // Nothing links back into the app, and no login form appears.
    expect(container.querySelectorAll('a')).toHaveLength(0)
    expect(screen.queryByLabelText(/password/i)).toBeNull()
  })

  it('shows one message for any link that does not open', async () => {
    getSharedVideo.mockRejectedValue(Object.assign(new Error('This link is not available.'), { code: 404 }))
    renderAt('/s/whatever')
    expect(await screen.findByRole('heading', { name: 'Link not available' })).toBeTruthy()
    expect(screen.getByText('This link has expired or been revoked.')).toBeTruthy()
    expect(me).not.toHaveBeenCalled()
  })
})
