import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { api, ApiError } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card'
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

export function PasskeySetupPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [label, setLabel] = useState('')
  const [registering, setRegistering] = useState(false)
  const [done, setDone] = useState(false)

  async function handleRegister() {
    if (!window.PublicKeyCredential) {
      toast.error('Passkeys are not supported in this browser')
      return
    }

    setRegistering(true)
    try {
      if (label.trim()) {
        await api.setPasskeyLabel(label.trim())
      }

      const options = await api.beginPasskeyRegistration()
      const publicKey = options.publicKey

      const createOptions: CredentialCreationOptions = {
        publicKey: {
          rp: publicKey.rp,
          user: {
            id: base64urlToBuffer(publicKey.user.id),
            name: publicKey.user.name,
            displayName: publicKey.user.displayName,
          },
          challenge: base64urlToBuffer(publicKey.challenge),
          pubKeyCredParams: publicKey.pubKeyCredParams.map((p: { type: string; alg: number }) => ({
            type: p.type as 'public-key',
            alg: p.alg,
          })),
          timeout: publicKey.timeout,
          excludeCredentials: publicKey.excludeCredentials?.map((c: { type: string; id: string; transports?: string[] }) => ({
            type: c.type as 'public-key',
            id: base64urlToBuffer(c.id),
            transports: c.transports as AuthenticatorTransport[],
          })),
          authenticatorSelection: publicKey.authenticatorSelection as AuthenticatorSelectionCriteria,
          attestation: (publicKey.attestation ?? 'none') as AttestationConveyancePreference,
        },
      }

      const credential = await navigator.credentials.create(createOptions) as PublicKeyCredential
      if (!credential) return

      const response = credential.response as AuthenticatorAttestationResponse

      await api.finishPasskeyRegistration({
        id: credential.id,
        rawId: bufferToBase64url(credential.rawId),
        type: credential.type,
        response: {
          attestationObject: bufferToBase64url(response.attestationObject),
          clientDataJSON: bufferToBase64url(response.clientDataJSON),
        },
      })

      queryClient.invalidateQueries({ queryKey: ['passkeys'] })
      setDone(true)
    } catch (err) {
      if (err instanceof ApiError) {
        toast.error(err.message)
      } else if (err instanceof DOMException && err.name === 'NotAllowedError') {
        toast.error('Registration was cancelled or timed out')
      } else {
        toast.error('Failed to register passkey')
      }
    } finally {
      setRegistering(false)
    }
  }

  async function handleSkip() {
    try {
      await api.skipPasskeySetup()
    } catch {
      // Non-critical — just navigate anyway
    }
    navigate('/', { replace: true })
  }

  if (done) {
    return (
      <div className="flex items-center justify-center min-h-screen bg-background">
        <Card className="w-full max-w-sm">
          <CardHeader className="text-center">
            <CardTitle>Passkey Registered!</CardTitle>
            <CardDescription>
              Your passkey is ready. Next time you can sign in with just your fingerprint or face.
            </CardDescription>
          </CardHeader>
          <CardFooter className="justify-center">
            <Button onClick={() => navigate('/', { replace: true })}>
              Continue to Dashboard
            </Button>
          </CardFooter>
        </Card>
      </div>
    )
  }

  return (
    <div className="flex items-center justify-center min-h-screen bg-background">
      <Card className="w-full max-w-sm">
        <CardHeader className="text-center">
          <CardTitle>Set Up a Passkey</CardTitle>
          <CardDescription>
            Passkeys let you sign in securely with your fingerprint, face, or device PIN — no password needed.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {!window.PublicKeyCredential && (
            <Alert variant="warning">
              Your browser doesn't support passkeys. You can skip this step.
            </Alert>
          )}
          <div>
            <Label>Passkey Name</Label>
            <Input
              value={label}
              onChange={e => setLabel(e.target.value)}
              placeholder="e.g. MacBook Touch ID"
            />
          </div>
          <Button
            className="w-full"
            size="lg"
            onClick={handleRegister}
            disabled={registering || !window.PublicKeyCredential}
          >
            {registering ? 'Registering...' : 'Register Passkey'}
          </Button>
        </CardContent>
        <CardFooter className="justify-center">
          <Button variant="ghost" size="sm" onClick={handleSkip}>
            Skip for now
          </Button>
        </CardFooter>
      </Card>
    </div>
  )
}
