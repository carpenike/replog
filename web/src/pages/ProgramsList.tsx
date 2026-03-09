import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { api } from '@/api/client'
import { Spinner } from '@/components/ui'

export function ProgramsList() {
  const { data: programs, isLoading, error } = useQuery({
    queryKey: ['programs'],
    queryFn: () => api.listProgramTemplates(),
  })

  if (isLoading) return <Spinner />
  if (error) return <p className="text-destructive">Failed to load programs.</p>

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">Programs</h1>
      </div>

      {programs && programs.length === 0 ? (
        <p className="text-muted-foreground">No program templates found.</p>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {programs?.map(program => (
            <Link
              key={program.id}
              to={`/programs/${program.id}`}
              className="rounded-lg border border-border bg-card p-4 hover:border-primary/50 transition-colors"
            >
              <h3 className="font-semibold">{program.name}</h3>
              {program.description && (
                <p className="text-sm text-muted-foreground mt-1 line-clamp-2">{program.description}</p>
              )}
              <div className="flex gap-3 mt-3 text-xs text-muted-foreground">
                <span>{program.num_weeks}w / {program.num_days}d</span>
                {program.is_loop && <span className="text-primary">Loop</span>}
                {(program.athlete_count ?? 0) > 0 && (
                  <span>{program.athlete_count} athlete{program.athlete_count !== 1 ? 's' : ''}</span>
                )}
              </div>
            </Link>
          ))}
        </div>
      )}
    </div>
  )
}
