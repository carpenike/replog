import { useState } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api, ApiError } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'

export function NewUser() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const [username, setUsername] = useState('')
  const [name, setName] = useState('')
  const [password, setPassword] = useState('')
  const [email, setEmail] = useState('')
  const [isCoach, setIsCoach] = useState(false)
  const [isAdmin, setIsAdmin] = useState(false)
  const [error, setError] = useState('')

  const mutation = useMutation({
    mutationFn: () => api.createUser({
      username, name: name || undefined, password: password || undefined,
      email: email || undefined, is_coach: isCoach, is_admin: isAdmin,
    }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] })
      navigate('/users')
    },
    onError: (err) => {
      setError(err instanceof ApiError ? err.message : 'Failed to create user')
    },
  })

  return (
    <div className="max-w-lg">
      <p className="text-sm text-muted-foreground mb-1">
        <Link to="/users" className="hover:text-foreground">Users</Link> / New
      </p>
      <h1 className="text-2xl font-bold mb-6">New User</h1>

      <form onSubmit={(e) => { e.preventDefault(); mutation.mutate() }} className="space-y-4">
        {error && (
          <div className="rounded-md bg-destructive/10 border border-destructive/30 p-3 text-sm text-destructive">{error}</div>
        )}

        <div>
          <Label htmlFor="username" >Username *</Label>
          <Input id="username" type="text" value={username} onChange={e => setUsername(e.target.value)} required autoComplete="off" />
        </div>

        <div>
          <Label htmlFor="name" >Display Name</Label>
          <Input id="name" type="text" value={name} onChange={e => setName(e.target.value)} />
        </div>

        <div>
          <Label htmlFor="email" >Email</Label>
          <Input id="email" type="email" value={email} onChange={e => setEmail(e.target.value)} />
        </div>

        <div>
          <Label htmlFor="password" >Password</Label>
          <Input id="password" type="password" value={password} onChange={e => setPassword(e.target.value)} autoComplete="new-password" placeholder="Leave empty for passwordless login" />
        </div>

        <div className="flex gap-4">
          <div className="flex items-center gap-2">
            <input id="isCoach" type="checkbox" checked={isCoach} onChange={e => setIsCoach(e.target.checked)}
              className="rounded border-border" />
            <Label htmlFor="isCoach">Coach</Label>
          </div>
          <div className="flex items-center gap-2">
            <input id="isAdmin" type="checkbox" checked={isAdmin} onChange={e => setIsAdmin(e.target.checked)}
              className="rounded border-border" />
            <Label htmlFor="isAdmin">Admin</Label>
          </div>
        </div>

        <div className="flex gap-3 pt-2">
          <Button type="submit" disabled={mutation.isPending}
            >
            {mutation.isPending ? 'Creating...' : 'Create User'}
          </Button>
          <Link to="/users" className="rounded-md border border-border px-4 py-2 text-sm hover:bg-accent transition-colors">
            Cancel
          </Link>
        </div>
      </form>
    </div>
  )
}
