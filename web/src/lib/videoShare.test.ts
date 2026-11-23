import { describe, expect, it } from 'vitest'
import type { VideoShare } from '@/api/types'
import {
  DEFAULT_SHARE_EXPIRY_HOURS,
  SHARE_EXPIRY_OPTIONS,
  absoluteShareURL,
  formatShareExpiry,
  isShareExpiry,
  shareStatusText,
  shareTokenFromPath,
  shareViewsText,
} from './videoShare'

describe('share expiry options', () => {
  it('offers 24h, 7d and 30d with 7d as the default', () => {
    expect(SHARE_EXPIRY_OPTIONS.map(o => o.hours)).toEqual([24, 168, 720])
    expect(DEFAULT_SHARE_EXPIRY_HOURS).toBe(168)
    expect(isShareExpiry(DEFAULT_SHARE_EXPIRY_HOURS)).toBe(true)
  })

  it('never offers more than 30 days', () => {
    expect(Math.max(...SHARE_EXPIRY_OPTIONS.map(o => o.hours))).toBeLessThanOrEqual(30 * 24)
    expect(isShareExpiry(721)).toBe(false)
    expect(isShareExpiry(48)).toBe(false)
  })
})

describe('share URLs', () => {
  it('builds the absolute link from the relative path', () => {
    expect(absoluteShareURL('/s/abc_DEF-123', 'https://replog.example.com')).toBe('https://replog.example.com/s/abc_DEF-123')
  })

  it('reads the token from a viewer path only', () => {
    expect(shareTokenFromPath('/s/abc_DEF-123')).toBe('abc_DEF-123')
    expect(shareTokenFromPath('/s/abc/')).toBe('abc')
    expect(shareTokenFromPath('/s/')).toBeNull()
    expect(shareTokenFromPath('/s/abc/video')).toBeNull()
    expect(shareTokenFromPath('/sets/abc')).toBeNull()
    expect(shareTokenFromPath('/s/%E0%A4%A')).toBeNull()
  })
})

describe('formatShareExpiry', () => {
  const now = new Date(2026, 10, 23, 9, 0)

  it('says today and tomorrow', () => {
    expect(formatShareExpiry(new Date(2026, 10, 23, 15, 0).toISOString(), now, 'en-US')).toBe('today at 3:00 PM')
    expect(formatShareExpiry(new Date(2026, 10, 24, 9, 30).toISOString(), now, 'en-US')).toBe('tomorrow at 9:30 AM')
  })

  it('names the date, with the year only when it differs', () => {
    expect(formatShareExpiry(new Date(2026, 10, 30, 9, 0).toISOString(), now, 'en-US')).toBe('Nov 30 at 9:00 AM')
    expect(formatShareExpiry(new Date(2027, 0, 2, 9, 0).toISOString(), now, 'en-US')).toBe('Jan 2, 2027 at 9:00 AM')
  })

  it('returns empty text for a bad date', () => {
    expect(formatShareExpiry('nope', now)).toBe('')
  })
})

describe('share list text', () => {
  const base: VideoShare = {
    id: 1, created_at: '2026-11-23T09:00:00Z', expires_at: new Date(2026, 10, 30, 9, 0).toISOString(),
    access_count: 0, status: 'active',
  }
  const now = new Date(2026, 10, 23, 9, 0)

  it('describes each status', () => {
    expect(shareStatusText(base, now, 'en-US')).toBe('Expires Nov 30 at 9:00 AM')
    expect(shareStatusText({ ...base, status: 'expired' }, now, 'en-US')).toBe('Expired Nov 30 at 9:00 AM')
    expect(shareStatusText({ ...base, status: 'revoked' }, now, 'en-US')).toBe('Revoked')
  })

  it('counts views', () => {
    expect(shareViewsText(0)).toBe('No views yet')
    expect(shareViewsText(1)).toBe('1 view')
    expect(shareViewsText(4)).toBe('4 views')
  })
})
