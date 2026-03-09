import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'
import { Spinner } from '@/components/ui'

function formatWeight(w: number): string {
  return w === Math.floor(w) ? w.toString() : w.toFixed(1)
}

export function PrescriptionPage() {
  const { id } = useParams<{ id: string }>()
  const athleteId = Number(id)

  const { data: athlete } = useQuery({
    queryKey: ['athlete', athleteId],
    queryFn: () => api.getAthlete(athleteId),
    enabled: !isNaN(athleteId),
  })

  const { data: prescription, isLoading, error } = useQuery({
    queryKey: ['prescription', athleteId],
    queryFn: () => api.getPrescription(athleteId),
    enabled: !isNaN(athleteId),
  })

  if (isLoading) return <Spinner />

  return (
    <div>
      <p className="text-sm text-muted-foreground mb-1">
        <Link to={`/athletes/${athleteId}`} className="hover:text-foreground">{athlete?.name ?? 'Athlete'}</Link>
        {' / Prescription'}
      </p>

      {error || !prescription ? (
        <div>
          <h1 className="text-2xl font-bold mb-4">Today's Workout</h1>
          <div className="rounded-lg border border-border bg-card p-6 text-center">
            <span className="text-3xl block mb-2">📋</span>
            <p className="text-muted-foreground">No program assigned for today.</p>
            <p className="text-sm text-muted-foreground mt-1">Ask your coach to assign a program.</p>
          </div>
        </div>
      ) : (
        <div>
          <div className="flex items-center justify-between mb-4">
            <div>
              <h1 className="text-2xl font-bold">Today's Workout</h1>
              <p className="text-sm text-muted-foreground">
                {prescription.program_name} — Week {prescription.current_week}, Day {prescription.current_day}
                {prescription.cycle_number > 1 && ` (Cycle ${prescription.cycle_number})`}
              </p>
            </div>
            <Link to={`/athletes/${athleteId}/workouts/new`}
              className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90">
              Start Logging
            </Link>
          </div>

          {prescription.lines.length === 0 ? (
            <p className="text-muted-foreground">Rest day — no exercises prescribed.</p>
          ) : (
            <div className="space-y-4">
              {prescription.lines.map((line, i) => (
                <div key={`${line.exercise_id}-${i}`} className="rounded-lg border border-border overflow-hidden">
                  <div className="bg-muted/50 px-4 py-3 border-b border-border flex items-center justify-between">
                    <div>
                      <h3 className="font-semibold">{line.exercise_name}</h3>
                      {line.training_max && (
                        <p className="text-xs text-muted-foreground">TM: {formatWeight(line.training_max)}</p>
                      )}
                    </div>
                  </div>
                  <table className="w-full">
                    <thead>
                      <tr className="text-xs text-muted-foreground border-b border-border">
                        <th className="text-left px-4 py-2 w-16">Set</th>
                        <th className="text-left px-4 py-2">Reps</th>
                        <th className="text-left px-4 py-2">Weight</th>
                        <th className="text-left px-4 py-2">%</th>
                        <th className="text-left px-4 py-2">Notes</th>
                      </tr>
                    </thead>
                    <tbody>
                      {line.sets.map((set, j) => (
                        <tr key={j} className="border-b border-border last:border-0 text-sm">
                          <td className="px-4 py-2 text-muted-foreground">{set.set_number}</td>
                          <td className="px-4 py-2 font-medium">
                            {set.reps ? set.reps : 'AMRAP'}
                            {set.reps && set.rep_type === 'amrap' && '+'}
                          </td>
                          <td className="px-4 py-2 font-medium text-primary">
                            {set.target_weight ? formatWeight(set.target_weight)
                              : set.absolute_weight ? formatWeight(set.absolute_weight)
                              : 'BW'}
                          </td>
                          <td className="px-4 py-2 text-muted-foreground">
                            {set.percentage ? `${set.percentage}%` : ''}
                          </td>
                          <td className="px-4 py-2 text-muted-foreground text-xs">{set.notes ?? ''}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  )
}
