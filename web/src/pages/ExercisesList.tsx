import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link, useNavigate } from 'react-router-dom'
import { api } from '@/api/client'
import { Spinner } from '@/components/ui'
import { usePageTitle } from '@/lib/usePageTitle'
import type { User } from '@/api/types'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

export function ExercisesList({ user }: { user: User }) {
  usePageTitle('Exercises')
  const navigate = useNavigate()
  const [search, setSearch] = useState('')

  const { data: exercises, isLoading, error } = useQuery({
    queryKey: ['exercises'],
    queryFn: () => api.listExercises(),
  })

  if (isLoading) return <Spinner />
  if (error) return <p className="text-destructive">Failed to load exercises.</p>

  const filtered = exercises?.filter(e =>
    e.name.toLowerCase().includes(search.toLowerCase())
  ) ?? []

  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <h1 className="text-2xl font-bold">Exercises</h1>
        {(user.is_coach || user.is_admin) && (
          <Button onClick={() => navigate('/exercises/new')}>
            + New Exercise
          </Button>
        )}
      </div>

      <Input type="text" value={search} onChange={e => setSearch(e.target.value)} placeholder="Search exercises..." className="mb-4" />

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
            <TableRow key={exercise.id} className="cursor-pointer"
              onClick={() => navigate(`/exercises/${exercise.id}`)}>
              <TableCell className="font-medium">
                <Link to={`/exercises/${exercise.id}`} className="hover:text-primary">{exercise.name}</Link>
              </TableCell>
              <TableCell className="text-muted-foreground capitalize">{exercise.tier?.replace('_', ' ') ?? '—'}</TableCell>
              <TableCell>{exercise.featured ? '⭐' : ''}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}
