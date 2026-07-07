import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'
import { Spinner, EmptyState, QueryError } from '@/components/ui'
import { usePageTitle } from '@/lib/usePageTitle'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Sparkline } from '@/components/ui/sparkline'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { formatDate, formatWeight } from '@/lib/utils'
export function TrainingMaxesList() {
  const { id } = useParams<{ id: string }>()
  const athleteId = Number(id)
  usePageTitle('Training Maxes')
  const { data: athlete } = useQuery({
    queryKey: ['athlete', athleteId],
    queryFn: () => api.getAthlete(athleteId),
    enabled: !isNaN(athleteId),
  })
  const queryClient = useQueryClient()
  const [showForm, setShowForm] = useState(false)
  const [exerciseId, setExerciseId] = useState('')
  const [weight, setWeight] = useState('')
  const [effectiveDate, setEffectiveDate] = useState(new Date().toISOString().slice(0, 10))
  const [notes, setNotes] = useState('')
  const [selectedExercise, setSelectedExercise] = useState<{ id: number; name: string } | null>(null)
  const { data: history } = useQuery({
    queryKey: ['tm-history', athleteId, selectedExercise?.id],
    queryFn: () => api.getTrainingMaxHistory(athleteId, selectedExercise!.id),
    enabled: !!selectedExercise,
  })
  const { data: maxes, isLoading, error, refetch } = useQuery({
    queryKey: ['training-maxes', athleteId],
    queryFn: () => api.listTrainingMaxes(athleteId),
    enabled: !isNaN(athleteId),
  })
  const { data: exercises } = useQuery({
    queryKey: ['exercises'],
    queryFn: () => api.listExercises(),
    enabled: showForm,
  })
  const createMutation = useMutation({
    mutationFn: () => api.createTrainingMax(athleteId, {
      exercise_id: parseInt(exerciseId),
      weight: parseFloat(weight),
      effective_date: effectiveDate,
      notes: notes || undefined,
    }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['training-maxes', athleteId] })
      setWeight('')
      setNotes('')
      setShowForm(false)
    },
  })
  if (isLoading) return <Spinner />
  if (error) return <QueryError error={error} onRetry={refetch} resource="training maxes" />
  return (
    <div>
      <p className="text-sm text-muted-foreground mb-1">
        <Link to={`/athletes/${athleteId}`} className="hover:text-foreground">{athlete?.name ?? 'Athlete'}</Link>
        {' / Training Maxes'}
      </p>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">Training Maxes</h1>
        <Button variant="ghost" onClick={() => setShowForm(!showForm)}
          >
          {showForm ? 'Cancel' : '+ Set TM'}
        </Button>
      </div>
      {/* Add form */}
      {showForm && (
        <form onSubmit={(e) => { e.preventDefault(); createMutation.mutate() }}
          className="rounded-lg border border-border bg-card p-4 mb-6 space-y-3">
          <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
            <div className="col-span-2">
              <Label htmlFor="tm-exercise" >Exercise</Label>
              <Select value={exerciseId || null} onValueChange={(val) => setExerciseId(val ?? "")} required>
                <SelectTrigger className="w-full">
                  <SelectValue placeholder="Select exercise...">
                    {(value: string | null) => {
                      if (!value) return 'Select exercise...'
                      return exercises?.find(ex => String(ex.id) === value)?.name ?? value
                    }}
                  </SelectValue>
                </SelectTrigger>
                <SelectContent>
                  {exercises?.map(ex => (
                    <SelectItem key={ex.id} value={String(ex.id)}>{ex.name}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div>
              <Label htmlFor="tm-weight" >Weight</Label>
              <Input id="tm-weight" type="number" inputMode="decimal" step="0.5" value={weight} onChange={e => setWeight(e.target.value)} required min={1} />
            </div>
            <div>
              <Label htmlFor="tm-date" >Effective Date</Label>
              <Input id="tm-date" type="date" value={effectiveDate} onChange={e => setEffectiveDate(e.target.value)} />
            </div>
          </div>
          <div>
            <Label htmlFor="tm-notes" >Notes</Label>
            <Input id="tm-notes" type="text" value={notes} onChange={e => setNotes(e.target.value)} placeholder="Optional" />
          </div>
          <Button type="submit" disabled={createMutation.isPending || !exerciseId || !weight}
            >
            {createMutation.isPending ? 'Saving...' : 'Set Training Max'}
          </Button>
        </form>
      )}
      {maxes && maxes.length === 0 ? (
        <EmptyState icon="💪" title="No training maxes recorded" description="Set a TM above to drive percentage-based prescriptions." />
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {maxes?.map(tm => {
            const selected = selectedExercise?.id === tm.exercise_id
            const toggle = () => setSelectedExercise(selected ? null : { id: tm.exercise_id, name: tm.exercise_name ?? '' })
            return (
            <div key={tm.id}
              role="button"
              tabIndex={0}
              aria-pressed={selected}
              onClick={toggle}
              onKeyDown={e => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); toggle() } }}
              className={`rounded-lg border bg-card p-4 cursor-pointer transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/50 ${
                selected ? 'border-primary' : 'border-border hover:border-primary/30'
              }`}>
              <p className="text-sm text-muted-foreground">{tm.exercise_name}</p>
              <p className="text-2xl font-bold mt-1">{formatWeight(tm.weight)}</p>
              <p className="text-xs text-muted-foreground mt-1">
                Effective: {formatDate(tm.effective_date)}
              </p>
              {tm.notes && (
                <p className="text-xs text-muted-foreground mt-1">{tm.notes}</p>
              )}
            </div>
            )
          })}
        </div>
      )}
      {/* TM History */}
      {selectedExercise && history && (
        <div className="mt-6">
          <div className="flex items-center justify-between mb-3">
            <h2 className="text-lg font-semibold">{selectedExercise.name} — History</h2>
            {history.length >= 2 && (
              <Sparkline
                data={history.map(tm => tm.weight).slice().reverse()}
                width={160}
                height={40}
                ariaLabel={`${selectedExercise.name} training max progression`}
              />
            )}
          </div>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Date</TableHead>
                <TableHead>Weight</TableHead>
                <TableHead>Notes</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {history.map(tm => (
                <TableRow key={tm.id}>
                  <TableCell>{formatDate(tm.effective_date)}</TableCell>
                  <TableCell className="font-medium">{formatWeight(tm.weight)}</TableCell>
                  <TableCell className="text-muted-foreground">{tm.notes ?? ''}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </div>
  )
}