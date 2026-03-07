import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { api } from '@/api/client'
import type { User } from '@/api/types'

export function ExercisesList({ user }: { user: User }) {
  const { data: exercises, isLoading, error } = useQuery({
    queryKey: ['exercises'],
    queryFn: () => api.listExercises(),
  })

  if (isLoading) return <p className="text-muted-foreground">Loading exercises...</p>
  if (error) return <p className="text-destructive">Failed to load exercises.</p>

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">Exercises</h1>
        {(user.is_coach || user.is_admin) && (
          <Link to="/exercises/new" className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90">
            + New Exercise
          </Link>
        )}
      </div>

      <div className="rounded-lg border border-border overflow-hidden">
        <table className="w-full">
          <thead>
            <tr className="border-b border-border bg-muted/50">
              <th className="text-left p-3 text-sm font-medium text-muted-foreground">Name</th>
              <th className="text-left p-3 text-sm font-medium text-muted-foreground">Tier</th>
              <th className="text-left p-3 text-sm font-medium text-muted-foreground">Featured</th>
            </tr>
          </thead>
          <tbody>
            {exercises?.map(exercise => (
              <tr key={exercise.id} className="border-b border-border last:border-0 hover:bg-muted/30 transition-colors cursor-pointer"
                onClick={() => window.location.href = `/exercises/${exercise.id}`}>
                <td className="p-3 text-sm font-medium">
                  <Link to={`/exercises/${exercise.id}`} className="hover:text-primary">{exercise.name}</Link>
                </td>
                <td className="p-3 text-sm text-muted-foreground capitalize">{exercise.tier?.replace('_', ' ') ?? '—'}</td>
                <td className="p-3 text-sm">{exercise.featured ? '⭐' : ''}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
