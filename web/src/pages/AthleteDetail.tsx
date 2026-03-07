import { useParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'

export function AthleteDetail() {
  const { id } = useParams<{ id: string }>()
  const athleteId = Number(id)

  const { data: athlete, isLoading, error } = useQuery({
    queryKey: ['athlete', athleteId],
    queryFn: () => api.getAthlete(athleteId),
    enabled: !isNaN(athleteId),
  })

  if (isLoading) return <p className="text-muted-foreground">Loading athlete...</p>
  if (error) return <p className="text-destructive">Failed to load athlete.</p>
  if (!athlete) return <p className="text-muted-foreground">Athlete not found.</p>

  return (
    <div>
      <h1 className="text-2xl font-bold mb-2">{athlete.name}</h1>
      {athlete.tier && (
        <span className="text-sm text-muted-foreground capitalize">{athlete.tier.replace('_', ' ')}</span>
      )}

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mt-6">
        {athlete.goal && (
          <div className="rounded-lg border border-border bg-card p-4">
            <h2 className="text-sm font-medium text-muted-foreground mb-1">Goal</h2>
            <p className="text-foreground">{athlete.goal}</p>
          </div>
        )}

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
    </div>
  )
}
