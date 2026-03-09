import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'

export function ExportPage() {
  const { id } = useParams<{ id: string }>()
  const athleteId = Number(id)

  const { data: athlete } = useQuery({
    queryKey: ['athlete', athleteId],
    queryFn: () => api.getAthlete(athleteId),
    enabled: !isNaN(athleteId),
  })

  function downloadFile(url: string, filename: string) {
    const a = document.createElement('a')
    a.href = url
    a.download = filename
    a.click()
  }

  return (
    <div>
      <p className="text-sm text-muted-foreground mb-1">
        <Link to={`/athletes/${athleteId}`} className="hover:text-foreground">{athlete?.name ?? 'Athlete'}</Link>
        {' / Export'}
      </p>
      <h1 className="text-2xl font-bold mb-6">Export Data</h1>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div className="rounded-lg border border-border bg-card p-6">
          <h2 className="font-semibold mb-2">RepLog JSON</h2>
          <p className="text-sm text-muted-foreground mb-4">
            Full export of workouts, training maxes, body weights, and programs in RepLog format.
            Can be re-imported into another RepLog instance.
          </p>
          <button
            onClick={() => downloadFile(`/athletes/${athleteId}/export/json`, `athlete-${athleteId}-export.json`)}
            className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90">
            Download JSON
          </button>
        </div>

        <div className="rounded-lg border border-border bg-card p-6">
          <h2 className="font-semibold mb-2">CSV Export</h2>
          <p className="text-sm text-muted-foreground mb-4">
            Workout data in CSV format compatible with Strong and other workout apps.
          </p>
          <button
            onClick={() => downloadFile(`/athletes/${athleteId}/export/csv`, `athlete-${athleteId}-export.csv`)}
            className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90">
            Download CSV
          </button>
        </div>
      </div>
    </div>
  )
}
