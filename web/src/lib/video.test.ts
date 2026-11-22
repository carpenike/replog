import { describe, expect, it } from 'vitest'
import { parseDemoVideo } from './video'

describe('parseDemoVideo', () => {
  it('embeds YouTube watch, short and youtu.be links with the no-cookie player', () => {
    const embed = 'https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ'
    for (const url of [
      'https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=30s',
      'https://youtu.be/dQw4w9WgXcQ',
      'https://m.youtube.com/shorts/dQw4w9WgXcQ',
      'https://www.youtube.com/embed/dQw4w9WgXcQ',
    ]) {
      expect(parseDemoVideo(url)).toMatchObject({ kind: 'youtube', embedUrl: embed })
    }
  })

  it('embeds Vimeo links', () => {
    expect(parseDemoVideo('https://vimeo.com/76979871')).toMatchObject({ kind: 'vimeo', embedUrl: 'https://player.vimeo.com/video/76979871' })
    expect(parseDemoVideo('https://player.vimeo.com/video/76979871?h=abc')).toMatchObject({ kind: 'vimeo' })
  })

  it('falls back to a link for other sites and malformed ids', () => {
    expect(parseDemoVideo('https://example.com/squat.mp4')).toEqual({ kind: 'link', url: 'https://example.com/squat.mp4' })
    expect(parseDemoVideo('https://www.youtube.com/watch?v=bad')).toMatchObject({ kind: 'link' })
  })

  it('drops empty, invalid and non-http URLs', () => {
    expect(parseDemoVideo('')).toBeNull()
    expect(parseDemoVideo(null)).toBeNull()
    expect(parseDemoVideo('not a url')).toBeNull()
    expect(parseDemoVideo('javascript:alert(1)')).toBeNull()
  })
})
