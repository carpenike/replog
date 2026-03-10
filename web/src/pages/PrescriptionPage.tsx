import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'
import { Spinner } from '@/components/ui'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

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
          <Card className="text-center">
            <CardContent>
            <span className="text-3xl block mb-2">📋</span>
            <p className="text-muted-foreground">No program assigned for today.</p>
            <p className="text-sm text-muted-foreground mt-1">Ask your coach to assign a program.</p>
            </CardContent>
          </Card>
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
            <Button onClick={() => window.location.href = `/athletes/${athleteId}/workouts/new`}>
              Start Logging
            </Button>
          </div>

          {prescription.lines.length === 0 ? (
            <p className="text-muted-foreground">Rest day — no exercises prescribed.</p>
          ) : (
            <div className="space-y-4">
              {prescription.lines.map((line, i) => (
                <Card key={`${line.exercise_id}-${i}`} size="sm">
                  <CardHeader>
                    <CardTitle>{line.exercise_name}</CardTitle>
                    {line.training_max && (
                      <CardDescription>TM: {formatWeight(line.training_max)}</CardDescription>
                    )}
                  </CardHeader>
                  <CardContent>
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead className="w-16">Set</TableHead>
                          <TableHead>Reps</TableHead>
                          <TableHead>Weight</TableHead>
                          <TableHead>%</TableHead>
                          <TableHead>Notes</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {line.sets.map((set, j) => (
                          <TableRow key={j}>
                            <TableCell className="text-muted-foreground">{set.set_number}</TableCell>
                            <TableCell className="font-medium">
                              {set.reps ? set.reps : 'AMRAP'}
                              {set.reps && set.rep_type === 'amrap' && '+'}
                            </TableCell>
                            <TableCell className="font-medium text-primary">
                              {set.target_weight ? formatWeight(set.target_weight)
                                : set.absolute_weight ? formatWeight(set.absolute_weight)
                                : 'BW'}
                            </TableCell>
                            <TableCell className="text-muted-foreground">
                              {set.percentage ? `${set.percentage}%` : ''}
                            </TableCell>
                            <TableCell className="text-muted-foreground text-xs">{set.notes ?? ''}</TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </CardContent>
                </Card>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  )
}
