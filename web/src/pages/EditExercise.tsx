import { useState } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api, ApiError } from '@/api/client'
import { Spinner } from '@/components/ui'
import { useConfirm } from '@/lib/useConfirm'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'

export function EditExercise() {
  const { id } = useParams<{ id: string }>()
  const exerciseId = Number(id)
  const navigate = useNavigate()
  const { confirm, dialog: confirmDialog } = useConfirm()
  const queryClient = useQueryClient()

  const { data: exercise, isLoading } = useQuery({
    queryKey: ['exercise', exerciseId],
    queryFn: () => api.getExercise(exerciseId),
    enabled: !isNaN(exerciseId),
  })

  const [name, setName] = useState('')
  const [tier, setTier] = useState('')
  const [formNotes, setFormNotes] = useState('')
  const [demoUrl, setDemoUrl] = useState('')
  const [restSeconds, setRestSeconds] = useState('')
  const [featured, setFeatured] = useState(false)
  const [initialized, setInitialized] = useState(false)
  const [error, setError] = useState('')

  if (exercise && !initialized) {
    setName(exercise.name)
    setTier(exercise.tier ?? '')
    setFormNotes(exercise.form_notes ?? '')
    setDemoUrl(exercise.demo_url ?? '')
    setRestSeconds(exercise.rest_seconds?.toString() ?? '')
    setFeatured(exercise.featured)
    setInitialized(true)
  }

  const mutation = useMutation({
    mutationFn: () => api.updateExercise(exerciseId, {
      name, tier,
      form_notes: formNotes || undefined,
      demo_url: demoUrl || undefined,
      rest_seconds: restSeconds ? parseInt(restSeconds) : undefined,
      featured,
    }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['exercise', exerciseId] })
      queryClient.invalidateQueries({ queryKey: ['exercises'] })
      navigate(`/exercises/${exerciseId}`)
    },
    onError: (err) => {
      setError(err instanceof ApiError ? err.message : 'Failed to update exercise')
    },
  })

  const deleteMutation = useMutation({
    mutationFn: () => api.deleteExercise(exerciseId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['exercises'] })
      navigate('/exercises')
    },
  })

  if (isLoading) return <Spinner />

  return (
    <div className="max-w-lg">
      <p className="text-sm text-muted-foreground mb-1">
        <Link to={`/exercises/${exerciseId}`} className="hover:text-foreground">{exercise?.name ?? 'Exercise'}</Link> / Edit
      </p>
      <h1 className="text-2xl font-bold mb-6">Edit Exercise</h1>

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
            className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm" />
        </div>

        <div>
          <Label htmlFor="demoUrl" >Demo URL</Label>
          <Input id="demoUrl" type="url" value={demoUrl} onChange={e => setDemoUrl(e.target.value)} />
        </div>

        <div>
          <Label htmlFor="rest" >Rest Timer (seconds)</Label>
          <Input id="rest" type="number" value={restSeconds} onChange={e => setRestSeconds(e.target.value)} min={0} />
        </div>

        <div className="flex items-center gap-2">
          <input id="featured" type="checkbox" checked={featured} onChange={e => setFeatured(e.target.checked)}
            className="rounded border-border" />
          <Label htmlFor="featured">Featured lift</Label>
        </div>

        <div className="flex items-center justify-between pt-2">
          <div className="flex gap-3">
            <Button type="submit" disabled={mutation.isPending}
              >
              {mutation.isPending ? 'Saving...' : 'Save Changes'}
            </Button>
            <Link to={`/exercises/${exerciseId}`}
              className="rounded-md border border-border px-4 py-2 text-sm hover:bg-accent transition-colors">
              Cancel
            </Link>
          </div>
          <Button variant="ghost" type="button" onClick={async () => { if (await confirm({ title: 'Delete Exercise', description: `Delete ${exercise?.name}? This will fail if workouts reference it.`, confirmLabel: 'Delete', variant: 'danger' })) deleteMutation.mutate() }}
            className="text-sm text-destructive hover:text-destructive/80">
            Delete
          </Button>
        </div>
      </form>
      {confirmDialog()}
    </div>
  )
}
