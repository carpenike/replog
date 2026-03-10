import { useState } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { AthleteProgram } from '@/api/types'
import { Spinner } from '@/components/ui'
import { useConfirm } from '@/lib/useConfirm'
import { Button, buttonVariants } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Card, CardContent } from '@/components/ui/card'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { formatDate } from '@/lib/utils'

const tierColors: Record<string, string> = {
  foundational: 'bg-emerald-500/10 text-emerald-400',
  intermediate: 'bg-amber-500/10 text-amber-400',
  sport_performance: 'bg-purple-500/10 text-purple-400',
}
function tierLabel(tier: string): string {
  switch (tier) {
    case 'foundational': return 'Foundational'
    case 'intermediate': return 'Intermediate'
    case 'sport_performance': return 'Sport Performance'
    default: return tier
  }
}
export function AthleteDetail() {
  const navigate = useNavigate()
  const { id } = useParams<{ id: string }>()
  const athleteId = Number(id)
  const { data: athlete, isLoading, error } = useQuery({
    queryKey: ['athlete', athleteId],
    queryFn: () => api.getAthlete(athleteId),
    enabled: !isNaN(athleteId),
  })
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
  const [showAssign, setShowAssign] = useState(false)
  const [assignTemplateId, setAssignTemplateId] = useState('')
  const [assignDate, setAssignDate] = useState(new Date().toISOString().slice(0, 10))
  const [assignRole, setAssignRole] = useState('primary')
  const { data: allPrograms } = useQuery({
    queryKey: ['programs'],
    queryFn: () => api.listProgramTemplates(),
    enabled: showAssign,
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
    }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['athlete-programs', athleteId] })
      setShowAssign(false)
      setAssignTemplateId('')
    },
  })
  const deactivateMutation = useMutation({
    mutationFn: (programId: number) => api.deactivateProgram(athleteId, programId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['athlete-programs', athleteId] }),
  })
  const promoteMutation = useMutation({
    mutationFn: () => api.promoteAthlete(athleteId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['athlete', athleteId] })
      queryClient.invalidateQueries({ queryKey: ['athletes'] })
    },
  })
  if (isLoading) return <Spinner />
  if (error) return <p className="text-destructive">Failed to load athlete.</p>
  if (!athlete) return <p className="text-muted-foreground">Athlete not found.</p>
  const recentWorkouts = workouts?.workouts.slice(0, 5) ?? []
  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-4">
          {athlete.avatar_url ? (
            <img src={athlete.avatar_url} alt={athlete.name} className="h-16 w-16 rounded-full object-cover ring-2 ring-border" />
          ) : (
            <div className="h-16 w-16 rounded-full bg-muted flex items-center justify-center text-2xl font-bold text-muted-foreground">
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
          {athlete.tier && athlete.tier !== 'sport_performance' && (
            <Button variant="ghost" onClick={() => promoteMutation.mutate()}
              disabled={promoteMutation.isPending}
              >
              📈 Promote
            </Button>
          )}
          <Button variant="outline" size="sm" onClick={() => navigate(`/athletes/${athleteId}/edit`)}>
            ✏️ Edit
          </Button>
        </div>
      </div>
      {/* Quick nav */}
      <div className="flex flex-wrap gap-2 mb-6">
        <Link to={`/athletes/${athleteId}/prescription`} className={buttonVariants({ variant: "default", size: "sm" })}>
          📋 Today's Workout
        </Link>
        <Link to={`/athletes/${athleteId}/workouts`} className={buttonVariants({ variant: "outline", size: "sm" })}>
          📝 Workouts
        </Link>
        <Link to={`/athletes/${athleteId}/body-weights`} className={buttonVariants({ variant: "outline", size: "sm" })}>
          ⚖️ Body Weight
        </Link>
        <Link to={`/athletes/${athleteId}/training-maxes`} className={buttonVariants({ variant: "outline", size: "sm" })}>
          💪 Training Maxes
        </Link>
        <Link to={`/athletes/${athleteId}/journal`} className={buttonVariants({ variant: "outline", size: "sm" })}>
          📖 Journal
        </Link>
        <Link to={`/athletes/${athleteId}/accessories`} className={buttonVariants({ variant: "outline", size: "sm" })}>
          🔧 Accessories
        </Link>
        <Link to={`/athletes/${athleteId}/assignments`} className={buttonVariants({ variant: "outline", size: "sm" })}>
          🎯 Assignments
        </Link>
        <Link to={`/athletes/${athleteId}/tm-setup`} className={buttonVariants({ variant: "outline", size: "sm" })}>
          🔧 TM Setup
        </Link>
        <Link to={`/athletes/${athleteId}/cycle-review`} className={buttonVariants({ variant: "outline", size: "sm" })}>
          📈 Cycle Review
        </Link>
        <Link to={`/athletes/${athleteId}/export`} className={buttonVariants({ variant: "outline", size: "sm" })}>
          📦 Export
        </Link>
        <Link to={`/athletes/${athleteId}/import`} className={buttonVariants({ variant: "outline", size: "sm" })}>
          📥 Import
        </Link>
        <Link to={`/athletes/${athleteId}/generate`} className={buttonVariants({ variant: "outline", size: "sm" })}>
          🤖 AI Coach
        </Link>
      </div>
      {/* Info cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-6">
        <div className="rounded-lg border border-border bg-card p-4"
          onClick={() => { if (!editingGoal) { setGoalText(athlete.goal ?? ''); setEditingGoal(true) } }}>
          <h2 className="text-sm font-medium text-muted-foreground mb-1">Goal</h2>
          {editingGoal ? (
            <div onClick={e => e.stopPropagation()}>
              <Textarea value={goalText} onChange={e => setGoalText(e.target.value)}
                className="mb-2" />
              <div className="flex gap-2">
                <Button variant="ghost" onClick={() => goalMutation.mutate()} disabled={goalMutation.isPending}
                  >Save</Button>
                <Button variant="ghost" onClick={() => setEditingGoal(false)}
                  >Cancel</Button>
              </div>
            </div>
          ) : (
            <p className="text-foreground cursor-pointer hover:text-primary/80">
              {athlete.goal || <span className="text-muted-foreground italic">Click to set goal...</span>}
            </p>
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
            <p className="text-foreground">{formatDate(athlete.date_of_birth)}</p>
            </CardContent>
          </Card>
        )}
      </div>
      {/* Active Programs */}
      <div className="mb-6">
        <div className="flex items-center justify-between mb-2">
          <h2 className="text-lg font-semibold">Programs</h2>
          <Button variant="ghost" onClick={() => setShowAssign(!showAssign)}
            className="text-sm text-primary hover:text-primary/80">
            {showAssign ? 'Cancel' : '+ Assign'}
          </Button>
        </div>
        {showAssign && (
          <form onSubmit={(e) => { e.preventDefault(); assignMutation.mutate() }}
            className="rounded-lg border border-border bg-card p-4 mb-3 space-y-3">
            <div className="grid grid-cols-2 md:grid-cols-3 gap-3">
              <div className="col-span-2 md:col-span-1">
                <Label >Program</Label>
                <Select value={assignTemplateId || null} onValueChange={(val) => setAssignTemplateId(val ?? "")} required>
                  <SelectTrigger className="w-full">
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
                      <SelectItem key={p.id} value={String(p.id)}>{p.name}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div>
                <Label >Start Date</Label>
                <Input type="date" value={assignDate} onChange={e => setAssignDate(e.target.value)} required />
              </div>
              <div>
                <Label >Role</Label>
                <Select value={assignRole} onValueChange={(val) => setAssignRole(val ?? "primary")}>
                  <SelectTrigger className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="primary">Primary</SelectItem>
                    <SelectItem value="supplemental">Supplemental</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
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
                      Started {formatDate(p.start_date)} • {p.role}
                      {p.num_weeks ? ` • ${p.num_weeks}w` : ''}
                    </p>
                  </Link>
                  <Button variant="ghost" size="xs" onClick={async () => { if (await confirm({ title: 'Deactivate Program', description: 'Deactivate this program assignment?', confirmLabel: 'Deactivate', variant: 'danger' })) deactivateMutation.mutate(p.id) }}
                    className="text-muted-foreground hover:text-destructive ml-3">
                    Deactivate
                  </Button>
                </CardContent>
              </Card>
            ))}
          </div>
        ) : (
          <p className="text-sm text-muted-foreground">No active programs.</p>
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
                  <span className="text-xs">{formatDate(p.start_date)} – {p.deactivated_at ? formatDate(p.deactivated_at) : '?'}</span>
                </div>
              ))}
            </div>
          </details>
        )}
      </div>
      {/* Training Maxes */}
      {trainingMaxes && trainingMaxes.length > 0 && (
        <div className="mb-6">
          <div className="flex items-center justify-between mb-2">
            <h2 className="text-lg font-semibold">Current Training Maxes</h2>
            <Link to={`/athletes/${athleteId}/training-maxes`} className="text-sm text-primary hover:text-primary/80">
              View all &rarr;
            </Link>
          </div>
          <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-2">
            {trainingMaxes.map(tm => (
              <Card size="sm">
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
          <p className="text-sm text-muted-foreground">No workouts logged yet.</p>
        ) : (
          <div className="space-y-2">
            {recentWorkouts.map(w => (
              <Link
                key={w.id}
                to={`/athletes/${athleteId}/workouts/${w.id}`}
                className="flex items-center justify-between rounded-lg border border-border bg-card p-3 hover:border-primary/50 transition-colors"
              >
                <div>
                  <p className="text-sm font-medium">{w.date}</p>
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