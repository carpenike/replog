import { useState, useEffect } from 'react'
import { useParams, Link, useNavigate, useSearchParams } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import {
  ArrowRight, Bot, BookOpen, Check, ChevronDown, ChevronUp, ClipboardList, Download, Dumbbell, Eye, Flame,
  CalendarRange, Gauge, LineChart, ListChecks, Moon, NotebookPen, Pencil, Plus, Puzzle, RotateCcw, Scale,
  Target, Timer, TrendingUp, Upload, Wrench, X,
} from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { api } from '@/api/client'
import type { AthleteProgram } from '@/api/types'
import { EmptyState } from '@/components/ui'
import { QueryState } from '@/components/QueryState'
import { Skeleton } from '@/components/ui/skeleton'
import { useFriendlyDate } from '@/lib/dates'
import { usePageTitle } from '@/lib/usePageTitle'
import { useConfirm } from '@/lib/useConfirm'
import { Button, buttonVariants } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Card, CardContent } from '@/components/ui/card'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { cn, localDateISO } from '@/lib/utils'
import { nextTier, promoteCopy, tierLabel } from '@/lib/tiers'

const tierColors: Record<string, string> = {
  foundational: 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-400',
  intermediate: 'bg-amber-500/10 text-amber-700 dark:text-amber-400',
  sport_performance: 'bg-purple-500/10 text-purple-700 dark:text-purple-400',
}

const assignmentWeekdays = [
  { value: 1, label: 'Mon' },
  { value: 2, label: 'Tue' },
  { value: 3, label: 'Wed' },
  { value: 4, label: 'Thu' },
  { value: 5, label: 'Fri' },
  { value: 6, label: 'Sat' },
  { value: 7, label: 'Sun' },
]

function formatProgramSchedule(schedule?: string | null): string | null {
  if (!schedule) return null
  try {
    const days: unknown = JSON.parse(schedule)
    if (!Array.isArray(days)) return null
    const labels = days
      .filter((day): day is number => typeof day === 'number')
      .map(day => assignmentWeekdays.find(weekday => weekday.value === day)?.label)
      .filter((label): label is string => label != null)
    return labels.length > 0 ? labels.join(', ') : null
  } catch {
    return null
  }
}

/** One quick-nav link: lucide icon + label, styled as a small button. */
function QuickLink({ to, icon: Icon, label, primary }: { to: string; icon: LucideIcon; label: string; primary?: boolean }) {
  return (
    <Link to={to} className={buttonVariants({ variant: primary ? 'default' : 'outline', size: 'sm' })}>
      <Icon aria-hidden="true" />
      {label}
    </Link>
  )
}

export function AthleteDetail() {
  const navigate = useNavigate()
  const { id } = useParams<{ id: string }>()
  const athleteId = Number(id)
  const [searchParams, setSearchParams] = useSearchParams()
  // Deep-link from the generate page: ?assign=<templateId> pre-opens the
  // assign panel with the freshly-drafted template selected.
  const assignParam = searchParams.get('assign')
  const { data: me } = useQuery({
    queryKey: ['me'],
    queryFn: () => api.me(),
  })
  const isCoach = me?.is_coach || me?.is_admin
  const friendlyDate = useFriendlyDate()
  const athleteQuery = useQuery({
    queryKey: ['athlete', athleteId],
    queryFn: () => api.getAthlete(athleteId),
    enabled: !isNaN(athleteId),
  })
  const athlete = athleteQuery.data
  usePageTitle(athlete?.name)
  const { data: workouts } = useQuery({
    queryKey: ['workouts', athleteId, 'recent'],
    queryFn: () => api.listWorkouts(athleteId, 0),
    enabled: !isNaN(athleteId),
  })
  const { data: trainingMaxes } = useQuery({
    queryKey: ['training-maxes', athleteId],
    queryFn: () => api.listTrainingMaxes(athleteId),
    enabled: !isNaN(athleteId),
  })
  const { data: programs } = useQuery({
    queryKey: ['athlete-programs', athleteId],
    queryFn: () => api.listAthletePrograms(athleteId) as Promise<AthleteProgram[]>,
    enabled: !isNaN(athleteId),
  })
  const { confirm, dialog: confirmDialog } = useConfirm()
  const queryClient = useQueryClient()
  const [editingGoal, setEditingGoal] = useState(false)
  const [goalText, setGoalText] = useState('')
  const [showMore, setShowMore] = useState(false)
  const [showAssign, setShowAssign] = useState(assignParam != null)
  const [assignTemplateId, setAssignTemplateId] = useState(assignParam ?? '')
  const [assignDate, setAssignDate] = useState(localDateISO)
  const [assignRole, setAssignRole] = useState('primary')
  const [assignWeekdays, setAssignWeekdays] = useState<number[]>([])
  // Consume the deep-link param once so a refresh doesn't re-trigger it.
  useEffect(() => {
    if (assignParam != null) {
      searchParams.delete('assign')
      setSearchParams(searchParams, { replace: true })
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])
  const { data: allPrograms } = useQuery({
    queryKey: ['programs', 'for-athlete', athleteId],
    queryFn: () => api.listProgramTemplates(athleteId),
    enabled: showAssign && !isNaN(athleteId),
  })
  const goalMutation = useMutation({
    mutationFn: () => api.updateAthleteGoal(athleteId, goalText),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['athlete', athleteId] })
      setEditingGoal(false)
    },
  })
  const assignMutation = useMutation({
    mutationFn: () => api.assignProgram(athleteId, {
      template_id: parseInt(assignTemplateId),
      start_date: assignDate,
      role: assignRole,
      schedule: assignWeekdays.length > 0 ? JSON.stringify(assignWeekdays) : undefined,
    }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['athlete-programs', athleteId] })
      setShowAssign(false)
      setAssignTemplateId('')
      setAssignWeekdays([])
    },
  })

  function toggleAssignmentWeekday(day: number, checked: boolean) {
    setAssignWeekdays(current => {
      if (checked) return [...new Set([...current, day])].sort((a, b) => a - b)
      return current.filter(value => value !== day)
    })
  }
  const deactivateMutation = useMutation({
    mutationFn: (programId: number) => api.deactivateProgram(athleteId, programId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['athlete-programs', athleteId] }),
  })
  const promoteMutation = useMutation({
    mutationFn: () => api.promoteAthlete(athleteId),
    // The toast below names the athlete; skip the generic global one.
    meta: { skipGlobalError: true },
    onSuccess: (updated) => {
      queryClient.invalidateQueries({ queryKey: ['athlete', athleteId] })
      queryClient.invalidateQueries({ queryKey: ['athletes'] })
      toast.success(`${updated.name} promoted to ${tierLabel(updated.tier ?? '')}.`)
    },
    onError: (err) => {
      toast.error(`Couldn't promote: ${err instanceof Error ? err.message : 'something went wrong.'}`)
    },
  })
  async function handlePromote() {
    if (!athlete?.tier) return
    const copy = promoteCopy(athlete.name, athlete.tier)
    if (!copy) return
    if (await confirm(copy)) promoteMutation.mutate()
  }
  // Failed mutations surface through the global error toast (main.tsx).
  const reactivateMutation = useMutation({
    mutationFn: (programId: number) => api.reactivateProgram(athleteId, programId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['athlete-programs', athleteId] })
      toast.success('Program reactivated.')
    },
  })
  const deleteAssignmentMutation = useMutation({
    mutationFn: (programId: number) => api.deleteAthleteProgram(athleteId, programId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['athlete-programs', athleteId] })
      toast.success('Assignment removed.')
    },
  })
  const impersonateMutation = useMutation({
    mutationFn: (userId: number) => api.startImpersonation(userId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['me'] })
      window.location.href = '/'
    },
  })
  if (!athlete) {
    return (
      <QueryState
        query={athleteQuery}
        resource="athlete"
        skeleton="detail"
        notFound={<EmptyState icon="🔍" title="Athlete not found" action="All athletes" actionTo="/athletes" />}
      >
        {() => null}
      </QueryState>
    )
  }
  const recentWorkouts = workouts?.workouts.slice(0, 5) ?? []
  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-4">
          {athlete.avatar_url ? (
            <img src={athlete.avatar_url} alt="" className="h-16 w-16 rounded-full object-cover ring-2 ring-border" />
          ) : (
            <div aria-hidden="true" className="h-16 w-16 rounded-full bg-muted flex items-center justify-center text-2xl font-bold text-muted-foreground">
              {athlete.name.charAt(0).toUpperCase()}
            </div>
          )}
          <div>
            <h1 className="text-2xl font-bold">{athlete.name}</h1>
            {athlete.tier && (
            <span className={`text-xs px-2 py-0.5 rounded-full mt-1 inline-block ${tierColors[athlete.tier] ?? 'bg-muted text-muted-foreground'}`}>
              {tierLabel(athlete.tier)}
            </span>
          )}
          </div>
        </div>
        <div className="flex gap-2">
          {isCoach && nextTier(athlete.tier) && (
            <Button variant="ghost" onClick={handlePromote}
              disabled={promoteMutation.isPending}
              >
              <TrendingUp aria-hidden="true" />
              {promoteMutation.isPending ? 'Promoting…' : 'Promote'}
            </Button>
          )}
          {isCoach && (
            <Button variant="outline" size="sm" onClick={() => navigate(`/athletes/${athleteId}/edit`)}>
              <Pencil aria-hidden="true" />
              Edit
            </Button>
          )}
          {isCoach && athlete.linked_user_id && (
            <Button variant="ghost" size="sm"
              disabled={impersonateMutation.isPending}
              onClick={() => impersonateMutation.mutate(athlete.linked_user_id!)}>
              <Eye aria-hidden="true" />
              View as Athlete
            </Button>
          )}
        </div>
      </div>
      {/* Quick nav */}
      <div className="space-y-4 mb-6">
        {/* Train */}
        <div>
          <p className="text-xs font-medium text-muted-foreground mb-2">Train</p>
          <div className="flex flex-wrap gap-2">
            <QuickLink to={`/athletes/${athleteId}/prescription`} icon={ClipboardList} label="Today's Workout" primary />
            <QuickLink to={`/athletes/${athleteId}/workouts`} icon={NotebookPen} label="Workouts" />
            <QuickLink to={`/athletes/${athleteId}/body-weights`} icon={Scale} label="Body Weight" />
          </div>
        </div>
        {/* Sessions */}
        <div>
          <p className="text-xs font-medium text-muted-foreground mb-2">Sessions</p>
          <div className="flex flex-wrap gap-2">
            <QuickLink to={`/athletes/${athleteId}/throwing-sessions`} icon={Target} label="Throwing" />
            <QuickLink to={`/athletes/${athleteId}/conditioning-sessions`} icon={Timer} label="Conditioning" />
            <QuickLink to={`/athletes/${athleteId}/skill-sessions`} icon={Puzzle} label="Skill" />
            <QuickLink to={`/athletes/${athleteId}/recovery-checkins`} icon={Moon} label="Recovery" />
            <QuickLink to={`/athletes/${athleteId}/load`} icon={Gauge} label="Load" />
          </div>
        </div>
        {/* Program */}
        <div>
          <p className="text-xs font-medium text-muted-foreground mb-2">Program</p>
          <div className="flex flex-wrap gap-2">
            <QuickLink to={`/athletes/${athleteId}/training-maxes`} icon={Dumbbell} label="Training Maxes" />
            <QuickLink to={`/athletes/${athleteId}/accessories`} icon={Wrench} label="Accessories" />
            {isCoach && (
              <>
                <QuickLink to={`/athletes/${athleteId}/assignments`} icon={ListChecks} label="Assignments" />
                <QuickLink to={`/athletes/${athleteId}/cycle-review`} icon={LineChart} label="Cycle Review" />
                <QuickLink to={`/athletes/${athleteId}/season-phases`} icon={CalendarRange} label="Season Phases" />
                <QuickLink to={`/athletes/${athleteId}/tm-setup`} icon={Wrench} label="TM Setup" />
              </>
            )}
          </div>
        </div>
        {/* Coach tools */}
        {isCoach && (
          <div>
            <p className="text-xs font-medium text-muted-foreground mb-2">Coach Tools</p>
            <div className="flex flex-wrap gap-2">
              <QuickLink to={`/athletes/${athleteId}/generate`} icon={Bot} label="AI Coach" />
              <QuickLink to={`/athletes/${athleteId}/wod`} icon={Flame} label="WOD" />
              <QuickLink to={`/athletes/${athleteId}/import`} icon={Upload} label="Import" />
              <QuickLink to={`/athletes/${athleteId}/export`} icon={Download} label="Export" />
            </div>
          </div>
        )}
        {/* More */}
        <div>
          <Button variant="ghost" size="sm" aria-expanded={showMore} onClick={() => setShowMore(!showMore)}
            className="text-xs text-muted-foreground">
            {showMore ? <>Less <ChevronUp aria-hidden="true" /></> : <>More <ChevronDown aria-hidden="true" /></>}
          </Button>
          {showMore && (
            <div className="mt-2">
              <p className="text-xs font-medium text-muted-foreground mb-2">More</p>
              <div className="flex flex-wrap gap-2">
                <QuickLink to={`/athletes/${athleteId}/journal`} icon={BookOpen} label="Journal" />
              </div>
            </div>
          )}
        </div>
      </div>
      {/* Info cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-6">
        <div className="rounded-lg border border-border bg-card p-4">
          <h2 id="athlete-goal-heading" className="text-sm font-medium text-muted-foreground mb-1">Goal</h2>
          {editingGoal ? (
            <div>
              <Textarea aria-labelledby="athlete-goal-heading" value={goalText} onChange={e => setGoalText(e.target.value)}
                className="mb-2" />
              <div className="flex gap-2">
                <Button onClick={() => goalMutation.mutate()} disabled={goalMutation.isPending}
                  >Save</Button>
                <Button variant="ghost" onClick={() => setEditingGoal(false)}
                  >Cancel</Button>
              </div>
            </div>
          ) : (
            <button
              type="button"
              aria-label={athlete.goal ? `Edit goal: ${athlete.goal}` : 'Set goal'}
              onClick={() => { setGoalText(athlete.goal ?? ''); setEditingGoal(true) }}
              className="w-full min-h-11 rounded-md text-left text-foreground hover:text-primary/80 focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-ring/50 md:min-h-0"
            >
              {athlete.goal || <span className="text-muted-foreground italic">Tap to set goal...</span>}
            </button>
          )}
        </div>
        {athlete.notes && (
          <Card>
            <CardContent>
            <h2 className="text-sm font-medium text-muted-foreground mb-1">Notes</h2>
            <p className="text-foreground">{athlete.notes}</p>
            </CardContent>
          </Card>
        )}
        {athlete.date_of_birth && (
          <Card>
            <CardContent>
            <h2 className="text-sm font-medium text-muted-foreground mb-1">Date of Birth</h2>
            <p className="text-foreground">{friendlyDate(athlete.date_of_birth, { absolute: true })}</p>
            </CardContent>
          </Card>
        )}
      </div>
      {/* Active Programs */}
      <div className="mb-6">
        <div className="flex items-center justify-between mb-2">
          <h2 className="text-lg font-semibold">Programs</h2>
          {isCoach && (
            <Button variant="ghost" onClick={() => setShowAssign(!showAssign)}
              className="text-sm text-primary hover:text-primary/80">
              {showAssign ? <><X aria-hidden="true" /> Cancel</> : <><Plus aria-hidden="true" /> Assign</>}
            </Button>
          )}
        </div>
        {showAssign && (
          <form onSubmit={async (e) => {
            e.preventDefault()
            const existingInRole = programs?.find(p => p.active && p.role === assignRole)
            if (existingInRole) {
              const ok = await confirm({
                title: 'Replace Active Program',
                description: `This will deactivate "${existingInRole.template_name}" (${existingInRole.role}) and assign the new program.`,
                confirmLabel: 'Replace',
                variant: 'danger',
              })
              if (!ok) return
            }
            assignMutation.mutate()
          }}
            className="rounded-lg border border-border bg-card p-4 mb-3 space-y-3">
            <div className="grid grid-cols-2 md:grid-cols-3 gap-3">
              <div className="col-span-2 md:col-span-1">
                <Label htmlFor="assign-program">Program <span aria-hidden="true" className="text-destructive">*</span></Label>
                <Select value={assignTemplateId || null} onValueChange={(val) => setAssignTemplateId(val ?? "")} required>
                  <SelectTrigger id="assign-program" className="w-full">
                    <SelectValue placeholder="Select...">
                      {(value: string | null) => {
                        if (!value) return 'Select...'
                        const match = allPrograms?.find(p => String(p.id) === value)
                        return match?.name ?? value
                      }}
                    </SelectValue>
                  </SelectTrigger>
                  <SelectContent>
                    {allPrograms?.map(p => (
                      <SelectItem key={p.id} value={String(p.id)}>
                        {p.name}{p.athlete_id != null ? ' · athlete-specific' : ''}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div>
                <Label htmlFor="assign-start-date">Start Date <span aria-hidden="true" className="text-destructive">*</span></Label>
                <Input id="assign-start-date" type="date" value={assignDate} onChange={e => setAssignDate(e.target.value)} required />
              </div>
              <div>
                <Label htmlFor="assign-role">Role</Label>
                <Select value={assignRole} onValueChange={(val) => setAssignRole(val ?? "primary")}>
                  <SelectTrigger id="assign-role" className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="primary">Primary</SelectItem>
                    <SelectItem value="supplemental">Supplemental</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
      <fieldset>
        <legend className="mb-2 text-sm font-medium">Training Days</legend>
        <div className="flex flex-wrap gap-x-4 gap-y-2">
          {assignmentWeekdays.map(weekday => (
            <label key={weekday.value} htmlFor={`assignment-day-${weekday.value}`} className="flex min-h-11 items-center gap-2 text-sm md:min-h-0">
              <Checkbox
                id={`assignment-day-${weekday.value}`}
                checked={assignWeekdays.includes(weekday.value)}
                onCheckedChange={checked => toggleAssignmentWeekday(weekday.value, checked)}
              />
              {weekday.label}
            </label>
          ))}
        </div>
      </fieldset>
            <Button type="submit" disabled={assignMutation.isPending || !assignTemplateId}
              >
              Assign Program
            </Button>
          </form>
        )}
        {programs && programs.filter(p => p.active).length > 0 ? (
          <div className="space-y-2">
            {programs.filter(p => p.active).map(p => (
              <Card key={p.id} size="sm">
                <CardContent className="flex items-center justify-between">
                  <Link to={`/programs/${p.template_id}`} className="flex-1 hover:text-primary transition-colors">
                    <p className="font-medium">{p.template_name}</p>
                    <p className="text-xs text-muted-foreground">
                      Started {friendlyDate(p.start_date)} • {p.role}
                      {p.num_weeks ? ` • ${p.num_weeks}w` : ''}
                      {p.schedule ? ` • ${formatProgramSchedule(p.schedule) ?? 'Scheduled'}` : ''}
                    </p>
                  </Link>
                  <Button variant="ghost" size="sm" onClick={async () => { if (await confirm({ title: 'Deactivate Program', description: 'Deactivate this program assignment?', confirmLabel: 'Deactivate', variant: 'danger' })) deactivateMutation.mutate(p.id) }}
                    className="text-muted-foreground hover:text-destructive ml-3">
                    Deactivate
                  </Button>
                </CardContent>
              </Card>
            ))}
          </div>
        ) : (
          <p className="text-sm text-muted-foreground">No active programs.{isCoach && ' Assign one to start prescribing sessions.'}</p>
        )}

        {/* Deactivated Programs */}
        {programs && programs.filter(p => !p.active).length > 0 && (
          <details className="mt-3">
            <summary className="text-sm text-muted-foreground cursor-pointer hover:text-foreground">
              {programs.filter(p => !p.active).length} previous program{programs.filter(p => !p.active).length !== 1 ? 's' : ''}
            </summary>
            <div className="mt-2 space-y-1">
              {programs.filter(p => !p.active).map(p => (
                <div key={p.id} className="flex items-center justify-between py-1 text-sm text-muted-foreground">
                  <Link to={`/programs/${p.template_id}`} className="hover:text-foreground">
                    {p.template_name}
                  </Link>
                  <div className="flex items-center gap-2">
                    <span className="text-xs">{friendlyDate(p.start_date, { absolute: true })} – {p.deactivated_at ? friendlyDate(p.deactivated_at, { absolute: true, timestamp: true }) : '?'}</span>
                    {isCoach && (
                      <>
                        <Button variant="ghost" size="icon-sm"
                          aria-label={`Reactivate ${p.template_name}`}
                          disabled={reactivateMutation.isPending}
                          onClick={async () => {
                            if (await confirm({ title: 'Reactivate Program', description: `Reactivate "${p.template_name}"? Any active program in the same role will be deactivated.`, confirmLabel: 'Reactivate' })) {
                              reactivateMutation.mutate(p.id)
                            }
                          }}>
                          <RotateCcw aria-hidden="true" />
                        </Button>
                        <Button variant="ghost" size="icon-sm"
                          aria-label={`Delete ${p.template_name} assignment`}
                          className="hover:text-destructive"
                          disabled={deleteAssignmentMutation.isPending}
                          onClick={async () => {
                            if (await confirm({ title: 'Delete Assignment', description: `Remove this program assignment from history?`, confirmLabel: 'Delete', variant: 'danger' })) {
                              deleteAssignmentMutation.mutate(p.id)
                            }
                          }}>
                          <X aria-hidden="true" />
                        </Button>
                      </>
                    )}
                  </div>
                </div>
              ))}
            </div>
          </details>
        )}
      </div>
      {/* Equipment (coach only) */}
      {isCoach && <AthleteEquipmentSection athleteId={athleteId} />}

      {/* Training Maxes */}
      {trainingMaxes && trainingMaxes.length > 0 && (
        <div className="mb-6">
          <div className="flex items-center justify-between mb-2">
            <h2 className="text-lg font-semibold">Current Training Maxes</h2>
            <Link to={`/athletes/${athleteId}/training-maxes`} className="inline-flex min-h-11 items-center gap-1 text-sm text-primary hover:text-primary/80 md:min-h-0">
              View all <ArrowRight className="size-4" aria-hidden="true" />
            </Link>
          </div>
          <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-2">
            {trainingMaxes.map(tm => (
              <Card key={tm.id} size="sm">
                <CardContent>
                <p className="text-sm text-muted-foreground">{tm.exercise_name}</p>
                <p className="text-lg font-bold">{tm.weight}</p>
                </CardContent>
              </Card>
            ))}
          </div>
        </div>
      )}
      {/* Recent Workouts */}
      <div>
        <div className="flex items-center justify-between mb-2">
          <h2 className="text-lg font-semibold">Recent Workouts</h2>
          <Link to={`/athletes/${athleteId}/workouts`} className="text-sm text-primary hover:text-primary/80">
            View all &rarr;
          </Link>
        </div>
        {recentWorkouts.length === 0 ? (
          <EmptyState icon="🏋️" title="No workouts logged yet" action="Today's workout" actionTo={`/athletes/${athleteId}/prescription`} />
        ) : (
          <div className="space-y-2">
            {recentWorkouts.map(w => (
              <Link
                key={w.id}
                to={`/athletes/${athleteId}/workouts/${w.id}`}
                className="flex items-center justify-between rounded-lg border border-border bg-card p-3 hover:border-primary/50 transition-colors"
              >
                <div>
                  <p className="text-sm font-medium">{friendlyDate(w.date)}</p>
                  {w.program_name && <p className="text-xs text-muted-foreground">{w.program_name}</p>}
                </div>
                <span className="text-xs text-muted-foreground">{w.set_count} sets</span>
              </Link>
            ))}
          </div>
        )}
      </div>
      {confirmDialog()}
    </div>
  )
}

function AthleteEquipmentSection({ athleteId }: { athleteId: number }) {
  const queryClient = useQueryClient()
  const { confirm, dialog: confirmDialog } = useConfirm()
  const [pendingId, setPendingId] = useState<number | null>(null)

  const { data: owned, isLoading } = useQuery({
    queryKey: ['athlete-equipment', athleteId],
    queryFn: () => api.listAthleteEquipment(athleteId),
    enabled: !isNaN(athleteId),
  })

  const { data: all } = useQuery({
    queryKey: ['equipment'],
    queryFn: () => api.listEquipment(),
  })

  const addMutation = useMutation({
    mutationFn: (equipmentId: number) => api.addAthleteEquipment(athleteId, equipmentId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['athlete-equipment', athleteId] }),
    onSettled: () => setPendingId(null),
  })

  const removeMutation = useMutation({
    mutationFn: (equipmentId: number) => api.removeAthleteEquipment(athleteId, equipmentId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['athlete-equipment', athleteId] }),
    onSettled: () => setPendingId(null),
  })

  const ownedIds = new Set(owned?.map(o => o.EquipmentID) ?? [])

  return (
    <div className="mb-6">
      <h2 className="text-lg font-semibold mb-2">Equipment</h2>
      <p className="text-xs text-muted-foreground mb-3">
        Tap to toggle what this athlete has access to. Used by the program compatibility check.
      </p>
      {isLoading ? (
        <Skeleton className="h-8 w-full" />
      ) : !all || all.length === 0 ? (
        <p className="text-sm text-muted-foreground">No equipment defined yet. <Link to="/equipment" className="text-primary hover:underline">Add equipment</Link></p>
      ) : (
        <div className="flex flex-wrap gap-2">
          {all.map(e => {
            const isOwned = ownedIds.has(e.id)
            const isPending = pendingId === e.id
            return (
              <button
                key={e.id}
                type="button"
                disabled={isPending}
                aria-pressed={isOwned}
                onClick={async () => {
                  if (isOwned) {
                    if (await confirm({
                      title: 'Remove Equipment',
                      description: `Remove ${e.name} from this athlete's inventory?`,
                      confirmLabel: 'Remove',
                      variant: 'danger',
                    })) {
                      setPendingId(e.id)
                      removeMutation.mutate(e.id)
                    }
                  } else {
                    setPendingId(e.id)
                    addMutation.mutate(e.id)
                  }
                }}
                className={cn(
                  'inline-flex min-h-11 items-center gap-1.5 rounded-full border px-3 py-1 text-sm transition-colors disabled:opacity-50 md:min-h-0',
                  isOwned
                    ? 'border-primary bg-primary/15 text-primary hover:bg-primary/25'
                    : 'border-border text-muted-foreground hover:bg-muted',
                )}
              >
                {isOwned ? <Check className="size-3.5" aria-hidden="true" /> : <Plus className="size-3.5" aria-hidden="true" />}
                {e.name}
              </button>
            )
          })}
        </div>
      )}
      {confirmDialog()}
    </div>
  )
}