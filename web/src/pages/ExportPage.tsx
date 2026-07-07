import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation } from '@tanstack/react-query'
import { toast } from 'sonner'
import { api } from '@/api/client'
import { usePageTitle } from '@/lib/usePageTitle'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'

export function ExportPage() {
  const { id } = useParams<{ id: string }>()
  const athleteId = Number(id)
  usePageTitle('Export Data')

  const { data: athlete } = useQuery({
    queryKey: ['athlete', athleteId],
    queryFn: () => api.getAthlete(athleteId),
    enabled: !isNaN(athleteId),
  })

  const exportMutation = useMutation({
    mutationFn: (format: 'json' | 'csv') => api.exportAthlete(athleteId, format),
    // Global toast handles failures; onSuccess triggers the file download.
    onSuccess: (blob, format) => {
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `athlete-${athleteId}-export.${format}`
      document.body.appendChild(a)
      a.click()
      a.remove()
      URL.revokeObjectURL(url)
      toast.success(`${format.toUpperCase()} export ready`)
    },
  })

  const pendingFormat = exportMutation.isPending ? exportMutation.variables : null

  return (
    <div>
      <p className="text-sm text-muted-foreground mb-1">
        <Link to={`/athletes/${athleteId}`} className="hover:text-foreground">{athlete?.name ?? 'Athlete'}</Link>
        {' / Export'}
      </p>
      <h1 className="text-2xl font-bold mb-6">Export Data</h1>
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <Card>
          <CardContent>
          <h2 className="font-semibold mb-2">RepLog JSON</h2>
          <p className="text-sm text-muted-foreground mb-4">
            Full export of workouts, training maxes, body weights, and programs in RepLog format.
            Can be re-imported into another RepLog instance.
          </p>
          <Button onClick={() => exportMutation.mutate('json')} disabled={exportMutation.isPending}>
            {pendingFormat === 'json' ? 'Preparing…' : 'Download JSON'}
          </Button>
          </CardContent>
        </Card>
        <Card>
          <CardContent>
          <h2 className="font-semibold mb-2">CSV Export</h2>
          <p className="text-sm text-muted-foreground mb-4">
            Workout data in CSV format compatible with Strong and other workout apps.
          </p>
          <Button onClick={() => exportMutation.mutate('csv')} disabled={exportMutation.isPending}>
            {pendingFormat === 'csv' ? 'Preparing…' : 'Download CSV'}
          </Button>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
