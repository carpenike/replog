import type { ServiceTokenData } from '@/api/types'

interface TokenOwner {
  is_admin: boolean
  is_coach: boolean
  athlete_id?: number | null
}

/**
 * What an rlcst_ service token reaches. Tokens carry no scope or expiry of
 * their own: a bearer acts as the owning user until it is revoked. Mirrors
 * middleware.ServiceCanAccessAthlete: the owner's linked athlete, plus the
 * athletes they coach when they are a coach. Admin rights do not carry over.
 */
export function serviceTokenScope(owner: TokenOwner, linkedAthleteName?: string | null): string {
  const parts: string[] = []
  if (owner.athlete_id != null) parts.push(linkedAthleteName ? `${linkedAthleteName} (linked athlete)` : 'their linked athlete')
  if (owner.is_coach) parts.push('the athletes they coach')
  const reach = parts.length > 0 ? parts.join(' and ') : 'no athlete data (no linked athlete, not a coach)'
  const admin = owner.is_admin ? ' Admin rights do not apply to tokens.' : ''
  return `Acts as this user: ${reach}.${admin}`
}

/** Active tokens first (newest first), then revoked; revoked only when asked. */
export function visibleServiceTokens(tokens: ServiceTokenData[], showRevoked: boolean): ServiceTokenData[] {
  const active = tokens.filter(t => !t.revoked_at).sort((a, b) => b.id - a.id)
  if (!showRevoked) return active
  const revoked = tokens.filter(t => t.revoked_at).sort((a, b) => b.id - a.id)
  return [...active, ...revoked]
}
