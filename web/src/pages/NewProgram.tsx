import { useState } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api, ApiError } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'

export function NewProgram() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [numWeeks, setNumWeeks] = useState('4')
  const [numDays, setNumDays] = useState('3')
  const [isLoop, setIsLoop] = useState(false)
  const [error, setError] = useState('')

  const mutation = useMutation({
    mutationFn: () => api.createProgramTemplate({
      name, description: description || undefined,
      num_weeks: parseInt(numWeeks), num_days: parseInt(numDays),
      is_loop: isLoop,
    }),
    onSuccess: (program) => {
      queryClient.invalidateQueries({ queryKey: ['programs'] })
      navigate(`/programs/${program.id}`)
    },
    onError: (err) => {
      setError(err instanceof ApiError ? err.message : 'Failed to create program')
    },
  })

  return (
    <div className="max-w-lg">
      <p className="text-sm text-muted-foreground mb-1">
        <Link to="/programs" className="hover:text-foreground">Programs</Link> / New
      </p>
      <h1 className="text-2xl font-bold mb-6">New Program Template</h1>

      <form onSubmit={(e) => { e.preventDefault(); mutation.mutate() }} className="space-y-4">
        {error && (
          <div className="rounded-md bg-destructive/10 border border-destructive/30 p-3 text-sm text-destructive">{error}</div>
        )}

        <div>
          <Label htmlFor="name" >Name *</Label>
          <Input id="name" type="text" value={name} onChange={e => setName(e.target.value)} required placeholder="e.g. 5/3/1 BBB, GZCL T1/T2" />
        </div>

        <div>
          <Label htmlFor="desc" >Description</Label>
          <Textarea id="desc" value={description} onChange={e => setDescription(e.target.value)} 
            placeholder="Brief description of the program..."
            className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm" />
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div>
            <Label htmlFor="weeks" >Weeks *</Label>
            <Input id="weeks" type="number" min={1} max={52} value={numWeeks} onChange={e => setNumWeeks(e.target.value)} />
          </div>
          <div>
            <Label htmlFor="days" >Days/Week *</Label>
            <Input id="days" type="number" min={1} max={7} value={numDays} onChange={e => setNumDays(e.target.value)} />
          </div>
        </div>

        <div className="flex items-center gap-2">
          <input id="loop" type="checkbox" checked={isLoop} onChange={e => setIsLoop(e.target.checked)}
            className="rounded border-border" />
          <Label htmlFor="loop">Loop (repeat week sequence)</Label>
        </div>

        <div className="flex gap-3 pt-2">
          <Button type="submit" disabled={mutation.isPending}
            >
            {mutation.isPending ? 'Creating...' : 'Create Program'}
          </Button>
          <Link to="/programs" className="rounded-md border border-border px-4 py-2 text-sm hover:bg-accent transition-colors">
            Cancel
          </Link>
        </div>
      </form>
    </div>
  )
}
