import { describe, expect, it } from 'vitest'
import type { ServiceTokenData } from '@/api/types'
import { serviceTokenScope, visibleServiceTokens } from '@/lib/serviceTokens'

function tok(id: number, revoked = false): ServiceTokenData {
  return {
    id,
    user_id: 1,
    label: `t${id}`,
    created_at: '2026-11-01T00:00:00Z',
    last_used_at: null,
    revoked_at: revoked ? '2026-11-02T00:00:00Z' : null,
  }
}

describe('visibleServiceTokens', () => {
  const tokens = [tok(1, true), tok(2), tok(3, true), tok(4)]

  it('hides revoked tokens by default and lists newest first', () => {
    expect(visibleServiceTokens(tokens, false).map(t => t.id)).toEqual([4, 2])
  })

  it('appends revoked tokens after the active ones when shown', () => {
    expect(visibleServiceTokens(tokens, true).map(t => t.id)).toEqual([4, 2, 3, 1])
  })
})

describe('serviceTokenScope', () => {
  it('names the linked athlete and coached athletes', () => {
    expect(serviceTokenScope({ is_admin: false, is_coach: true, athlete_id: 3 }, 'Ryan'))
      .toBe('Acts as this user: Ryan (linked athlete) and the athletes they coach.')
    expect(serviceTokenScope({ is_admin: false, is_coach: false, athlete_id: 3 }, null))
      .toBe('Acts as this user: their linked athlete.')
  })

  it('says admin rights do not carry over to tokens', () => {
    expect(serviceTokenScope({ is_admin: true, is_coach: false, athlete_id: null }))
      .toBe('Acts as this user: no athlete data (no linked athlete, not a coach). Admin rights do not apply to tokens.')
  })
})
