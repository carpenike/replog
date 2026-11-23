import { useState } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api, ApiError } from '@/api/client'
import { Button, buttonVariants } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import { Field } from '@/components/Field'
import { PageHeader } from '@/components/PageHeader'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Alert } from '@/components/ui/alert'

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
      <PageHeader title="New Exercise" back={{ to: '/exercises', label: 'Exercises' }} />

      <form onSubmit={(e) => { e.preventDefault(); mutation.mutate() }} className="space-y-4">
        {error && (
          <Alert variant="destructive">{error}</Alert>
        )}

        <Field label="Name" id="name" required>
          <Input id="name" type="text" value={name} onChange={e => setName(e.target.value)} required />
        </Field>

        <Field label="Tier" id="tier">
          <Select value={tier} onValueChange={(val) => setTier(val ?? "")}>
            <SelectTrigger id="tier" className="w-full">
              <SelectValue placeholder="None" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="">None</SelectItem>
              <SelectItem value="foundational">Foundational</SelectItem>
              <SelectItem value="intermediate">Intermediate</SelectItem>
              <SelectItem value="sport_performance">Sport Performance</SelectItem>
            </SelectContent>
          </Select>
        </Field>

        <Field label="Form Notes" id="formNotes">
          <Textarea id="formNotes" value={formNotes} onChange={e => setFormNotes(e.target.value)}
            placeholder="Coaching cues, technique notes..."
          />
        </Field>

        <Field label="Demo URL" id="demoUrl">
          <Input id="demoUrl" type="url" value={demoUrl} onChange={e => setDemoUrl(e.target.value)} placeholder="https://..." />
        </Field>

        <Field label="Rest Timer (seconds)" id="rest">
          <Input id="rest" type="number" inputMode="numeric" value={restSeconds} onChange={e => setRestSeconds(e.target.value)} min={0} placeholder="e.g. 120" />
        </Field>

        <div className="flex items-center gap-2">
          <Checkbox id="featured" checked={featured} onCheckedChange={(checked) => setFeatured(checked)} />
          <Label htmlFor="featured">Featured lift</Label>
        </div>

        <div className="flex gap-3 pt-2">
          <Button type="submit" disabled={mutation.isPending}
            >
            {mutation.isPending ? 'Creating...' : 'Create Exercise'}
          </Button>
          <Link to="/exercises" className={buttonVariants({ variant: "outline" })}>
            Cancel
          </Link>
        </div>
      </form>
    </div>
  )
}
