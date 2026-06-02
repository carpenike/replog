import { useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { useLocation } from 'react-router-dom'
import { api, ApiError } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardFooter, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Alert } from '@/components/ui/alert'

// Map the machine-readable reason the OIDC callback appends on failure
// (/login?error=...) to a friendly message. Anything unrecognized falls back
// to a generic line so we never echo a raw reason at the user.
function oidcErrorMessage(reason: string): string {
  switch (reason) {
    case 'state_mismatch':
    case 'nonce_mismatch':
      return 'Your sign-in session expired. Please try again.'
    case 'empty_sub':
    case 'user_resolve_failed':
      return 'We could not match your identity-provider account. Contact your coach.'
    default:
      return 'Sign-in with PocketID failed. Please try again.'
  }
}

export function Login() {
  const location = useLocation()
  const params = new URLSearchParams(location.search)
  const oidcError = params.get('error')

  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [showBreakGlass, setShowBreakGlass] = useState(false)
  const [error, setError] = useState(oidcError ? oidcErrorMessage(oidcError) : '')
  const [loading, setLoading] = useState(false)
  const queryClient = useQueryClient()

  const returnTo = params.get('returnTo') ?? '/'

  function handleOIDCLogin() {
    // Full-page navigation to the relying-party start endpoint (served by the
    // Go binary, proxied in dev). The browser then bounces to PocketID and
    // back to /auth/oidc/callback, which establishes the session.
    const target = returnTo && returnTo !== '/'
      ? `/auth/oidc/start?returnTo=${encodeURIComponent(returnTo)}`
      : '/auth/oidc/start'
    window.location.href = target
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError('')
    setLoading(true)

    try {
      await api.login(username, password)
      await queryClient.invalidateQueries({ queryKey: ['me'] })
      window.location.href = returnTo
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.message)
      } else {
        setError('Login failed. Please try again.')
      }
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="flex items-center justify-center min-h-screen bg-background">
      <Card className="w-full max-w-sm">
        <CardHeader className="text-center">
          <CardTitle className="text-2xl font-bold text-primary">RepLog</CardTitle>
        </CardHeader>

        <CardContent className="space-y-4">
          {error && (
            <Alert variant="destructive">
              {error}
            </Alert>
          )}

          <Button
            type="button"
            className="w-full"
            size="lg"
            disabled={loading}
            onClick={handleOIDCLogin}
          >
            Sign in with PocketID
          </Button>

          {showBreakGlass && (
            <form onSubmit={handleSubmit} className="space-y-4 border-t pt-4">
              <div className="grid gap-1.5">
                <Label htmlFor="username">Username</Label>
                <Input
                  id="username"
                  type="text"
                  autoComplete="username"
                  required
                  value={username}
                  onChange={e => setUsername(e.target.value)}
                />
              </div>

              <div className="grid gap-1.5">
                <Label htmlFor="password">Password</Label>
                <Input
                  id="password"
                  type="password"
                  autoComplete="current-password"
                  required
                  value={password}
                  onChange={e => setPassword(e.target.value)}
                />
              </div>

              <Button type="submit" disabled={loading} variant="secondary" className="w-full">
                {loading ? 'Signing in...' : 'Sign in with password'}
              </Button>
            </form>
          )}
        </CardContent>

        <CardFooter className="flex-col gap-2 text-center">
          <Button
            variant="link"
            size="sm"
            className="text-xs text-muted-foreground"
            onClick={() => setShowBreakGlass(v => !v)}
          >
            {showBreakGlass ? 'Hide password sign-in' : 'Sign in with a password'}
          </Button>
          <p className="text-xs text-muted-foreground">
            Workout tracking for the family
          </p>
        </CardFooter>
      </Card>
    </div>
  )
}
