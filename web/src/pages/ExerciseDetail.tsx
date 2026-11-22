import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { Pencil, Star } from 'lucide-react'
import { api } from '@/api/client'
import { PageHeader } from '@/components/PageHeader'
import { QueryState } from '@/components/QueryState'
import { EmptyState } from '@/components/ui'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'

export function ExerciseDetail() {
  const { id } = useParams<{ id: string }>()
  const exerciseId = Number(id)

  const query = useQuery({
    queryKey: ['exercise', exerciseId],
    queryFn: () => api.getExercise(exerciseId),
    enabled: !isNaN(exerciseId),
  })
  const exercise = query.data

  return (
    <div>
      <PageHeader
        title={exercise?.name ?? 'Exercise'}
        pageTitle={exercise?.name ?? null}
        back={{ to: '/exercises', label: 'Exercises' }}
        meta={exercise?.featured && <Star className="size-4 fill-current text-warning-strong" aria-label="Featured" />}
        actions={exercise && (
          <Button variant="outline" render={<Link to={`/exercises/${exerciseId}/edit`} />}>
            <Pencil aria-hidden="true" /> Edit
          </Button>
        )}
      />

      <QueryState
        query={query}
        resource="exercise"
        skeleton="detail"
        notFound={<EmptyState icon="🔍" title="Exercise not found" action="All exercises" actionTo="/exercises" />}
      >
        {exercise => (
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
            {!exercise.tier && !exercise.rest_seconds && !exercise.form_notes && !exercise.demo_url && (
              <div className="md:col-span-2">
                <EmptyState icon="📝" title="No details yet" description="Add form cues, a demo video or a rest time." action="Edit exercise" actionTo={`/exercises/${exerciseId}/edit`} />
              </div>
            )}
          </div>
        )}
      </QueryState>
    </div>
  )
}
