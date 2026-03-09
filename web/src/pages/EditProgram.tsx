import { useState } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api, ApiError } from '@/api/client'
import { Spinner } from '@/components/ui'

export function EditProgram() {
  const { id } = useParams<{ id: string }>()
  const programId = Number(id)
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const { data, isLoading } = useQuery({
    queryKey: ['program', programId],
    queryFn: () => api.getProgramTemplate(programId),
    enabled: !isNaN(programId),
  })

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

  if (isLoading) return <Spinner />

  return (
    <div className="max-w-lg">
      <p className="text-sm text-muted-foreground mb-1">
        <Link to={`/programs/${programId}`} className="hover:text-foreground">{data?.program.name ?? 'Program'}</Link> / Edit
      </p>
      <h1 className="text-2xl font-bold mb-6">Edit Program</h1>

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
          <label htmlFor="desc" className="block text-sm font-medium mb-1">Description</label>
          <textarea id="desc" value={description} onChange={e => setDescription(e.target.value)} rows={2}
            className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm" />
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div>
            <label htmlFor="weeks" className="block text-sm font-medium mb-1">Weeks</label>
            <input id="weeks" type="number" min={1} value={numWeeks} onChange={e => setNumWeeks(e.target.value)}
              className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm" />
          </div>
          <div>
            <label htmlFor="days" className="block text-sm font-medium mb-1">Days/Week</label>
            <input id="days" type="number" min={1} max={7} value={numDays} onChange={e => setNumDays(e.target.value)}
              className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm" />
          </div>
        </div>

        <div className="flex items-center gap-2">
          <input id="loop" type="checkbox" checked={isLoop} onChange={e => setIsLoop(e.target.checked)}
            className="rounded border-border" />
          <label htmlFor="loop" className="text-sm">Loop</label>
        </div>

        <div className="flex gap-3 pt-2">
          <button type="submit" disabled={mutation.isPending}
            className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50">
            {mutation.isPending ? 'Saving...' : 'Save Changes'}
          </button>
          <Link to={`/programs/${programId}`}
            className="rounded-md border border-border px-4 py-2 text-sm hover:bg-accent transition-colors">
            Cancel
          </Link>
        </div>
      </form>
    </div>
  )
}
