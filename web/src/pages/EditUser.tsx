import { useState } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { X } from 'lucide-react'
import { api, ApiError } from '@/api/client'
import type { AthleteCard, User } from '@/api/types'
import { serviceTokenScope, visibleServiceTokens } from '@/lib/serviceTokens'
import { Spinner } from '@/components/ui'
import { useConfirm } from '@/lib/useConfirm'
import { Button, buttonVariants } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Alert } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

export function EditUser() {
  const { userId } = useParams<{ userId: string }>()
  const id = Number(userId)
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const { data: user, isLoading } = useQuery({
    queryKey: ['user', id],
    queryFn: () => api.getUser(id),
    enabled: !isNaN(id),
  })

  const { data: athletes } = useQuery({
    queryKey: ['athletes'],
    queryFn: () => api.listAthletes(),
  })

  const [username, setUsername] = useState('')
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [isCoach, setIsCoach] = useState(false)
  const [isAdmin, setIsAdmin] = useState(false)
  const [athleteId, setAthleteId] = useState('')
  const [initialized, setInitialized] = useState(false)
  const [error, setError] = useState('')

  if (user && !initialized) {
    setUsername(user.username)
    setName(user.name ?? '')
    setEmail(user.email ?? '')
    setIsCoach(user.is_coach)
    setIsAdmin(user.is_admin)
    setAthleteId(user.athlete_id ? String(user.athlete_id) : '')
    setInitialized(true)
  }

  const mutation = useMutation({
    mutationFn: () => api.updateUser(id, {
      username,
      name: name || undefined,
      email: email || undefined,
      password: password || undefined,
      is_coach: isCoach,
      is_admin: isAdmin,
      athlete_id: athleteId ? Number(athleteId) : null,
    }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] })
      navigate('/users')
    },
    onError: (err) => {
      setError(err instanceof ApiError ? err.message : 'Failed to update user')
    },
  })

  if (isLoading) return <Spinner />

  return (
    <div className="max-w-lg">
      <p className="text-sm text-muted-foreground mb-1">
        <Link to="/users" className="hover:text-foreground">Users</Link> / Edit
      </p>
      <h1 className="text-2xl font-bold mb-6">Edit User</h1>

      <form onSubmit={(e) => { e.preventDefault(); mutation.mutate() }} className="space-y-4">
        {error && (
          <Alert variant="destructive">{error}</Alert>
        )}

        <div>
          <Label htmlFor="username">Username *</Label>
          <Input id="username" type="text" value={username} onChange={e => setUsername(e.target.value)} required />
        </div>

        <div>
          <Label htmlFor="name">Display Name</Label>
          <Input id="name" type="text" value={name} onChange={e => setName(e.target.value)} />
        </div>

        <div>
          <Label htmlFor="email">Email</Label>
          <Input id="email" type="email" value={email} onChange={e => setEmail(e.target.value)} />
        </div>

        <div>
          <Label htmlFor="password">New Password</Label>
          <Input id="password" type="password" value={password} onChange={e => setPassword(e.target.value)} autoComplete="new-password" placeholder="Leave empty to keep current" />
        </div>

        <div>
          <Label>Linked Athlete</Label>
          <Select value={athleteId || null} onValueChange={(val) => setAthleteId(val ?? '')}>
            <SelectTrigger className="w-full">
              <SelectValue placeholder="None">
                {(value: string | null) => {
                  if (!value) return 'None'
                  const match = athletes?.find(a => String(a.id) === value)
                  return match?.name ?? value
                }}
              </SelectValue>
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="">None</SelectItem>
              {athletes?.map(a => (
                <SelectItem key={a.id} value={String(a.id)}>{a.name}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className="flex gap-4">
          <Label>
            <Checkbox checked={isCoach} onCheckedChange={(checked) => setIsCoach(checked)} />
            Coach
          </Label>
          <Label>
            <Checkbox checked={isAdmin} onCheckedChange={(checked) => setIsAdmin(checked)} />
            Admin
          </Label>
        </div>

        <div className="flex gap-3 pt-2">
          <Button type="submit" disabled={mutation.isPending}>
            {mutation.isPending ? 'Saving...' : 'Save Changes'}
          </Button>
          <Link to="/users" className={buttonVariants({ variant: "outline" })}>
            Cancel
          </Link>
        </div>
      </form>

      {/* Magic-link login tokens */}
      <LoginTokensSection userId={id} />
      <ServiceTokensSection userId={id} owner={user} athletes={athletes ?? []} />
    </div>
  )
}

function LoginTokensSection({ userId }: { userId: number }) {
  const queryClient = useQueryClient()
  const { confirm, dialog: confirmDialog } = useConfirm()
  const [label, setLabel] = useState('')
  const [issuedToken, setIssuedToken] = useState<{ url: string; label: string } | null>(null)

  const { data: tokens, isLoading } = useQuery({
    queryKey: ['login-tokens', userId],
    queryFn: () => api.listLoginTokens(userId),
    enabled: !isNaN(userId),
  })

  const createMutation = useMutation({
    mutationFn: () => api.createLoginToken(userId, label || undefined),
    onSuccess: (res) => {
      queryClient.invalidateQueries({ queryKey: ['login-tokens', userId] })
      setIssuedToken({ url: window.location.origin + res.url, label: label || 'Unnamed' })
      setLabel('')
    },
    onError: (err) => toast.error(err instanceof ApiError ? err.message : 'Failed to create token'),
  })

  const deleteMutation = useMutation({
    mutationFn: (tokenId: number) => api.deleteLoginToken(userId, tokenId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['login-tokens', userId] }),
  })

  return (
    <section className="mt-10 border-t pt-6">
      <h2 className="text-lg font-semibold">Login Tokens</h2>
      <p className="text-sm text-muted-foreground mt-1 mb-4 max-w-md">
        Single-use magic-link URLs for this user. The token bytes are shown
        once when issued — share the URL out-of-band and revoke when used.
      </p>

      {issuedToken && (
        <Alert className="mb-4">
          <div className="space-y-2">
            <p className="font-medium">New login link issued ({issuedToken.label})</p>
            <p className="text-xs text-muted-foreground">
              Copy this now — the token will not be shown again. The user can sign in once via this URL.
            </p>
            <div className="flex gap-2 items-center">
              <Input readOnly value={issuedToken.url} onFocus={(e) => e.currentTarget.select()} />
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => {
                  navigator.clipboard.writeText(issuedToken.url).then(
                    () => toast.success('Copied to clipboard'),
                    () => toast.error('Copy failed'),
                  )
                }}
              >
                Copy
              </Button>
              <Button type="button" variant="ghost" size="sm" onClick={() => setIssuedToken(null)}>
                Dismiss
              </Button>
            </div>
          </div>
        </Alert>
      )}

      <form
        onSubmit={(e) => { e.preventDefault(); createMutation.mutate() }}
        className="flex gap-2 items-end mb-4 max-w-md"
      >
        <div className="flex-1">
          <Label htmlFor="login-token-label">New token label (optional)</Label>
          <Input
            id="login-token-label"
            value={label}
            onChange={e => setLabel(e.target.value)}
            placeholder="e.g. iPad setup"
          />
        </div>
        <Button type="submit" disabled={createMutation.isPending}>
          {createMutation.isPending ? 'Issuing...' : 'Issue Link'}
        </Button>
      </form>

      {isLoading ? (
        <Spinner />
      ) : tokens && tokens.length > 0 ? (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Label</TableHead>
              <TableHead>Created</TableHead>
              <TableHead>Expires</TableHead>
              <TableHead>Status</TableHead>
              <TableHead className="w-12"></TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {tokens.map(t => (
              <TableRow key={t.id}>
                <TableCell className="font-medium">{t.label ?? '—'}</TableCell>
                <TableCell className="text-muted-foreground">{new Date(t.created_at).toLocaleDateString()}</TableCell>
                <TableCell className="text-muted-foreground">{t.expires_at ? new Date(t.expires_at).toLocaleDateString() : 'Never'}</TableCell>
                <TableCell>
                  {t.expired
                    ? <Badge variant="secondary">Expired</Badge>
                    : <Badge>Active</Badge>}
                </TableCell>
                <TableCell>
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    aria-label={`Revoke login link ${t.label ?? t.id}`}
                    className="hover:text-destructive"
                    onClick={async () => {
                      if (await confirm({
                        title: 'Revoke Token',
                        description: 'This login link will stop working immediately.',
                        confirmLabel: 'Revoke',
                        variant: 'danger',
                      })) deleteMutation.mutate(t.id)
                    }}
                  >
                    <X aria-hidden="true" />
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      ) : (
        <p className="text-sm text-muted-foreground">No active login tokens.</p>
      )}
      {confirmDialog()}
    </section>
  )
}

function ServiceTokensSection({ userId, owner, athletes }: {
  userId: number
  owner: User | undefined
  athletes: AthleteCard[]
}) {
  const queryClient = useQueryClient()
  const { confirm, dialog: confirmDialog } = useConfirm()
  const [label, setLabel] = useState('')
  const [issuedToken, setIssuedToken] = useState<string | null>(null)
  const [showRevoked, setShowRevoked] = useState(false)

  const { data: tokens, isLoading } = useQuery({
    queryKey: ['service-tokens', userId],
    queryFn: () => api.listServiceTokens(userId),
    enabled: !isNaN(userId),
  })

  const createMutation = useMutation({
    mutationFn: () => api.issueServiceToken(userId, label || undefined),
    onSuccess: (res) => {
      queryClient.invalidateQueries({ queryKey: ['service-tokens', userId] })
      setIssuedToken(res.token)
      setLabel('')
    },
    meta: { skipGlobalError: true },
    onError: (err) => toast.error(err instanceof ApiError ? err.message : 'Failed to issue token'),
  })

  const revokeMutation = useMutation({
    mutationFn: (tokenId: number) => api.revokeServiceToken(userId, tokenId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['service-tokens', userId] })
      toast.success('Service token revoked.')
    },
    meta: { skipGlobalError: true },
    onError: (err) => toast.error(err instanceof ApiError ? err.message : 'Failed to revoke token'),
  })

  const all = tokens ?? []
  const revokedCount = all.filter(t => t.revoked_at).length
  const shown = visibleServiceTokens(all, showRevoked)
  const linkedName = owner?.athlete_id != null ? athletes.find(a => a.id === owner.athlete_id)?.name : null

  return (
    <section className="mt-10 border-t pt-6">
      <h2 className="text-lg font-semibold">Service Tokens</h2>
      <p className="text-sm text-muted-foreground mt-1 mb-3 max-w-md">
        Long-lived bearers for this user. The token is shown once, when it is
        issued. Revoke it here when it should stop working.
      </p>

      <dl className="mb-4 grid max-w-md grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-sm">
        <dt className="text-muted-foreground">Expires</dt>
        <dd>Never. A token works until it is revoked.</dd>
        <dt className="text-muted-foreground">Scope</dt>
        <dd>{owner ? serviceTokenScope(owner, linkedName) : '—'}</dd>
      </dl>

      {issuedToken && (
        <Alert className="mb-4">
          <div className="space-y-2">
            <p className="font-medium">Service token issued</p>
            <p className="text-xs text-muted-foreground">
              Copy this now. It will not be shown again.
            </p>
            <div className="flex gap-2 items-center">
              <Input readOnly aria-label="New service token" value={issuedToken} onFocus={(e) => e.currentTarget.select()} />
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => {
                  navigator.clipboard.writeText(issuedToken).then(
                    () => toast.success('Copied to clipboard'),
                    () => toast.error('Copy failed'),
                  )
                }}
              >
                Copy
              </Button>
              <Button type="button" variant="ghost" size="sm" onClick={() => setIssuedToken(null)}>
                Dismiss
              </Button>
            </div>
          </div>
        </Alert>
      )}

      <form
        onSubmit={(e) => { e.preventDefault(); createMutation.mutate() }}
        className="flex gap-2 items-end mb-4 max-w-md"
      >
        <div className="flex-1">
          <Label htmlFor="service-token-label">New token label (optional)</Label>
          <Input
            id="service-token-label"
            value={label}
            onChange={e => setLabel(e.target.value)}
            placeholder="e.g. coach agent"
          />
        </div>
        <Button type="submit" disabled={createMutation.isPending}>
          {createMutation.isPending ? 'Issuing...' : 'Issue Token'}
        </Button>
      </form>

      {isLoading ? (
        <Spinner />
      ) : shown.length > 0 ? (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Label</TableHead>
              <TableHead>Created</TableHead>
              <TableHead>Last used</TableHead>
              <TableHead>Status</TableHead>
              <TableHead className="w-12"><span className="sr-only">Actions</span></TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {shown.map(t => (
              <TableRow key={t.id}>
                <TableCell className="font-medium">{t.label ?? '—'}</TableCell>
                <TableCell className="text-muted-foreground">{new Date(t.created_at).toLocaleDateString()}</TableCell>
                <TableCell className="text-muted-foreground">{t.last_used_at ? new Date(t.last_used_at).toLocaleDateString() : 'Never'}</TableCell>
                <TableCell>
                  {t.revoked_at
                    ? <Badge variant="secondary">Revoked {new Date(t.revoked_at).toLocaleDateString()}</Badge>
                    : <Badge>Active</Badge>}
                </TableCell>
                <TableCell>
                  {!t.revoked_at && (
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      aria-label={`Revoke service token ${t.label ?? t.id}`}
                      className="hover:text-destructive"
                      disabled={revokeMutation.isPending}
                      onClick={async () => {
                        if (await confirm({
                          title: 'Revoke Token',
                          description: `${t.label ? `"${t.label}"` : 'This service token'} will stop working immediately.`,
                          confirmLabel: 'Revoke',
                          variant: 'danger',
                        })) revokeMutation.mutate(t.id)
                      }}
                    >
                      <X aria-hidden="true" />
                    </Button>
                  )}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      ) : (
        <p className="text-sm text-muted-foreground">No active service tokens.</p>
      )}
      {revokedCount > 0 && (
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="mt-2 text-muted-foreground"
          aria-expanded={showRevoked}
          onClick={() => setShowRevoked(v => !v)}
        >
          {showRevoked ? 'Hide revoked' : `Show revoked (${revokedCount})`}
        </Button>
      )}
      {confirmDialog()}
    </section>
  )
}
