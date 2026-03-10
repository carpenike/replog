import { useState } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, ApiError } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'

export function NewWorkout() {
  const { id } = useParams<{ id: string }>()
  const athleteId = Number(id)
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const [date, setDate] = useState(new Date().toISOString().slice(0, 10))
  const [notes, setNotes] = useState('')
  const [error, setError] = useState('')

  const { data: athlete } = useQuery({
    queryKey: ['athlete', athleteId],
    queryFn: () => api.getAthlete(athleteId),
    enabled: !isNaN(athleteId),
  })

  const createMutation = useMutation({
    mutationFn: () => api.createWorkout(athleteId, date, notes),
    onSuccess: (workout) => {
      queryClient.invalidateQueries({ queryKey: ['workouts', athleteId] })
      navigate(`/athletes/${athleteId}/workouts/${workout.id}`)
    },
    onError: (err) => {
      if (err instanceof ApiError) setError(err.message)
      else setError('Failed to create workout')
    },
  })

  return (
    <div className="max-w-lg">
      <p className="text-sm text-muted-foreground mb-1">
        <Link to={`/athletes/${athleteId}`} className="hover:text-foreground">
          {athlete?.name ?? 'Athlete'}
        </Link>
        {' / '}
        <Link to={`/athletes/${athleteId}/workouts`} className="hover:text-foreground">Workouts</Link>
        {' / New'}
      </p>
      <h1 className="text-2xl font-bold mb-6">New Workout</h1>

      <form
        onSubmit={(e) => { e.preventDefault(); createMutation.mutate() }}
        className="space-y-4"
      >
        {error && (
          <div className="rounded-md bg-destructive/10 border border-destructive/30 p-3 text-sm text-destructive">
            {error}
          </div>
        )}

        <div>
          <Label htmlFor="date" >Date</Label>
          <Input id="date" type="date" value={date} onChange={e => setDate(e.target.value)} required />
        </div>

        <div>
          <Label htmlFor="notes" >Notes</Label>
          <Textarea id="notes" value={notes} onChange={e => setNotes(e.target.value)}
            rows={3} placeholder="Optional session notes..."
            className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm" />
        </div>

        <div className="flex gap-3">
          <Button type="submit" disabled={createMutation.isPending}
            >
            {createMutation.isPending ? 'Creating...' : 'Create Workout'}
          </Button>
          <Link to={`/athletes/${athleteId}/workouts`}
            className="rounded-md border border-border px-4 py-2 text-sm hover:bg-accent transition-colors">
            Cancel
          </Link>
        </div>
      </form>
    </div>
  )
}
