import { describe, expect, it } from 'vitest'
import { checkClip, formatBytes } from './formCheck'

const MB = 1024 * 1024

describe('formatBytes', () => {
  it('scales units', () => {
    expect(formatBytes(512)).toBe('512 B')
    expect(formatBytes(2048)).toBe('2 KB')
    expect(formatBytes(4.2 * MB)).toBe('4.2 MB')
  })
})

describe('checkClip', () => {
  it('accepts phone video types', () => {
    expect(checkClip({ name: 'IMG_0001.MOV', type: 'video/quicktime', size: 20 * MB })).toBeNull()
    expect(checkClip({ name: 'squat.mp4', type: 'video/mp4', size: MB })).toBeNull()
    expect(checkClip({ name: 'clip.webm', type: 'video/webm', size: MB })).toBeNull()
  })

  it('falls back to the extension when the type is missing', () => {
    expect(checkClip({ name: 'clip.mov', type: '', size: MB })).toBeNull()
    expect(checkClip({ name: 'clip.gif', type: '', size: MB })).toMatch(/MP4, MOV or WebM/)
  })

  it('rejects other types, empty files and oversize clips', () => {
    expect(checkClip({ name: 'photo.jpg', type: 'image/jpeg', size: MB })).toMatch(/MP4, MOV or WebM/)
    expect(checkClip({ name: 'trick.mp4', type: 'text/html', size: MB })).toMatch(/MP4, MOV or WebM/)
    expect(checkClip({ name: 'a.mp4', type: 'video/mp4', size: 0 })).toMatch(/empty/)
    expect(checkClip({ name: 'long.mov', type: 'video/quicktime', size: 150 * MB })).toMatch(/limit is 100.0 MB/)
    expect(checkClip({ name: 'a.mp4', type: 'video/mp4', size: 6 * MB }, 5 * MB)).toMatch(/limit/)
  })
})
