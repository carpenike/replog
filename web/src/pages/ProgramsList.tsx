import { useQuery } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { api } from '@/api/client'
import { Spinner } from '@/components/ui'
import { usePageTitle } from '@/lib/usePageTitle'
import type { User } from '@/api/types'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

export function ProgramsList({ user }: { user: User }) {
  usePageTitle('Programs')
  const navigate = useNavigate()

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
        {(user.is_coach || user.is_admin) && (
          <Button onClick={() => navigate('/programs/new')}>
            + New Program
          </Button>
        )}
      </div>

      {programs && programs.length === 0 ? (
        <p className="text-muted-foreground">No program templates found.</p>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Schedule</TableHead>
              <TableHead>Loop</TableHead>
              <TableHead>Athletes</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {programs?.map(program => (
              <TableRow key={program.id} className="cursor-pointer" onClick={() => navigate(`/programs/${program.id}`)}>
                <TableCell className="whitespace-normal max-w-xs">
                  <div>
                    <p className="font-medium">{program.name}</p>
                    {program.description && (
                      <p className="text-xs text-muted-foreground line-clamp-2">{program.description}</p>
                    )}
                  </div>
                </TableCell>
                <TableCell className="text-muted-foreground">{program.num_weeks}w / {program.num_days}d</TableCell>
                <TableCell>{program.is_loop ? <Badge variant="secondary">Loop</Badge> : '—'}</TableCell>
                <TableCell>{(program.athlete_count ?? 0) > 0 ? program.athlete_count : '—'}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </div>
  )
}