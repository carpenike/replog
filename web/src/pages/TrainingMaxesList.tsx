import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'
import { Plus, X } from 'lucide-react'
import { PageHeader } from '@/components/PageHeader'
import { QueryState } from '@/components/QueryState'
import { useFriendlyDate } from '@/lib/dates'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Sparkline } from '@/components/ui/sparkline'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { formatWeight, localDateISO } from '@/lib/utils'
export function TrainingMaxesList() {
  const { id } = useParams<{ id: string }>()
  const athleteId = Number(id)
  const friendlyDate = useFriendlyDate()
  const { data: athlete } = useQuery({
    queryKey: ['athlete', athleteId],
    queryFn: () => api.getAthlete(athleteId),
    enabled: !isNaN(athleteId),
  })
  const queryClient = useQueryClient()
  const [showForm, setShowForm] = useState(false)
  const [exerciseId, setExerciseId] = useState('')
  const [weight, setWeight] = useState('')
  const [effectiveDate, setEffectiveDate] = useState(localDateISO)
  const [notes, setNotes] = useState('')
  const [selectedExercise, setSelectedExercise] = useState<{ id: number; name: string } | null>(null)
  const { data: history } = useQuery({
    queryKey: ['tm-history', athleteId, selectedExercise?.id],
    queryFn: () => api.getTrainingMaxHistory(athleteId, selectedExercise!.id),
    enabled: !!selectedExercise,
  })
  const query = useQuery({
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
  return (
    <div>
      <PageHeader
        title="Training Maxes"
        back={{ to: `/athletes/${athleteId}`, label: athlete?.name ?? 'Athlete' }}
        actions={
          <Button variant={showForm ? 'ghost' : 'default'} onClick={() => setShowForm(!showForm)} aria-expanded={showForm}>
            {showForm ? <><X aria-hidden="true" /> Cancel</> : <><Plus aria-hidden="true" /> Set TM</>}
          </Button>
        }
      />
      {/* Add form */}
      {showForm && (
        <form onSubmit={(e) => { e.preventDefault(); createMutation.mutate() }}
          className="rounded-lg border border-border bg-card p-4 mb-6 space-y-3">
          <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
            <div className="col-span-2">
              <Label htmlFor="tm-exercise">Exercise <span aria-hidden="true" className="text-destructive">*</span></Label>
              <Select value={exerciseId || null} onValueChange={(val) => setExerciseId(val ?? "")} required>
                <SelectTrigger id="tm-exercise" className="w-full">
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
              <Label htmlFor="tm-weight">Weight <span aria-hidden="true" className="text-destructive">*</span></Label>
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
      <QueryState
        query={query}
        resource="training maxes"
        skeleton="cards"
        empty={{ icon: '💪', title: 'No training maxes recorded', description: 'Set a TM to drive percentage-based prescriptions.', action: showForm ? undefined : 'Set TM', onAction: () => setShowForm(true) }}
      >
        {maxes => (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {maxes.map(tm => {
            const selected = selectedExercise?.id === tm.exercise_id
            const toggle = () => setSelectedExercise(selected ? null : { id: tm.exercise_id, name: tm.exercise_name ?? '' })
            return (
            <button key={tm.id}
              type="button"
              aria-pressed={selected}
              aria-label={`${tm.exercise_name}: ${formatWeight(tm.weight)}. Show history`}
              onClick={toggle}
              className={`rounded-lg border bg-card p-4 text-left cursor-pointer transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/50 ${
                selected ? 'border-primary' : 'border-border hover:border-primary/30'
              }`}>
              <p className="text-sm text-muted-foreground">{tm.exercise_name}</p>
              <p className="text-2xl font-bold mt-1">{formatWeight(tm.weight)}</p>
              <p className="text-xs text-muted-foreground mt-1">
                Effective {friendlyDate(tm.effective_date)}
              </p>
              {tm.notes && (
                <p className="text-xs text-muted-foreground mt-1">{tm.notes}</p>
              )}
            </button>
            )
          })}
        </div>
        )}
      </QueryState>
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
                  <TableCell>{friendlyDate(tm.effective_date)}</TableCell>
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