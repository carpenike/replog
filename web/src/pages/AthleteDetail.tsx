import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { AthleteProgram } from '@/api/types'
import { Spinner } from '@/components/ui'
import { useConfirm } from '@/lib/useConfirm'

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

  if (isLoading) return <Spinner />
  if (error) return <p className="text-destructive">Failed to load athlete.</p>
  const promoteMutation = useMutation({
    mutationFn: () => api.promoteAthlete(athleteId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['athlete', athleteId] })
      queryClient.invalidateQueries({ queryKey: ['athletes'] })
    },
  })

  if (!athlete) return <p className="text-muted-foreground">Athlete not found.</p>

  const recentWorkouts = workouts?.workouts.slice(0, 5) ?? []

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold">{athlete.name}</h1>
          {athlete.tier && (
            <span className={`text-xs px-2 py-0.5 rounded-full mt-1 inline-block ${tierColors[athlete.tier] ?? 'bg-muted text-muted-foreground'}`}>
              {tierLabel(athlete.tier)}
            </span>
          )}
        </div>
        <div className="flex gap-2">
          {athlete.tier && athlete.tier !== 'sport_performance' && (
            <button onClick={() => promoteMutation.mutate()}
              disabled={promoteMutation.isPending}
              className="rounded-md border border-border px-3 py-1.5 text-sm hover:bg-accent transition-colors disabled:opacity-50">
              📈 Promote
            </button>
          )}
          <Link to={`/athletes/${athleteId}/edit`}
            className="rounded-md border border-border px-3 py-1.5 text-sm hover:bg-accent transition-colors">
            ✏️ Edit
          </Link>
        </div>
      </div>

      {/* Quick nav */}
      <div className="flex flex-wrap gap-2 mb-6">
        <Link to={`/athletes/${athleteId}/prescription`} className="rounded-md border border-primary/30 bg-primary/5 px-3 py-1.5 text-sm hover:border-primary/50 transition-colors font-medium">
          📋 Today's Workout
        </Link>
        <Link to={`/athletes/${athleteId}/workouts`} className="rounded-md border border-border bg-card px-3 py-1.5 text-sm hover:border-primary/50 transition-colors">
          📝 Workouts
        </Link>
        <Link to={`/athletes/${athleteId}/body-weights`} className="rounded-md border border-border bg-card px-3 py-1.5 text-sm hover:border-primary/50 transition-colors">
          ⚖️ Body Weight
        </Link>
        <Link to={`/athletes/${athleteId}/training-maxes`} className="rounded-md border border-border bg-card px-3 py-1.5 text-sm hover:border-primary/50 transition-colors">
          💪 Training Maxes
        </Link>
        <Link to={`/athletes/${athleteId}/journal`} className="rounded-md border border-border bg-card px-3 py-1.5 text-sm hover:border-primary/50 transition-colors">
          📖 Journal
        </Link>
        <Link to={`/athletes/${athleteId}/accessories`} className="rounded-md border border-border bg-card px-3 py-1.5 text-sm hover:border-primary/50 transition-colors">
          🔧 Accessories
        </Link>
        <Link to={`/athletes/${athleteId}/assignments`} className="rounded-md border border-border bg-card px-3 py-1.5 text-sm hover:border-primary/50 transition-colors">
          🎯 Assignments
        </Link>
        <Link to={`/athletes/${athleteId}/cycle-review`} className="rounded-md border border-border bg-card px-3 py-1.5 text-sm hover:border-primary/50 transition-colors">
          📈 Cycle Review
        </Link>
        <Link to={`/athletes/${athleteId}/export`} className="rounded-md border border-border bg-card px-3 py-1.5 text-sm hover:border-primary/50 transition-colors">
          📦 Export
        </Link>
      </div>

      {/* Info cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-6">
        <div className="rounded-lg border border-border bg-card p-4"
          onClick={() => { if (!editingGoal) { setGoalText(athlete.goal ?? ''); setEditingGoal(true) } }}>
          <h2 className="text-sm font-medium text-muted-foreground mb-1">Goal</h2>
          {editingGoal ? (
            <div onClick={e => e.stopPropagation()}>
              <textarea value={goalText} onChange={e => setGoalText(e.target.value)}
                rows={2} className="w-full rounded-md border border-border bg-background px-3 py-1.5 text-sm mb-2" />
              <div className="flex gap-2">
                <button onClick={() => goalMutation.mutate()} disabled={goalMutation.isPending}
                  className="rounded-md bg-primary px-3 py-1 text-xs text-primary-foreground hover:bg-primary/90">Save</button>
                <button onClick={() => setEditingGoal(false)}
                  className="text-xs text-muted-foreground hover:text-foreground">Cancel</button>
              </div>
            </div>
          ) : (
            <p className="text-foreground cursor-pointer hover:text-primary/80">
              {athlete.goal || <span className="text-muted-foreground italic">Click to set goal...</span>}
            </p>
          )}
        </div>
        {athlete.notes && (
          <div className="rounded-lg border border-border bg-card p-4">
            <h2 className="text-sm font-medium text-muted-foreground mb-1">Notes</h2>
            <p className="text-foreground">{athlete.notes}</p>
          </div>
        )}
        {athlete.date_of_birth && (
          <div className="rounded-lg border border-border bg-card p-4">
            <h2 className="text-sm font-medium text-muted-foreground mb-1">Date of Birth</h2>
            <p className="text-foreground">{athlete.date_of_birth}</p>
          </div>
        )}
      </div>

      {/* Active Programs */}
      <div className="mb-6">
        <div className="flex items-center justify-between mb-2">
          <h2 className="text-lg font-semibold">Programs</h2>
          <button onClick={() => setShowAssign(!showAssign)}
            className="text-sm text-primary hover:text-primary/80">
            {showAssign ? 'Cancel' : '+ Assign'}
          </button>
        </div>

        {showAssign && (
          <form onSubmit={(e) => { e.preventDefault(); assignMutation.mutate() }}
            className="rounded-lg border border-border bg-card p-4 mb-3 space-y-3">
            <div className="grid grid-cols-2 md:grid-cols-3 gap-3">
              <div className="col-span-2 md:col-span-1">
                <label className="block text-xs text-muted-foreground mb-1">Program</label>
                <select value={assignTemplateId} onChange={e => setAssignTemplateId(e.target.value)} required
                  className="w-full rounded-md border border-border bg-background px-3 py-1.5 text-sm">
                  <option value="">Select...</option>
                  {allPrograms?.map(p => (
                    <option key={p.id} value={p.id}>{p.name}</option>
                  ))}
                </select>
              </div>
              <div>
                <label className="block text-xs text-muted-foreground mb-1">Start Date</label>
                <input type="date" value={assignDate} onChange={e => setAssignDate(e.target.value)} required
                  className="w-full rounded-md border border-border bg-background px-3 py-1.5 text-sm" />
              </div>
              <div>
                <label className="block text-xs text-muted-foreground mb-1">Role</label>
                <select value={assignRole} onChange={e => setAssignRole(e.target.value)}
                  className="w-full rounded-md border border-border bg-background px-3 py-1.5 text-sm">
                  <option value="primary">Primary</option>
                  <option value="supplemental">Supplemental</option>
                </select>
              </div>
            </div>
            <button type="submit" disabled={assignMutation.isPending || !assignTemplateId}
              className="rounded-md bg-primary px-4 py-1.5 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50">
              Assign Program
            </button>
          </form>
        )}

        {programs && programs.filter(p => p.active).length > 0 ? (
          <div className="space-y-2">
            {programs.filter(p => p.active).map(p => (
              <div key={p.id} className="flex items-center justify-between rounded-lg border border-border bg-card p-3">
                <Link to={`/programs/${p.template_id}`} className="flex-1 hover:text-primary transition-colors">
                  <p className="font-medium">{p.template_name}</p>
                  <p className="text-xs text-muted-foreground">
                    Started {p.start_date} • {p.role}
                    {p.num_weeks ? ` • ${p.num_weeks}w` : ''}
                  </p>
                </Link>
                <button onClick={async () => { if (await confirm({ title: 'Deactivate Program', description: 'Deactivate this program assignment?', confirmLabel: 'Deactivate', variant: 'danger' })) deactivateMutation.mutate(p.id) }}
                  className="text-xs text-muted-foreground hover:text-destructive ml-3">
                  Deactivate
                </button>
              </div>
            ))}
          </div>
        ) : (
          <p className="text-sm text-muted-foreground">No active programs.</p>
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
              <div key={tm.id} className="rounded-lg border border-border bg-card p-3">
                <p className="text-sm text-muted-foreground">{tm.exercise_name}</p>
                <p className="text-lg font-bold">{tm.weight}</p>
              </div>
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
