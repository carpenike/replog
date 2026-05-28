import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { api, ApiError } from '@/api/client'
import { Spinner } from '@/components/ui'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Checkbox } from '@/components/ui/checkbox'
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
      {me && (
        <Card className="mt-8">
          <CardHeader>
            <CardTitle>Avatar</CardTitle>
            <CardDescription>Your profile photo appears in the sidebar and across the app.</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="flex flex-col items-center gap-4">
              {me.avatar_url ? (
                <img src={me.avatar_url} alt="Avatar" className="h-24 w-24 rounded-full object-cover ring-2 ring-border" />
              ) : (
                <div className="h-24 w-24 rounded-full bg-muted flex items-center justify-center text-4xl font-bold text-muted-foreground">
                  {(me.name ?? me.username).charAt(0).toUpperCase()}
                </div>
              )}
              <div className="flex gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  disabled={uploading}
                  onClick={() => {
                    const input = document.createElement('input')
                    input.type = 'file'
                    input.accept = 'image/*'
                    input.onchange = async () => {
                      const file = input.files?.[0]
                      if (!file) return
                      setUploading(true)
                      try {
                        await api.uploadAvatar(file)
                        queryClient.invalidateQueries({ queryKey: ['me'] })
                        toast.success('Avatar updated')
                      } catch (err) {
                        toast.error(err instanceof ApiError ? err.message : 'Upload failed')
                      } finally {
                        setUploading(false)
                      }
                    }
                    input.click()
                  }}
                >
                  {uploading ? 'Uploading...' : me.avatar_url ? 'Change photo' : 'Upload photo'}
                </Button>
                {me.avatar_url && (
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={async () => {
                      if (await confirm({ title: 'Remove Avatar', description: 'Delete your profile photo?', confirmLabel: 'Remove', variant: 'danger' })) {
                        try {
                          await api.deleteAvatar()
                          queryClient.invalidateQueries({ queryKey: ['me'] })
                          toast.success('Avatar removed')
                        } catch {
                          toast.error('Failed to remove avatar')
                        }
                      }
                    }}
                  >
                    Remove
                  </Button>
                )}
              </div>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Notification Preferences */}
      <NotificationPreferencesCard />

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

// Notification type labels — mirror models.AllNotificationTypes in Go.
// The API returns only `type`/`in_app`/`external`; labels live client-side
// so unknown types still render with a sensible fallback.
const NOTIFICATION_TYPE_META: Record<string, { label: string; description: string }> = {
  review_submitted:    { label: 'Workout Reviewed',       description: 'When a coach reviews your workout' },
  program_assigned:    { label: 'Program Assigned',       description: 'When a new program is assigned to you' },
  tm_updated:          { label: 'Training Max Updated',   description: 'When a training max is updated' },
  note_added:          { label: 'Coach Note Added',       description: 'When a coach adds a public note' },
  workout_logged:      { label: 'Workout Logged',         description: 'When an athlete logs a workout' },
  magic_link_sent:     { label: 'Login Link Sent',        description: 'When a login link is generated for you' },
  generation_complete: { label: 'AI Coach Draft Ready',   description: 'When an AI Coach program draft finishes generating' },
  generation_failed:   { label: 'AI Coach Draft Failed',  description: 'When an AI Coach program draft fails to generate' },
}

function NotificationPreferencesCard() {
  const queryClient = useQueryClient()
  const { data: prefs, isLoading } = useQuery({
    queryKey: ['notification-preferences'],
    queryFn: () => api.listNotificationPreferences(),
  })

  const updateMutation = useMutation({
    mutationFn: (vars: { type: string; in_app: boolean; external: boolean }) =>
      api.updateNotificationPreference(vars.type, vars.in_app, vars.external),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['notification-preferences'] }),
    onError: (err) => toast.error(err instanceof ApiError ? err.message : 'Failed to update preference'),
  })

  return (
    <Card className="mt-8">
      <CardHeader>
        <CardTitle>Notifications</CardTitle>
        <CardDescription>
          Choose which events alert you in-app and which also send to your configured external channel.
        </CardDescription>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <Spinner />
        ) : prefs && prefs.length > 0 ? (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Event</TableHead>
                <TableHead className="w-24 text-center">In-app</TableHead>
                <TableHead className="w-24 text-center">External</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {prefs.map(p => {
                const meta = NOTIFICATION_TYPE_META[p.type] ?? { label: p.type, description: '' }
                return (
                  <TableRow key={p.type}>
                    <TableCell>
                      <p className="font-medium">{meta.label}</p>
                      {meta.description && (
                        <p className="text-xs text-muted-foreground">{meta.description}</p>
                      )}
                    </TableCell>
                    <TableCell className="text-center">
                      <Checkbox
                        checked={p.in_app}
                        onCheckedChange={(checked) =>
                          updateMutation.mutate({ type: p.type, in_app: checked, external: p.external })
                        }
                      />
                    </TableCell>
                    <TableCell className="text-center">
                      <Checkbox
                        checked={p.external}
                        onCheckedChange={(checked) =>
                          updateMutation.mutate({ type: p.type, in_app: p.in_app, external: checked })
                        }
                      />
                    </TableCell>
                  </TableRow>
                )
              })}
            </TableBody>
          </Table>
        ) : (
          <p className="text-sm text-muted-foreground">No notification types available.</p>
        )}
      </CardContent>
    </Card>
  )
}
