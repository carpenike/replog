import { describe, expect, it } from 'vitest'
import { isSetVideoPath } from './swRoutes'

describe('isSetVideoPath', () => {
  it('matches clip routes only', () => {
    expect(isSetVideoPath('/api/athletes/1/workouts/2/sets/3/video')).toBe(true)
    expect(isSetVideoPath('/api/athletes/1/workouts/2/sets/3/video/comment')).toBe(true)
    expect(isSetVideoPath('/api/athletes/1/workouts/2/sets/3')).toBe(false)
    expect(isSetVideoPath('/api/athletes/1/workouts/2/sets')).toBe(false)
    expect(isSetVideoPath('/api/athletes/1/videos')).toBe(false)
  })
})
