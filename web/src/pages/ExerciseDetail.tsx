import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'
import { Spinner } from '@/components/ui'
import { Card, CardContent } from '@/components/ui/card'

export function ExerciseDetail() {
  const { id } = useParams<{ id: string }>()
  const exerciseId = Number(id)

  const { data: exercise, isLoading, error } = useQuery({
    queryKey: ['exercise', exerciseId],
    queryFn: () => api.getExercise(exerciseId),
    enabled: !isNaN(exerciseId),
  })

  if (isLoading) return <Spinner />
  if (error) return <p className="text-destructive">Failed to load exercise.</p>
  if (!exercise) return <p className="text-muted-foreground">Exercise not found.</p>

  return (
    <div>
      <p className="text-sm text-muted-foreground mb-1">
        <Link to="/exercises" className="hover:text-foreground">Exercises</Link>
        {' / '}
        {exercise.name}
      </p>
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-3">
          <h1 className="text-2xl font-bold">{exercise.name}</h1>
          {exercise.featured && <span className="text-sm">⭐</span>}
        </div>
        <Link to={`/exercises/${exerciseId}/edit`}
          className="rounded-md border border-border px-3 py-1.5 text-sm hover:bg-accent transition-colors">
          ✏️ Edit
        </Link>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {exercise.tier && (
          <Card>
            <CardContent>
            <h2 className="text-sm font-medium text-muted-foreground mb-1">Tier</h2>
            <p className="text-foreground capitalize">{exercise.tier.replace('_', ' ')}</p>
            </CardContent>
          </Card>
        )}

        {exercise.rest_seconds && (
          <Card>
            <CardContent>
            <h2 className="text-sm font-medium text-muted-foreground mb-1">Rest Timer</h2>
            <p className="text-foreground">{exercise.rest_seconds}s</p>
            </CardContent>
          </Card>
        )}

        {exercise.form_notes && (
          <Card className="md:col-span-2">
            <CardContent>
            <h2 className="text-sm font-medium text-muted-foreground mb-1">Form Notes</h2>
            <p className="text-foreground whitespace-pre-wrap">{exercise.form_notes}</p>
            </CardContent>
          </Card>
        )}

        {exercise.demo_url && (
          <Card className="md:col-span-2">
            <CardContent>
            <h2 className="text-sm font-medium text-muted-foreground mb-1">Demo Video</h2>
            <a href={exercise.demo_url} target="_blank" rel="noopener noreferrer"
              className="text-primary hover:text-primary/80 text-sm break-all">
              {exercise.demo_url}
            </a>
            </CardContent>
          </Card>
        )}
      </div>
    </div>
  )
}
