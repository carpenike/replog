import { useState } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api, ApiError } from '@/api/client'
import { Button, buttonVariants } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Alert } from '@/components/ui/alert'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

export function NewUser() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const { data: athletes } = useQuery({
    queryKey: ['athletes'],
    queryFn: () => api.listAthletes(),
  })

  const [username, setUsername] = useState('')
  const [name, setName] = useState('')
  const [password, setPassword] = useState('')
  const [email, setEmail] = useState('')
  const [isCoach, setIsCoach] = useState(false)
  const [isAdmin, setIsAdmin] = useState(false)
  const [athleteId, setAthleteId] = useState('')
  const [createAthlete, setCreateAthlete] = useState(false)
  const [error, setError] = useState('')

  const mutation = useMutation({
    mutationFn: async () => {
      let linkedAthleteId: number | undefined

      // Create a new athlete first if requested.
      if (createAthlete && (name || username)) {
        const athlete = await api.createAthlete({
          name: name || username,
          track_body_weight: true,
        })
        linkedAthleteId = athlete.id
      } else if (athleteId) {
        linkedAthleteId = Number(athleteId)
      }

      return api.createUser({
        username,
        name: name || undefined,
        password: password || undefined,
        email: email || undefined,
        is_coach: isCoach,
        is_admin: isAdmin,
        athlete_id: linkedAthleteId,
      })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] })
      queryClient.invalidateQueries({ queryKey: ['athletes'] })
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
          <Alert variant="destructive">{error}</Alert>
        )}

        <div>
          <Label htmlFor="username">Username *</Label>
          <Input id="username" type="text" value={username} onChange={e => setUsername(e.target.value)} required autoComplete="off" />
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
          <Label htmlFor="password">Password</Label>
          <Input id="password" type="password" value={password} onChange={e => setPassword(e.target.value)} autoComplete="new-password" placeholder="Leave empty for passwordless login" />
        </div>

        <div className="flex gap-4">
          <div className="flex items-center gap-2">
            <Checkbox id="isCoach" checked={isCoach} onCheckedChange={(checked) => setIsCoach(checked)} />
            <Label htmlFor="isCoach">Coach</Label>
          </div>
          <div className="flex items-center gap-2">
            <Checkbox id="isAdmin" checked={isAdmin} onCheckedChange={(checked) => setIsAdmin(checked)} />
            <Label htmlFor="isAdmin">Admin</Label>
          </div>
        </div>

        <div>
          <Label>Linked Athlete</Label>
          <div className="space-y-2">
            <div className="flex items-center gap-2">
              <Checkbox id="createAthlete" checked={createAthlete} onCheckedChange={(checked) => { setCreateAthlete(checked); if (checked) setAthleteId('') }} />
              <Label htmlFor="createAthlete">Create new athlete with this name</Label>
            </div>
            {!createAthlete && (
              <Select value={athleteId || null} onValueChange={(val) => setAthleteId(val ?? '')}>
                <SelectTrigger className="w-full">
                  <SelectValue placeholder="None — link later">
                    {(value: string | null) => {
                      if (!value) return 'None — link later'
                      return athletes?.find(a => String(a.id) === value)?.name ?? value
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
            )}
          </div>
        </div>

        <div className="flex gap-3 pt-2">
          <Button type="submit" disabled={mutation.isPending}>
            {mutation.isPending ? 'Creating...' : 'Create User'}
          </Button>
          <Link to="/users" className={buttonVariants({ variant: "outline" })}>
            Cancel
          </Link>
        </div>
      </form>
    </div>
  )
}
