import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'

const typeIcons: Record<string, string> = {
  workout: '🏋️',
  body_weight: '⚖️',
  training_max: '💪',
  goal_change: '🎯',
  tier_change: '📈',
  program_start: '📋',
  review: '✅',
  note: '📝',
}

export function JournalPage() {
  const { id } = useParams<{ id: string }>()
  const athleteId = Number(id)

  const { data: entries, isLoading, error } = useQuery({
    queryKey: ['journal', athleteId],
    queryFn: () => api.listJournalEntries(athleteId),
    enabled: !isNaN(athleteId),
  })

  if (isLoading) return <p className="text-muted-foreground">Loading journal...</p>
  if (error) return <p className="text-destructive">Failed to load journal.</p>

  return (
    <div>
      <p className="text-sm text-muted-foreground mb-1">
        <Link to={`/athletes/${athleteId}`} className="hover:text-foreground">Athlete</Link>
        {' / Journal'}
      </p>
      <h1 className="text-2xl font-bold mb-6">Journal</h1>

      {entries && entries.length === 0 ? (
        <p className="text-muted-foreground">No journal entries yet.</p>
      ) : (
        <div className="space-y-2">
          {entries?.map((entry, i) => (
            <div key={`${entry.type}-${entry.id}-${i}`}
              className={`rounded-lg border bg-card p-3 ${entry.pinned ? 'border-primary/30' : 'border-border'}`}
            >
              <div className="flex items-start gap-3">
                <span className="text-lg">{typeIcons[entry.type] ?? '📄'}</span>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <p className="text-sm font-medium">{entry.summary}</p>
                    {entry.pinned && <span className="text-xs text-primary">📌</span>}
                    {entry.is_private && <span className="text-xs text-muted-foreground">🔒</span>}
                  </div>
                  {entry.detail && (
                    <p className="text-sm text-muted-foreground mt-0.5">{entry.detail}</p>
                  )}
                  <div className="flex items-center gap-2 mt-1">
                    <span className="text-xs text-muted-foreground">{entry.date}</span>
                    {entry.author && (
                      <span className="text-xs text-muted-foreground">by {entry.author}</span>
                    )}
                  </div>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
