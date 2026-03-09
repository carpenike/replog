import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'
import { Spinner } from '@/components/ui'

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
  const queryClient = useQueryClient()

  const [noteContent, setNoteContent] = useState('')
  const [isPrivate, setIsPrivate] = useState(false)
  const [showNoteForm, setShowNoteForm] = useState(false)

  const { data: entries, isLoading, error } = useQuery({
    queryKey: ['journal', athleteId],
    queryFn: () => api.listJournalEntries(athleteId),
    enabled: !isNaN(athleteId),
  })

  const createNoteMutation = useMutation({
    mutationFn: () => api.createNote(athleteId, noteContent, isPrivate),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['journal', athleteId] })
      setNoteContent('')
      setIsPrivate(false)
      setShowNoteForm(false)
    },
  })

  if (isLoading) return <Spinner />
  if (error) return <p className="text-destructive">Failed to load journal.</p>

  return (
    <div>
      <p className="text-sm text-muted-foreground mb-1">
        <Link to={`/athletes/${athleteId}`} className="hover:text-foreground">Athlete</Link>
        {' / Journal'}
      </p>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">Journal</h1>
        <button onClick={() => setShowNoteForm(!showNoteForm)}
          className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90">
          {showNoteForm ? 'Cancel' : '+ Add Note'}
        </button>
      </div>

      {/* Add note form */}
      {showNoteForm && (
        <form onSubmit={(e) => { e.preventDefault(); createNoteMutation.mutate() }}
          className="rounded-lg border border-border bg-card p-4 mb-6 space-y-3">
          <textarea value={noteContent} onChange={e => setNoteContent(e.target.value)}
            rows={3} placeholder="Write a note..."
            className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm" />
          <div className="flex items-center justify-between">
            <label className="flex items-center gap-2 text-sm">
              <input type="checkbox" checked={isPrivate} onChange={e => setIsPrivate(e.target.checked)}
                className="rounded border-border" />
              Private (coaches only)
            </label>
            <button type="submit" disabled={createNoteMutation.isPending || !noteContent.trim()}
              className="rounded-md bg-primary px-4 py-1.5 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50">
              {createNoteMutation.isPending ? 'Saving...' : 'Add Note'}
            </button>
          </div>
        </form>
      )}

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
