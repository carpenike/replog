import { useState } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api, ApiError } from '@/api/client'
import { Spinner } from '@/components/ui'
import { Button, buttonVariants } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'

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

  const [username, setUsername] = useState('')
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [isCoach, setIsCoach] = useState(false)
  const [isAdmin, setIsAdmin] = useState(false)
  const [initialized, setInitialized] = useState(false)
  const [error, setError] = useState('')

  if (user && !initialized) {
    setUsername(user.username)
    setName(user.name ?? '')
    setEmail(user.email ?? '')
    setIsCoach(user.is_coach)
    setIsAdmin(user.is_admin)
    setInitialized(true)
  }

  const mutation = useMutation({
    mutationFn: () => api.updateUser(id, {
      username, name: name || undefined, email: email || undefined,
      is_coach: isCoach, is_admin: isAdmin,
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
          <div className="rounded-md bg-destructive/10 border border-destructive/30 p-3 text-sm text-destructive">{error}</div>
        )}

        <div>
          <Label htmlFor="username" >Username *</Label>
          <Input id="username" type="text" value={username} onChange={e => setUsername(e.target.value)} required />
        </div>

        <div>
          <Label htmlFor="name" >Display Name</Label>
          <Input id="name" type="text" value={name} onChange={e => setName(e.target.value)} />
        </div>

        <div>
          <Label htmlFor="email" >Email</Label>
          <Input id="email" type="email" value={email} onChange={e => setEmail(e.target.value)} />
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
          <Button type="submit" disabled={mutation.isPending}
            >
            {mutation.isPending ? 'Saving...' : 'Save Changes'}
          </Button>
          <Link to="/users" className={buttonVariants({ variant: "outline" })}>
            Cancel
          </Link>
        </div>
      </form>
    </div>
  )
}
