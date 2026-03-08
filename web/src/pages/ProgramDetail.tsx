import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { ProgramTemplate } from '@/api/types'

interface PrescribedSetData {
  id: number
  exercise_name: string
  week: number
  day: number
  set_number: number
  reps: number | null
  percentage: number | null
  absolute_weight: number | null
  rep_type: string
  notes: string | null
}

function formatSetInfo(s: PrescribedSetData): string {
  const parts: string[] = []
  if (s.reps) parts.push(`${s.reps} reps`)
  else parts.push('AMRAP')
  if (s.percentage) parts.push(`@ ${s.percentage}%`)
  else if (s.absolute_weight) parts.push(`@ ${s.absolute_weight}`)
  return parts.join(' ')
}

export function ProgramDetail() {
  const { id } = useParams<{ id: string }>()
  const programId = Number(id)

  const { data, isLoading, error } = useQuery({
    queryKey: ['program', programId],
    queryFn: () => api.getProgramTemplate(programId),
    enabled: !isNaN(programId),
  })

  if (isLoading) return <p className="text-muted-foreground">Loading program...</p>
  if (error) return <p className="text-destructive">Failed to load program.</p>
  if (!data) return <p className="text-muted-foreground">Program not found.</p>

  const program = data.program as ProgramTemplate
  const sets = data.sets as PrescribedSetData[]

  // Group sets by week → day → exercise
  const weeks = new Map<number, Map<number, Map<string, PrescribedSetData[]>>>()
  for (const s of sets) {
    if (!weeks.has(s.week)) weeks.set(s.week, new Map())
    const days = weeks.get(s.week)!
    if (!days.has(s.day)) days.set(s.day, new Map())
    const exercises = days.get(s.day)!
    const key = s.exercise_name
    if (!exercises.has(key)) exercises.set(key, [])
    exercises.get(key)!.push(s)
  }

  return (
    <div>
      <p className="text-sm text-muted-foreground mb-1">
        <Link to="/programs" className="hover:text-foreground">Programs</Link>
        {' / '}
        {program.name}
      </p>
      <h1 className="text-2xl font-bold mb-2">{program.name}</h1>
      {program.description && (
        <p className="text-muted-foreground mb-4">{program.description}</p>
      )}

      <div className="flex gap-3 mb-6 text-sm text-muted-foreground">
        <span>{program.num_weeks} week{program.num_weeks !== 1 ? 's' : ''}</span>
        <span>•</span>
        <span>{program.num_days} day{program.num_days !== 1 ? 's' : ''}/week</span>
        {program.is_loop && <><span>•</span><span className="text-primary">Looping</span></>}
      </div>

      {sets.length === 0 ? (
        <p className="text-muted-foreground">No prescribed sets defined.</p>
      ) : (
        <div className="space-y-8">
          {Array.from(weeks.entries()).sort((a, b) => a[0] - b[0]).map(([week, days]) => (
            <div key={week}>
              <h2 className="text-lg font-semibold mb-3">Week {week}</h2>
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                {Array.from(days.entries()).sort((a, b) => a[0] - b[0]).map(([day, exercises]) => (
                  <div key={day} className="rounded-lg border border-border bg-card p-4">
                    <h3 className="text-sm font-medium text-muted-foreground mb-3">Day {day}</h3>
                    <div className="space-y-3">
                      {Array.from(exercises.entries()).map(([exerciseName, exSets]) => (
                        <div key={exerciseName}>
                          <p className="text-sm font-medium">{exerciseName}</p>
                          <div className="mt-1 space-y-0.5">
                            {exSets.map(s => (
                              <p key={s.id} className="text-xs text-muted-foreground">
                                Set {s.set_number}: {formatSetInfo(s)}
                                {s.notes && ` — ${s.notes}`}
                              </p>
                            ))}
                          </div>
                        </div>
                      ))}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
