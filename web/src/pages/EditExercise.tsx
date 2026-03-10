import { useState } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api, ApiError } from '@/api/client'
import { Spinner } from '@/components/ui'
import { useConfirm } from '@/lib/useConfirm'
import { Button, buttonVariants } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

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
          <Label>Tier</Label>
          <Select value={tier} onValueChange={(val) => setTier(val ?? "")}>
            <SelectTrigger className="w-full">
              <SelectValue placeholder="None" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="">None</SelectItem>
              <SelectItem value="foundational">Foundational</SelectItem>
              <SelectItem value="intermediate">Intermediate</SelectItem>
              <SelectItem value="sport_performance">Sport Performance</SelectItem>
            </SelectContent>
          </Select>
        </div>

        <div>
          <Label htmlFor="formNotes" >Form Notes</Label>
          <Textarea id="formNotes" value={formNotes} onChange={e => setFormNotes(e.target.value)} 
            />
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
          <Checkbox id="featured" checked={featured} onCheckedChange={(checked) => setFeatured(checked)} />
          <Label htmlFor="featured">Featured lift</Label>
        </div>

        <div className="flex items-center justify-between pt-2">
          <div className="flex gap-3">
            <Button type="submit" disabled={mutation.isPending}
              >
              {mutation.isPending ? 'Saving...' : 'Save Changes'}
            </Button>
            <Link to={`/exercises/${exerciseId}`}
              className={buttonVariants({ variant: "outline" })}>
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
