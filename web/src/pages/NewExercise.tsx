import { useState } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api, ApiError } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'

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
          <Label htmlFor="formNotes" >Form Notes</Label>
          <Textarea id="formNotes" value={formNotes} onChange={e => setFormNotes(e.target.value)} 
            placeholder="Coaching cues, technique notes..."
            className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm" />
        </div>

        <div>
          <Label htmlFor="demoUrl" >Demo URL</Label>
          <Input id="demoUrl" type="url" value={demoUrl} onChange={e => setDemoUrl(e.target.value)} placeholder="https://..." />
        </div>

        <div>
          <Label htmlFor="rest" >Rest Timer (seconds)</Label>
          <Input id="rest" type="number" value={restSeconds} onChange={e => setRestSeconds(e.target.value)} min={0} placeholder="e.g. 120" />
        </div>

        <div className="flex items-center gap-2">
          <input id="featured" type="checkbox" checked={featured} onChange={e => setFeatured(e.target.checked)}
            className="rounded border-border" />
          <Label htmlFor="featured">Featured lift</Label>
        </div>

        <div className="flex gap-3 pt-2">
          <Button type="submit" disabled={mutation.isPending}
            >
            {mutation.isPending ? 'Creating...' : 'Create Exercise'}
          </Button>
          <Link to="/exercises" className="rounded-md border border-border px-4 py-2 text-sm hover:bg-accent transition-colors">
            Cancel
          </Link>
        </div>
      </form>
    </div>
  )
}
