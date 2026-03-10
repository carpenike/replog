import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { api, ApiError } from '@/api/client'
import { Spinner } from '@/components/ui'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Alert } from '@/components/ui/alert'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { useConfirm } from '@/lib/useConfirm'

// WebAuthn helper: base64url encode
function bufferToBase64url(buffer: ArrayBuffer): string {
  const bytes = new Uint8Array(buffer)
  let str = ''
  for (const b of bytes) str += String.fromCharCode(b)
  return btoa(str).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
}

// WebAuthn helper: base64url decode
function base64urlToBuffer(base64url: string): ArrayBuffer {
  const base64 = base64url.replace(/-/g, '+').replace(/_/g, '/')
  const padded = base64 + '='.repeat((4 - (base64.length % 4)) % 4)
  const binary = atob(padded)
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i)
  return bytes.buffer
}

export function PreferencesPage() {
  const queryClient = useQueryClient()
  const { confirm, dialog: confirmDialog } = useConfirm()

  const { data: me } = useQuery({
    queryKey: ['me'],
    queryFn: () => api.me(),
  })

  const { data: athlete } = useQuery({
    queryKey: ['athlete', me?.athlete_id],
    queryFn: () => api.getAthlete(me!.athlete_id!),
    enabled: !!me?.athlete_id,
  })

  const { data: prefs, isLoading } = useQuery({
    queryKey: ['preferences'],
    queryFn: () => api.getPreferences(),
  })

  const { data: passkeys, isLoading: passkeysLoading } = useQuery({
    queryKey: ['passkeys'],
    queryFn: () => api.listPasskeys(),
  })

  const [weightUnit, setWeightUnit] = useState('')
  const [timezone, setTimezone] = useState('')
  const [dateFormat, setDateFormat] = useState('')
  const [saved, setSaved] = useState(false)
  const [passkeyLabel, setPasskeyLabel] = useState('')
  const [registering, setRegistering] = useState(false)
  const [uploading, setUploading] = useState(false)

  // Initialize form when data loads
  if (prefs && !weightUnit && !timezone) {
    setWeightUnit(prefs.weight_unit)
    setTimezone(prefs.timezone)
    setDateFormat(prefs.date_format)
  }

  const mutation = useMutation({
    mutationFn: () => api.updatePreferences({ weight_unit: weightUnit, timezone, date_format: dateFormat }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['preferences'] })
      setSaved(true)
      setTimeout(() => setSaved(false), 2000)
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => api.deletePasskey(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['passkeys'] })
      toast.success('Passkey deleted')
    },
  })

  async function handleRegisterPasskey() {
    if (!window.PublicKeyCredential) {
      toast.error('WebAuthn is not supported in this browser')
      return
    }

    setRegistering(true)
    try {
      // Set label first
      if (passkeyLabel.trim()) {
        await api.setPasskeyLabel(passkeyLabel.trim())
      }

      // Begin registration — get options from server
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const options: any = await api.beginPasskeyRegistration()

      // Convert base64url strings to ArrayBuffers for the browser API
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

      // Prompt user to create credential
      const credential = await navigator.credentials.create(createOptions) as PublicKeyCredential
      if (!credential) {
        toast.error('Registration cancelled')
        return
      }

      const response = credential.response as AuthenticatorAttestationResponse

      // Send credential to server
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
      setPasskeyLabel('')
      toast.success('Passkey registered successfully!')
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

  if (isLoading) return <Spinner />

  return (
    <div className="max-w-lg">
      <h1 className="text-2xl font-bold mb-6">Preferences</h1>

      <form onSubmit={(e) => { e.preventDefault(); mutation.mutate() }} className="space-y-4">
        {saved && (
          <Alert variant="success">
            Preferences saved
          </Alert>
        )}

        <div>
          <Label>Weight Unit</Label>
          <Select value={weightUnit} onValueChange={(val) => setWeightUnit(val ?? "lbs")}>
            <SelectTrigger className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="lbs">Pounds (lbs)</SelectItem>
              <SelectItem value="kg">Kilograms (kg)</SelectItem>
            </SelectContent>
          </Select>
        </div>

        <div>
          <Label>Timezone</Label>
          <Select value={timezone} onValueChange={(val) => setTimezone(val ?? "America/New_York")}>
            <SelectTrigger className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="America/New_York">Eastern (New York)</SelectItem>
              <SelectItem value="America/Chicago">Central (Chicago)</SelectItem>
              <SelectItem value="America/Denver">Mountain (Denver)</SelectItem>
              <SelectItem value="America/Los_Angeles">Pacific (Los Angeles)</SelectItem>
              <SelectItem value="UTC">UTC</SelectItem>
              <SelectItem value="Europe/London">Europe/London</SelectItem>
            </SelectContent>
          </Select>
        </div>

        <div>
          <Label>Date Format</Label>
          <Select value={dateFormat} onValueChange={(val) => setDateFormat(val ?? "Jan 2, 2006")}>
            <SelectTrigger className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="Jan 2, 2006">Jan 2, 2006</SelectItem>
              <SelectItem value="2006-01-02">2006-01-02</SelectItem>
              <SelectItem value="01/02/2006">01/02/2006</SelectItem>
              <SelectItem value="02/01/2006">02/01/2006</SelectItem>
            </SelectContent>
          </Select>
        </div>

        <Button type="submit" disabled={mutation.isPending}>
          {mutation.isPending ? 'Saving...' : 'Save Preferences'}
        </Button>
      </form>

      {/* Avatar */}
      {me?.athlete_id && (
        <Card className="mt-8">
          <CardHeader>
            <CardTitle>Avatar</CardTitle>
            <CardDescription>Upload a profile photo for your athlete profile.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="flex items-center gap-4">
              {athlete?.avatar_url ? (
                <img src={athlete.avatar_url} alt="Avatar" className="h-16 w-16 rounded-full object-cover" />
              ) : (
                <div className="h-16 w-16 rounded-full bg-muted flex items-center justify-center text-2xl">
                  {athlete?.name?.charAt(0)?.toUpperCase() ?? '?'}
                </div>
              )}
              <div className="flex-1 space-y-2">
                <Input
                  type="file"
                  accept="image/*"
                  disabled={uploading}
                  onChange={async (e) => {
                    const file = e.target.files?.[0]
                    if (!file) return
                    setUploading(true)
                    try {
                      await api.uploadAvatar(file)
                      queryClient.invalidateQueries({ queryKey: ['athlete', me.athlete_id] })
                      queryClient.invalidateQueries({ queryKey: ['me'] })
                      toast.success('Avatar updated')
                    } catch (err) {
                      toast.error(err instanceof ApiError ? err.message : 'Upload failed')
                    } finally {
                      setUploading(false)
                      e.target.value = ''
                    }
                  }}
                />
                {athlete?.avatar_url && (
                  <Button
                    variant="ghost"
                    size="xs"
                    onClick={async () => {
                      if (await confirm({ title: 'Remove Avatar', description: 'Delete your profile photo?', confirmLabel: 'Remove', variant: 'danger' })) {
                        try {
                          await api.deleteAvatar()
                          queryClient.invalidateQueries({ queryKey: ['athlete', me.athlete_id] })
                          toast.success('Avatar removed')
                        } catch {
                          toast.error('Failed to remove avatar')
                        }
                      }
                    }}
                    className="text-muted-foreground hover:text-destructive"
                  >
                    Remove avatar
                  </Button>
                )}
              </div>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Passkey Management */}
      <Card className="mt-8">
        <CardHeader>
          <CardTitle>Passkeys</CardTitle>
          <CardDescription>
            Manage your passkey credentials for passwordless login. Passkeys use your device's biometric or PIN to sign in securely.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {/* Existing passkeys */}
          {passkeysLoading ? (
            <Spinner />
          ) : passkeys && passkeys.length > 0 ? (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Created</TableHead>
                  <TableHead>Last Used</TableHead>
                  <TableHead>Uses</TableHead>
                  <TableHead className="w-12"></TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {passkeys.map(pk => (
                  <TableRow key={pk.id}>
                    <TableCell className="font-medium">{pk.label ?? 'Unnamed passkey'}</TableCell>
                    <TableCell className="text-muted-foreground">{new Date(pk.created_at).toLocaleDateString()}</TableCell>
                    <TableCell className="text-muted-foreground">{pk.last_used_at ? new Date(pk.last_used_at).toLocaleDateString() : 'Never'}</TableCell>
                    <TableCell className="text-muted-foreground">{pk.use_count}</TableCell>
                    <TableCell>
                      <Button
                        variant="ghost"
                        size="xs"
                        onClick={async () => {
                          if (await confirm({ title: 'Delete Passkey', description: 'This passkey will no longer work for login.', confirmLabel: 'Delete', variant: 'danger' }))
                            deleteMutation.mutate(pk.id)
                        }}
                      >
                        ×
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          ) : (
            <p className="text-sm text-muted-foreground">No passkeys registered yet.</p>
          )}

          {/* Register new passkey */}
          <div className="flex gap-2 items-end">
            <div className="flex-1">
              <Label>New Passkey Name</Label>
              <Input
                value={passkeyLabel}
                onChange={e => setPasskeyLabel(e.target.value)}
                placeholder="e.g. MacBook Touch ID"
              />
            </div>
            <Button onClick={handleRegisterPasskey} disabled={registering}>
              {registering ? 'Registering...' : '+ Add Passkey'}
            </Button>
          </div>
        </CardContent>
      </Card>
      {confirmDialog()}
    </div>
  )
}
