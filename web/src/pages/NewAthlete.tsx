import { useState } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api, ApiError } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'

export function NewAthlete() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const [name, setName] = useState('')
  const [tier, setTier] = useState('')
  const [goal, setGoal] = useState('')
  const [notes, setNotes] = useState('')
  const [dateOfBirth, setDateOfBirth] = useState('')
  const [grade, setGrade] = useState('')
  const [gender, setGender] = useState('')
  const [trackBW, setTrackBW] = useState(true)
  const [error, setError] = useState('')

  const mutation = useMutation({
    mutationFn: () => api.createAthlete({
      name, tier, goal, notes,
      date_of_birth: dateOfBirth || undefined,
      grade: grade || undefined,
      gender: gender || undefined,
      track_body_weight: trackBW,
    }),
    onSuccess: (athlete) => {
      queryClient.invalidateQueries({ queryKey: ['athletes'] })
      navigate(`/athletes/${athlete.id}`)
    },
    onError: (err) => {
      setError(err instanceof ApiError ? err.message : 'Failed to create athlete')
    },
  })

  return (
    <div className="max-w-lg">
      <p className="text-sm text-muted-foreground mb-1">
        <Link to="/athletes" className="hover:text-foreground">Athletes</Link> / New
      </p>
      <h1 className="text-2xl font-bold mb-6">New Athlete</h1>

      <form onSubmit={(e) => { e.preventDefault(); mutation.mutate() }} className="space-y-4">
        {error && (
          <div className="rounded-md bg-destructive/10 border border-destructive/30 p-3 text-sm text-destructive">{error}</div>
        )}

        <div>
          <Label htmlFor="name" >Name *</Label>
          <Input id="name" type="text" value={name} onChange={e => setName(e.target.value)} required />
        </div>

        <div>
          <Label htmlFor="tier" >Tier</Label>
          <select id="tier" value={tier} onChange={e => setTier(e.target.value)}
            className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm">
            <option value="">None</option>
            <option value="foundational">Foundational</option>
            <option value="intermediate">Intermediate</option>
            <option value="sport_performance">Sport Performance</option>
          </select>
        </div>

        <div>
          <Label htmlFor="goal" >Goal</Label>
          <Textarea id="goal" value={goal} onChange={e => setGoal(e.target.value)} 
            className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm" />
        </div>

        <div>
          <Label htmlFor="notes" >Notes</Label>
          <Textarea id="notes" value={notes} onChange={e => setNotes(e.target.value)} 
            className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm" />
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div>
            <Label htmlFor="dob" >Date of Birth</Label>
            <Input id="dob" type="date" value={dateOfBirth} onChange={e => setDateOfBirth(e.target.value)} />
          </div>
          <div>
            <Label htmlFor="gender" >Gender</Label>
            <select id="gender" value={gender} onChange={e => setGender(e.target.value)}
              className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm">
              <option value="">Not specified</option>
              <option value="male">Male</option>
              <option value="female">Female</option>
            </select>
          </div>
        </div>

        <div>
          <Label htmlFor="grade" >Grade</Label>
          <Input id="grade" type="text" value={grade} onChange={e => setGrade(e.target.value)} placeholder="e.g. 8th" />
        </div>

        <div className="flex items-center gap-2">
          <input id="trackBW" type="checkbox" checked={trackBW} onChange={e => setTrackBW(e.target.checked)}
            className="rounded border-border" />
          <Label htmlFor="trackBW">Track body weight</Label>
        </div>

        <div className="flex gap-3 pt-2">
          <Button type="submit" disabled={mutation.isPending}
            >
            {mutation.isPending ? 'Creating...' : 'Create Athlete'}
          </Button>
          <Link to="/athletes" className="rounded-md border border-border px-4 py-2 text-sm hover:bg-accent transition-colors">
            Cancel
          </Link>
        </div>
      </form>
    </div>
  )
}
