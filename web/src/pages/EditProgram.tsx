import { useState } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api, ApiError } from '@/api/client'
import { Field } from '@/components/Field'
import { PageHeader } from '@/components/PageHeader'
import { QueryState } from '@/components/QueryState'
import { Button, buttonVariants } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Alert } from '@/components/ui/alert'

export function EditProgram() {
  const { id } = useParams<{ id: string }>()
  const programId = Number(id)
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const query = useQuery({
    queryKey: ['program', programId],
    queryFn: () => api.getProgramTemplate(programId),
    enabled: !isNaN(programId),
  })
  const data = query.data

  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [numWeeks, setNumWeeks] = useState('')
  const [numDays, setNumDays] = useState('')
  const [isLoop, setIsLoop] = useState(false)
  const [initialized, setInitialized] = useState(false)
  const [error, setError] = useState('')

  if (data && !initialized) {
    setName(data.program.name)
    setDescription(data.program.description ?? '')
    setNumWeeks(data.program.num_weeks.toString())
    setNumDays(data.program.num_days.toString())
    setIsLoop(data.program.is_loop)
    setInitialized(true)
  }

  const mutation = useMutation({
    mutationFn: () => api.updateProgramTemplate(programId, {
      name, description: description || undefined,
      num_weeks: parseInt(numWeeks), num_days: parseInt(numDays),
      is_loop: isLoop,
    }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['program', programId] })
      queryClient.invalidateQueries({ queryKey: ['programs'] })
      navigate(`/programs/${programId}`)
    },
    onError: (err) => {
      setError(err instanceof ApiError ? err.message : 'Failed to update program')
    },
  })

  return (
    <div className="max-w-lg">
      <PageHeader title="Edit Program" back={{ to: `/programs/${programId}`, label: data?.program.name ?? 'Program' }} />

      <QueryState query={query} resource="program" skeleton="form">
      {({ program }) => program.locked ? (
        <Alert>
          This template was imported and is locked (v{program.import_version ?? program.version}). To change it, re-import a new version.
        </Alert>
      ) : (
      <form onSubmit={(e) => { e.preventDefault(); mutation.mutate() }} className="space-y-4">
        {error && (
          <Alert variant="destructive">{error}</Alert>
        )}

        <Field label="Name" id="name" required>
          <Input id="name" type="text" value={name} onChange={e => setName(e.target.value)} required />
        </Field>

        <Field label="Description" id="desc">
          <Textarea id="desc" value={description} onChange={e => setDescription(e.target.value)} />
        </Field>

        <div className="grid grid-cols-2 gap-3">
          <Field label="Weeks" id="weeks" required>
            <Input id="weeks" type="number" inputMode="numeric" min={1} value={numWeeks} onChange={e => setNumWeeks(e.target.value)} required />
          </Field>
          <Field label="Days/Week" id="days" required>
            <Input id="days" type="number" inputMode="numeric" min={1} max={7} value={numDays} onChange={e => setNumDays(e.target.value)} required />
          </Field>
        </div>

        <div className="flex items-center gap-2">
          <Checkbox id="loop" checked={isLoop} onCheckedChange={(checked) => setIsLoop(checked)} />
          <Label htmlFor="loop">Loop</Label>
        </div>

        <div className="flex gap-3 pt-2">
          <Button type="submit" disabled={mutation.isPending}
            >
            {mutation.isPending ? 'Saving...' : 'Save Changes'}
          </Button>
          <Link to={`/programs/${programId}`}
            className={buttonVariants({ variant: "outline" })}>
            Cancel
          </Link>
        </div>
      </form>
      )}
      </QueryState>
    </div>
  )
}
