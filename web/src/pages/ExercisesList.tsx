import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'

export function ExercisesList() {
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
              <tr key={exercise.id} className="border-b border-border last:border-0 hover:bg-muted/30 transition-colors">
                <td className="p-3 text-sm font-medium">{exercise.name}</td>
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
