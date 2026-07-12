import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Search } from 'lucide-react'
import { api } from '@/api/client'
import { cn } from '@/lib/utils'

export interface PickedExercise {
  id: number
  name: string
}

/**
 * Grouped, searchable exercise picker (Prescribed today / Assigned / All).
 * Owns its queries — the shared query keys mean the cache is reused across
 * pages that also fetch exercises or the prescription.
 */
export function ExercisePicker({
  athleteId,
  value,
  onSelect,
  triggerLabelId,
}: {
  athleteId: number
  value: number | null
  onSelect: (exercise: PickedExercise) => void
  /** id of an external <Label> naming the trigger button. */
  triggerLabelId?: string
}) {
  const [open, setOpen] = useState(false)
  const [search, setSearch] = useState('')

  const { data: exercises } = useQuery({
    queryKey: ['exercises'],
    queryFn: () => api.listExercises(),
  })
  const { data: prescription } = useQuery({
    queryKey: ['prescription', athleteId],
    queryFn: () => api.getPrescription(athleteId),
    enabled: !isNaN(athleteId),
    retry: false,
  })
  const { data: assignments } = useQuery({
    queryKey: ['assignments', athleteId],
    queryFn: () => api.listAssignments(athleteId),
    enabled: !isNaN(athleteId),
    retry: false,
  })

  const pickerGroups = useMemo(() => {
    const q = search.trim().toLowerCase()
    const match = (name: string) => !q || name.toLowerCase().includes(q)
    const prescribed = (prescription?.lines ?? [])
      .filter(l => match(l.exercise_name))
      .map(l => ({ id: l.exercise_id, name: l.exercise_name }))
    const prescribedIds = new Set(prescribed.map(e => e.id))
    const assigned = (assignments ?? [])
      .filter(a => a.active && a.exercise_name && match(a.exercise_name) && !prescribedIds.has(a.exercise_id))
      .map(a => ({ id: a.exercise_id, name: a.exercise_name as string }))
    const assignedIds = new Set(assigned.map(e => e.id))
    const all = (exercises ?? [])
      .filter(e => match(e.name) && !prescribedIds.has(e.id) && !assignedIds.has(e.id))
      .map(e => ({ id: e.id, name: e.name }))
    return [
      { key: 'prescribed', label: 'Prescribed today', items: prescribed },
      { key: 'assigned', label: 'Assigned', items: assigned },
      { key: 'all', label: 'All exercises', items: all },
    ].filter(g => g.items.length > 0)
  }, [search, prescription, assignments, exercises])

  function pick(item: PickedExercise) {
    setOpen(false)
    setSearch('')
    onSelect(item)
  }

  const selectedName = value != null
    ? exercises?.find(e => e.id === value)?.name
      ?? prescription?.lines.find(l => l.exercise_id === value)?.exercise_name
    : undefined

  return (
    <div>
      <button
        type="button"
        aria-labelledby={triggerLabelId}
        aria-expanded={open}
        onClick={() => setOpen(o => !o)}
        className="mt-1 flex h-11 w-full items-center justify-between rounded-lg border border-input bg-transparent px-3 text-left text-base transition-colors hover:bg-muted/40 focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 outline-none dark:bg-input/30"
      >
        <span className={cn(!selectedName && 'text-muted-foreground')}>
          {selectedName ?? 'Select exercise...'}
        </span>
      </button>
      {open && (
        <div className="mt-1 rounded-lg border border-border bg-popover shadow-md">
          <div className="flex items-center gap-2 border-b border-border px-3">
            <Search className="size-4 text-muted-foreground" aria-hidden="true" />
            <input
              autoFocus
              value={search}
              onChange={e => setSearch(e.target.value)}
              placeholder="Search exercises..."
              aria-label="Search exercises"
              className="h-11 w-full bg-transparent text-base outline-none placeholder:text-muted-foreground"
            />
          </div>
          <div className="max-h-64 overflow-y-auto p-1">
            {pickerGroups.length === 0 ? (
              <p className="px-3 py-4 text-sm text-muted-foreground">No matches</p>
            ) : pickerGroups.map(g => (
              <div key={g.key}>
                <p className="px-2 pt-2 pb-1 text-xs font-medium text-muted-foreground">{g.label}</p>
                {g.items.map(item => (
                  <button
                    key={`${g.key}-${item.id}`}
                    type="button"
                    onClick={() => pick(item)}
                    className={cn(
                      'flex h-11 w-full items-center rounded-md px-2 text-left text-sm hover:bg-accent hover:text-accent-foreground',
                      item.id === value && 'bg-accent text-accent-foreground',
                    )}
                  >
                    {item.name}
                  </button>
                ))}
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
