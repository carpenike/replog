import { useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useQuery, useMutation } from '@tanstack/react-query'
import { toast } from 'sonner'
import { api, ApiError } from '@/api/client'
import { Link2, Play } from 'lucide-react'
import { EmptyState } from '@/components/ui'
import { PageHeader } from '@/components/PageHeader'
import { QueryState } from '@/components/QueryState'
import { cn, formatDate, formatWeight, localDateISO } from '@/lib/utils'
import { groupBlocks, groupKind, memberLabel } from '@/lib/groups'
import { distinctNotes, summarizeSets } from '@/lib/setSummary'
import { CueNote } from '@/components/CueNote'
import { Badge } from '@/components/ui/badge'
import type { PrescriptionLineData, PrescriptionSetData } from '@/api/types'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'

export function PrescriptionPage() {
  const navigate = useNavigate()
  const { id } = useParams<{ id: string }>()
  const athleteId = Number(id)

  const { data: athlete } = useQuery({
    queryKey: ['athlete', athleteId],
    queryFn: () => api.getAthlete(athleteId),
    enabled: !isNaN(athleteId),
  })

  // The program day to start on. null is the default: the next day RepLog
  // prescribes. Picking one shows that day and stores it on the session.
  const [choice, setChoice] = useState<{ week: number; day: number } | null>(null)

  const query = useQuery({
    queryKey: ['prescription', athleteId, choice],
    queryFn: () => api.getPrescription(athleteId, choice ?? undefined),
    enabled: !isNaN(athleteId),
    retry: false,
    placeholderData: prev => prev,
  })

  const startLogging = useMutation({
    // fromPrescription=false: the session screen logs sets as the athlete
    // completes them — pre-creating the full prescription would mark every
    // row done (and record volume that was never performed). Local date, not
    // UTC: near midnight toISOString() lands the workout on the wrong day.
    // The session links to today's program and stores its week/day: the one
    // picked here, or RepLog's next day (ADR 028).
    mutationFn: () => api.createWorkout(athleteId, localDateISO(), '', false, choice ? { link: true, ...choice } : { link: true }),
    meta: { skipGlobalError: true },
    onSuccess: (workout) => {
      navigate(`/athletes/${athleteId}/workouts/${workout.id}/session`)
    },
    onError: async (err) => {
      // A workout already exists for today — resume its live session. The 409
      // was raised against the same date string createWorkout sent, so match
      // on it to find the colliding workout.
      if (err instanceof ApiError && err.code === 409) {
        toast.info('A workout already exists for today — resuming it.')
        const today = localDateISO()
        try {
          const page = await api.listWorkouts(athleteId)
          const existing = page.workouts.find(w => formatDate(w.date) === today)
          navigate(existing
            ? `/athletes/${athleteId}/workouts/${existing.id}/session`
            : `/athletes/${athleteId}/workouts`)
        } catch {
          navigate(`/athletes/${athleteId}/workouts`)
        }
        return
      }
      toast.error('Failed to start logging.')
    },
  })

  const prescription = query.data

  return (
    <div>
      <PageHeader
        title="Today's Workout"
        back={{ to: `/athletes/${athleteId}`, label: athlete?.name ?? 'Athlete' }}
        description={prescription && (
          <>
            {prescription.program_name} · Week {prescription.current_week}, Day {prescription.current_day}
            {prescription.cycle_number > 1 && ` (Cycle ${prescription.cycle_number})`}
          </>
        )}
        actions={prescription && (
          <Button size="touch" onClick={() => startLogging.mutate()} disabled={startLogging.isPending}>
            <Play aria-hidden="true" /> {startLogging.isPending ? 'Starting…' : 'Start Logging'}
          </Button>
        )}
      />

      {prescription && prescription.num_weeks * prescription.num_days > 1 && (
        <div className="mb-4 max-w-xs">
          <Label htmlFor="program-day">Program day</Label>
          <Select
            value={`${prescription.current_week}-${prescription.current_day}`}
            onValueChange={val => {
              const [week, day] = String(val ?? '').split('-').map(Number)
              if (week && day) setChoice({ week, day })
            }}
          >
            <SelectTrigger id="program-day" className="w-full">
              <SelectValue>
                {(value: string | null) => {
                  const [week, day] = String(value ?? '').split('-')
                  return `Week ${week}, Day ${day}`
                }}
              </SelectValue>
            </SelectTrigger>
            <SelectContent>
              {Array.from({ length: prescription.num_weeks }, (_, w) =>
                Array.from({ length: prescription.num_days }, (_, d) => (
                  <SelectItem key={`${w + 1}-${d + 1}`} value={`${w + 1}-${d + 1}`}>
                    Week {w + 1}, Day {d + 1}
                  </SelectItem>
                )),
              )}
            </SelectContent>
          </Select>
        </div>
      )}

      {/* A 404 means no program is assigned for today: an expected empty state. */}
      <QueryState
        query={query}
        resource="prescription"
        skeleton="detail"
        notFound={<EmptyState icon="📋" title="No program assigned for today" description="Ask your coach to assign a program." />}
        isEmpty={p => p.lines.length === 0}
        empty={{ icon: '😌', title: 'Rest day', description: 'No exercises prescribed.' }}
      >
        {prescription => (
            <div className="space-y-4">
              {groupBlocks(prescription.lines, l => l.group_key ?? l.sets.find(st => st.group_key)?.group_key).map((block, bi) =>
                block.key && block.items.length >= 2 ? (
                  <section key={`g-${block.key}`} aria-label={`${groupKind(block.items.length)} ${block.key}`} className="space-y-2 rounded-xl border-2 border-primary/30 p-2">
                    <p className="flex items-center gap-1.5 px-1 text-xs font-medium text-primary">
                      <Link2 className="size-3.5" aria-hidden="true" />
                      {groupKind(block.items.length)} {block.key} · alternate {block.items.map((_, i) => memberLabel(block.key as string, i)).join(' / ')} each round, rest after the round
                    </p>
                    {block.items.map((line, i) => (
                      <PrescriptionLineCard key={`${line.exercise_id}-${i}`} line={line} member={memberLabel(block.key as string, i)} />
                    ))}
                  </section>
                ) : (
                  block.items.map((line, i) => <PrescriptionLineCard key={`${line.exercise_id}-${bi}-${i}`} line={line} />)
                ))}
            </div>
        )}
      </QueryState>
    </div>
  )
}

/** Target load for a prescription line: "225 lb (75%)", "135 lb", or null for bodyweight. */
function prescriptionLoad(s: PrescriptionSetData): string | null {
  if (s.target_weight) return s.percentage ? `${formatWeight(s.target_weight)} (${s.percentage}%)` : formatWeight(s.target_weight)
  if (s.absolute_weight) return formatWeight(s.absolute_weight)
  if (s.percentage) return `${s.percentage}%`
  return null
}

/**
 * One exercise of today's prescription: warm-ups first (not counted), then
 * working sets. Matching sets share one line and the coach's cue shows once.
 */
function PrescriptionLineCard({ line, member }: { line: PrescriptionLineData; member?: string }) {
  // Warm-ups sort ahead of the working sets they lead into.
  const sets = [...line.sets].sort((a, b) => Number(!a.non_working) - Number(!b.non_working))
  const lines = summarizeSets(sets, { load: s => prescriptionLoad(s as PrescriptionSetData) })
  const notes = distinctNotes(sets)
  return (
    <Card size="sm">
      <CardHeader>
        <CardTitle className="flex items-center gap-1.5">
          {member && <Badge variant="secondary">{member}</Badge>}
          {line.exercise_name}
        </CardTitle>
        {line.training_max && (
          <CardDescription>TM: {formatWeight(line.training_max)}</CardDescription>
        )}
      </CardHeader>
      <CardContent className="space-y-1">
        <ul className="space-y-0.5 text-sm tabular-nums">
          {lines.map((l, i) => (
            <li key={i} className={cn(l.warmup && 'text-muted-foreground')}>
              {l.warmup && <Badge variant="outline" className="mr-1.5 text-[11px]">Warm-up</Badge>}
              <span className={cn('font-semibold', !l.warmup && 'text-foreground')}>{l.head}</span>
              {l.detail.map(d => <span key={d} className="text-muted-foreground"> · {d}</span>)}
            </li>
          ))}
        </ul>
        {notes.map(n => <CueNote key={n} text={n} />)}
      </CardContent>
    </Card>
  )
}
