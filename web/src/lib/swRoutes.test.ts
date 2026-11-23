import { describe, expect, it } from 'vitest'
import { isSetVideoPath, isSharePath } from './swRoutes'

describe('isSetVideoPath', () => {
  it('matches clip routes only', () => {
    expect(isSetVideoPath('/api/athletes/1/workouts/2/sets/3/video')).toBe(true)
    expect(isSetVideoPath('/api/athletes/1/workouts/2/sets/3/video/comment')).toBe(true)
    expect(isSetVideoPath('/api/athletes/1/workouts/2/sets/3')).toBe(false)
    expect(isSetVideoPath('/api/athletes/1/workouts/2/sets')).toBe(false)
    expect(isSetVideoPath('/api/athletes/1/videos')).toBe(false)
  })
})

describe('isSharePath', () => {
  it('matches the viewer page and the public share API', () => {
    expect(isSharePath('/s/abc')).toBe(true)
    expect(isSharePath('/api/share/abc')).toBe(true)
    expect(isSharePath('/api/share/abc/video')).toBe(true)
  })

  it('leaves app routes alone', () => {
    expect(isSharePath('/sets')).toBe(false)
    expect(isSharePath('/settings')).toBe(false)
    expect(isSharePath('/api/shares')).toBe(false)
    expect(isSharePath('/athletes/1/workouts/2')).toBe(false)
    // Owner-side share management is under .../video and skipped by isSetVideoPath.
    expect(isSetVideoPath('/api/athletes/1/workouts/2/sets/3/video/shares')).toBe(true)
    expect(isSetVideoPath('/api/athletes/1/workouts/2/sets/3/video/shares/9')).toBe(true)
  })
})
