import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { Lock, Plus } from 'lucide-react'
import { api } from '@/api/client'
import type { ProgramTemplate, User } from '@/api/types'
import { PageHeader } from '@/components/PageHeader'
import { QueryState } from '@/components/QueryState'
import { CardList, CardListItem, DesktopOnly, RowLink } from '@/components/CardList'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

function ProgramBadges({ program }: { program: ProgramTemplate }) {
  return (
    <span className="inline-flex flex-wrap gap-1">
      {program.is_loop && <Badge variant="secondary">Loop</Badge>}
      {program.locked && (
        <Badge variant="outline"><Lock aria-hidden="true" className="size-3" /> Locked · v{program.import_version ?? program.version}</Badge>
      )}
    </span>
  )
}

export function ProgramsList({ user }: { user: User }) {
  const canManage = user.is_coach || user.is_admin
  const query = useQuery({
    queryKey: ['programs'],
    queryFn: () => api.listProgramTemplates(),
  })

  return (
    <div>
      <PageHeader
        title="Programs"
        actions={canManage && (
          <Button render={<Link to="/programs/new" />}>
            <Plus aria-hidden="true" /> New Program
          </Button>
        )}
      />

      <QueryState
        query={query}
        resource="programs"
        empty={{
          icon: '📊',
          title: 'No program templates yet',
          description: canManage ? 'Build a template, import one, or draft one with the generator.' : undefined,
          action: canManage ? 'New Program' : undefined,
          actionTo: canManage ? '/programs/new' : undefined,
        }}
      >
        {programs => (
          <>
            <CardList label="Programs">
              {programs.map(p => (
                <CardListItem
                  key={p.id}
                  to={`/programs/${p.id}`}
                  title={p.name}
                  subtitle={`${p.num_weeks}w / ${p.num_days}d${(p.athlete_count ?? 0) > 0 ? ` · ${p.athlete_count} athlete${p.athlete_count === 1 ? '' : 's'}` : ''}`}
                  trailing={<ProgramBadges program={p} />}
                >
                  {p.description && <p className="text-xs text-muted-foreground line-clamp-2">{p.description}</p>}
                </CardListItem>
              ))}
            </CardList>
            <DesktopOnly>
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
                  {programs.map(program => (
                    <TableRow key={program.id} className="relative">
                      <TableCell className="whitespace-normal max-w-xs">
                        <RowLink to={`/programs/${program.id}`}>{program.name}</RowLink>
                        {program.description && (
                          <p className="text-xs text-muted-foreground line-clamp-2">{program.description}</p>
                        )}
                      </TableCell>
                      <TableCell className="text-muted-foreground">{program.num_weeks}w / {program.num_days}d</TableCell>
                      <TableCell>{program.is_loop || program.locked ? <ProgramBadges program={program} /> : '—'}</TableCell>
                      <TableCell>{(program.athlete_count ?? 0) > 0 ? program.athlete_count : '—'}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </DesktopOnly>
          </>
        )}
      </QueryState>
    </div>
  )
}
