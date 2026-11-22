import { useState } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, ApiError } from '@/api/client'
import { Button, buttonVariants } from '@/components/ui/button'
import { Field } from '@/components/Field'
import { PageHeader } from '@/components/PageHeader'
import { localDateISO } from '@/lib/utils'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Alert } from '@/components/ui/alert'

export function NewWorkout() {
  const { id } = useParams<{ id: string }>()
  const athleteId = Number(id)
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const [date, setDate] = useState(localDateISO)
  const [notes, setNotes] = useState('')
  const [error, setError] = useState('')

  const { data: athlete } = useQuery({
    queryKey: ['athlete', athleteId],
    queryFn: () => api.getAthlete(athleteId),
    enabled: !isNaN(athleteId),
  })

  const createMutation = useMutation({
    mutationFn: () => api.createWorkout(athleteId, date, notes),
    meta: { skipGlobalError: true },
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
      <PageHeader
        title="New Workout"
        back={{ to: `/athletes/${athleteId}/workouts`, label: athlete ? `${athlete.name}'s workouts` : 'Workouts' }}
      />

      <form
        onSubmit={(e) => { e.preventDefault(); createMutation.mutate() }}
        className="space-y-4"
      >
        {error && (
          <Alert variant="destructive">
            {error}
          </Alert>
        )}

        <Field label="Date" id="date" required>
          <Input id="date" type="date" enterKeyHint="done" value={date} onChange={e => setDate(e.target.value)} required />
        </Field>

        <Field label="Notes" id="notes">
          <Textarea id="notes" value={notes} onChange={e => setNotes(e.target.value)}
            rows={3} placeholder="Optional session notes..."
          />
        </Field>

        <div className="flex gap-3">
          <Button type="submit" size="touch" disabled={createMutation.isPending}
            >
            {createMutation.isPending ? 'Creating...' : 'Create Workout'}
          </Button>
          <Link to={`/athletes/${athleteId}/workouts`}
            className={buttonVariants({ variant: "outline", size: "touch" })}>
            Cancel
          </Link>
        </div>
      </form>
    </div>
  )
}
