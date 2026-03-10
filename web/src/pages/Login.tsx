import { useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { useLocation } from 'react-router-dom'
import { api, ApiError } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardFooter, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Alert } from '@/components/ui/alert'

// WebAuthn helpers
function bufferToBase64url(buffer: ArrayBuffer): string {
  const bytes = new Uint8Array(buffer)
  let str = ''
  for (const b of bytes) str += String.fromCharCode(b)
  return btoa(str).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
}

function base64urlToBuffer(base64url: string): ArrayBuffer {
  const base64 = base64url.replace(/-/g, '+').replace(/_/g, '/')
  const padded = base64 + '='.repeat((4 - (base64.length % 4)) % 4)
  const binary = atob(padded)
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i)
  return bytes.buffer
}

export function Login() {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const queryClient = useQueryClient()
  const location = useLocation()

  const returnTo = new URLSearchParams(location.search).get('returnTo') ?? '/'

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

  async function handlePasskeyLogin() {
    if (!window.PublicKeyCredential) {
      setError('Passkeys are not supported in this browser')
      return
    }

    setError('')
    setLoading(true)

    try {
      const options = await api.beginPasskeyLogin()

      const publicKey = options.publicKey
      const getOptions: CredentialRequestOptions = {
        publicKey: {
          challenge: base64urlToBuffer(publicKey.challenge),
          timeout: publicKey.timeout,
          rpId: publicKey.rpId,
          allowCredentials: publicKey.allowCredentials?.map((c: { type: string; id: string; transports?: string[] }) => ({
            type: c.type as 'public-key',
            id: base64urlToBuffer(c.id),
            transports: c.transports as AuthenticatorTransport[],
          })),
          userVerification: publicKey.userVerification as UserVerificationRequirement,
        },
      }

      const credential = await navigator.credentials.get(getOptions) as PublicKeyCredential
      if (!credential) {
        setError('Passkey login cancelled')
        return
      }

      const response = credential.response as AuthenticatorAssertionResponse

      await api.finishPasskeyLogin({
        id: credential.id,
        rawId: bufferToBase64url(credential.rawId),
        type: credential.type,
        response: {
          authenticatorData: bufferToBase64url(response.authenticatorData),
          clientDataJSON: bufferToBase64url(response.clientDataJSON),
          signature: bufferToBase64url(response.signature),
          userHandle: response.userHandle ? bufferToBase64url(response.userHandle) : '',
        },
      })

      await queryClient.invalidateQueries({ queryKey: ['me'] })
      window.location.href = returnTo
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.message)
      } else if (err instanceof DOMException && err.name === 'NotAllowedError') {
        setError('Passkey login was cancelled or timed out')
      } else {
        setError('Passkey login failed. Please try again.')
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

        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-4">
            {error && (
              <Alert variant="destructive">
                {error}
              </Alert>
            )}

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

            <Button type="submit" disabled={loading} className="w-full" size="lg">
              {loading ? 'Signing in...' : 'Sign in'}
            </Button>
          </form>
        </CardContent>

        <CardFooter className="flex-col gap-2 text-center">
          <Button
            variant="outline"
            size="sm"
            disabled={loading}
            onClick={handlePasskeyLogin}
          >
            Sign in with Passkey
          </Button>
          <p className="text-xs text-muted-foreground">
            Workout tracking for the family
          </p>
        </CardFooter>
      </Card>
    </div>
  )
}
