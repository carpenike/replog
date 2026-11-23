import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { Plus, Star } from 'lucide-react'
import { api } from '@/api/client'
import type { User } from '@/api/types'
import { PageHeader } from '@/components/PageHeader'
import { QueryState } from '@/components/QueryState'
import { CardList, CardListItem, DesktopOnly, RowLink } from '@/components/CardList'
import { EmptyState } from '@/components/ui'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

function Featured({ featured }: { featured: boolean }) {
  if (!featured) return null
  return <Star className="size-4 fill-current text-warning-strong" aria-label="Featured" />
}

export function ExercisesList({ user }: { user: User }) {
  const [search, setSearch] = useState('')
  const canManage = user.is_coach || user.is_admin

  const query = useQuery({
    queryKey: ['exercises'],
    queryFn: () => api.listExercises(),
  })

  return (
    <div>
      <PageHeader
        title="Exercises"
        actions={canManage && (
          <Button render={<Link to="/exercises/new" />}>
            <Plus aria-hidden="true" /> New Exercise
          </Button>
        )}
      />

      <QueryState
        query={query}
        resource="exercises"
        empty={{
          icon: '📋',
          title: 'No exercises yet',
          action: canManage ? 'New Exercise' : undefined,
          actionTo: canManage ? '/exercises/new' : undefined,
        }}
      >
        {exercises => {
          const filtered = exercises.filter(e => e.name.toLowerCase().includes(search.toLowerCase()))
          return (
            <>
              <Input type="search" aria-label="Search exercises" value={search} onChange={e => setSearch(e.target.value)} placeholder="Search exercises..." className="mb-4" />
              {filtered.length === 0 ? (
                <EmptyState icon="🔍" title="No exercises match your search" />
              ) : (
                <>
                  <CardList label="Exercises">
                    {filtered.map(e => (
                      <CardListItem
                        key={e.id}
                        to={`/exercises/${e.id}`}
                        title={e.name}
                        subtitle={<span className="capitalize">{e.tier?.replace('_', ' ') ?? 'Any tier'}</span>}
                        trailing={<Featured featured={e.featured} />}
                      />
                    ))}
                  </CardList>
                  <DesktopOnly>
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>Name</TableHead>
                          <TableHead>Tier</TableHead>
                          <TableHead>Featured</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {filtered.map(exercise => (
                          <TableRow key={exercise.id} className="relative">
                            <TableCell><RowLink to={`/exercises/${exercise.id}`}>{exercise.name}</RowLink></TableCell>
                            <TableCell className="text-muted-foreground capitalize">{exercise.tier?.replace('_', ' ') ?? '—'}</TableCell>
                            <TableCell><Featured featured={exercise.featured} /></TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </DesktopOnly>
                </>
              )}
            </>
          )
        }}
      </QueryState>
    </div>
  )
}
