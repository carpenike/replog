import { useState } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api, ApiError } from '@/api/client'

export function EditAthlete() {
  const { id } = useParams<{ id: string }>()
  const athleteId = Number(id)
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const { data: athlete, isLoading } = useQuery({
    queryKey: ['athlete', athleteId],
    queryFn: () => api.getAthlete(athleteId),
    enabled: !isNaN(athleteId),
  })

  const [name, setName] = useState('')
  const [tier, setTier] = useState('')
  const [goal, setGoal] = useState('')
  const [notes, setNotes] = useState('')
  const [dateOfBirth, setDateOfBirth] = useState('')
  const [grade, setGrade] = useState('')
  const [gender, setGender] = useState('')
  const [trackBW, setTrackBW] = useState(true)
  const [initialized, setInitialized] = useState(false)
  const [error, setError] = useState('')

  if (athlete && !initialized) {
    setName(athlete.name)
    setTier(athlete.tier ?? '')
    setGoal(athlete.goal ?? '')
    setNotes(athlete.notes ?? '')
    setDateOfBirth(athlete.date_of_birth ?? '')
    setGrade(athlete.grade ?? '')
    setGender(athlete.gender ?? '')
    setTrackBW(athlete.track_body_weight)
    setInitialized(true)
  }

  const mutation = useMutation({
    mutationFn: () => api.updateAthlete(athleteId, {
      name, tier, goal, notes,
      date_of_birth: dateOfBirth || undefined,
      grade: grade || undefined,
      gender: gender || undefined,
      track_body_weight: trackBW,
    }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['athlete', athleteId] })
      queryClient.invalidateQueries({ queryKey: ['athletes'] })
      navigate(`/athletes/${athleteId}`)
    },
    onError: (err) => {
      setError(err instanceof ApiError ? err.message : 'Failed to update athlete')
    },
  })

  const deleteMutation = useMutation({
    mutationFn: () => api.deleteAthlete(athleteId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['athletes'] })
      navigate('/athletes')
    },
  })

  if (isLoading) return <p className="text-muted-foreground">Loading...</p>

  return (
    <div className="max-w-lg">
      <p className="text-sm text-muted-foreground mb-1">
        <Link to={`/athletes/${athleteId}`} className="hover:text-foreground">{athlete?.name ?? 'Athlete'}</Link> / Edit
      </p>
      <h1 className="text-2xl font-bold mb-6">Edit Athlete</h1>

      <form onSubmit={(e) => { e.preventDefault(); mutation.mutate() }} className="space-y-4">
        {error && (
          <div className="rounded-md bg-destructive/10 border border-destructive/30 p-3 text-sm text-destructive">{error}</div>
        )}

        <div>
          <label htmlFor="name" className="block text-sm font-medium mb-1">Name *</label>
          <input id="name" type="text" value={name} onChange={e => setName(e.target.value)} required
            className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm" />
        </div>

        <div>
          <label htmlFor="tier" className="block text-sm font-medium mb-1">Tier</label>
          <select id="tier" value={tier} onChange={e => setTier(e.target.value)}
            className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm">
            <option value="">None</option>
            <option value="foundational">Foundational</option>
            <option value="intermediate">Intermediate</option>
            <option value="sport_performance">Sport Performance</option>
          </select>
        </div>

        <div>
          <label htmlFor="goal" className="block text-sm font-medium mb-1">Goal</label>
          <textarea id="goal" value={goal} onChange={e => setGoal(e.target.value)} rows={2}
            className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm" />
        </div>

        <div>
          <label htmlFor="notes" className="block text-sm font-medium mb-1">Notes</label>
          <textarea id="notes" value={notes} onChange={e => setNotes(e.target.value)} rows={2}
            className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm" />
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div>
            <label htmlFor="dob" className="block text-sm font-medium mb-1">Date of Birth</label>
            <input id="dob" type="date" value={dateOfBirth} onChange={e => setDateOfBirth(e.target.value)}
              className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm" />
          </div>
          <div>
            <label htmlFor="gender" className="block text-sm font-medium mb-1">Gender</label>
            <select id="gender" value={gender} onChange={e => setGender(e.target.value)}
              className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm">
              <option value="">Not specified</option>
              <option value="male">Male</option>
              <option value="female">Female</option>
            </select>
          </div>
        </div>

        <div>
          <label htmlFor="grade" className="block text-sm font-medium mb-1">Grade</label>
          <input id="grade" type="text" value={grade} onChange={e => setGrade(e.target.value)}
            className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm" />
        </div>

        <div className="flex items-center gap-2">
          <input id="trackBW" type="checkbox" checked={trackBW} onChange={e => setTrackBW(e.target.checked)}
            className="rounded border-border" />
          <label htmlFor="trackBW" className="text-sm">Track body weight</label>
        </div>

        <div className="flex items-center justify-between pt-2">
          <div className="flex gap-3">
            <button type="submit" disabled={mutation.isPending}
              className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50">
              {mutation.isPending ? 'Saving...' : 'Save Changes'}
            </button>
            <Link to={`/athletes/${athleteId}`}
              className="rounded-md border border-border px-4 py-2 text-sm hover:bg-accent transition-colors">
              Cancel
            </Link>
          </div>
          <button type="button"
            onClick={() => { if (confirm(`Delete ${athlete?.name}? This cannot be undone.`)) deleteMutation.mutate() }}
            className="text-sm text-destructive hover:text-destructive/80">
            Delete
          </button>
        </div>
      </form>
    </div>
  )
}
