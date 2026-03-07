import { useState } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api, ApiError } from '@/api/client'

export function NewExercise() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const [name, setName] = useState('')
  const [tier, setTier] = useState('')
  const [formNotes, setFormNotes] = useState('')
  const [demoUrl, setDemoUrl] = useState('')
  const [restSeconds, setRestSeconds] = useState('')
  const [featured, setFeatured] = useState(false)
  const [error, setError] = useState('')

  const mutation = useMutation({
    mutationFn: () => api.createExercise({
      name, tier,
      form_notes: formNotes || undefined,
      demo_url: demoUrl || undefined,
      rest_seconds: restSeconds ? parseInt(restSeconds) : undefined,
      featured,
    }),
    onSuccess: (exercise) => {
      queryClient.invalidateQueries({ queryKey: ['exercises'] })
      navigate(`/exercises/${exercise.id}`)
    },
    onError: (err) => {
      setError(err instanceof ApiError ? err.message : 'Failed to create exercise')
    },
  })

  return (
    <div className="max-w-lg">
      <p className="text-sm text-muted-foreground mb-1">
        <Link to="/exercises" className="hover:text-foreground">Exercises</Link> / New
      </p>
      <h1 className="text-2xl font-bold mb-6">New Exercise</h1>

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
          <label htmlFor="formNotes" className="block text-sm font-medium mb-1">Form Notes</label>
          <textarea id="formNotes" value={formNotes} onChange={e => setFormNotes(e.target.value)} rows={3}
            placeholder="Coaching cues, technique notes..."
            className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm" />
        </div>

        <div>
          <label htmlFor="demoUrl" className="block text-sm font-medium mb-1">Demo URL</label>
          <input id="demoUrl" type="url" value={demoUrl} onChange={e => setDemoUrl(e.target.value)}
            placeholder="https://..."
            className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm" />
        </div>

        <div>
          <label htmlFor="rest" className="block text-sm font-medium mb-1">Rest Timer (seconds)</label>
          <input id="rest" type="number" value={restSeconds} onChange={e => setRestSeconds(e.target.value)}
            min={0} placeholder="e.g. 120"
            className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm" />
        </div>

        <div className="flex items-center gap-2">
          <input id="featured" type="checkbox" checked={featured} onChange={e => setFeatured(e.target.checked)}
            className="rounded border-border" />
          <label htmlFor="featured" className="text-sm">Featured lift</label>
        </div>

        <div className="flex gap-3 pt-2">
          <button type="submit" disabled={mutation.isPending}
            className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50">
            {mutation.isPending ? 'Creating...' : 'Create Exercise'}
          </button>
          <Link to="/exercises" className="rounded-md border border-border px-4 py-2 text-sm hover:bg-accent transition-colors">
            Cancel
          </Link>
        </div>
      </form>
    </div>
  )
}
