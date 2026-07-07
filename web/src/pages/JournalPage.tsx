import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'
import { Spinner, EmptyState, QueryError } from '@/components/ui'
import { useConfirm } from '@/lib/useConfirm'
import { usePageTitle } from '@/lib/usePageTitle'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { formatDate } from '@/lib/utils'
const typeIcons: Record<string, string> = {
  workout: '🏋️',
  body_weight: '⚖️',
  training_max: '💪',
  goal_change: '🎯',
  tier_change: '📈',
  program_start: '📋',
  program_end: '🏁',
  review: '✅',
  note: '📝',
}
export function JournalPage() {
  const { id } = useParams<{ id: string }>()
  const athleteId = Number(id)
  const { confirm, dialog: confirmDialog } = useConfirm()
  const queryClient = useQueryClient()
  const [noteContent, setNoteContent] = useState('')
  const [isPrivate, setIsPrivate] = useState(false)
  const [showNoteForm, setShowNoteForm] = useState(false)
  const [editingNoteId, setEditingNoteId] = useState<number | null>(null)
  const [editContent, setEditContent] = useState('')
  usePageTitle('Journal')
  const { data: athlete } = useQuery({
    queryKey: ['athlete', athleteId],
    queryFn: () => api.getAthlete(athleteId),
    enabled: !isNaN(athleteId),
  })
  const { data: entries, isLoading, error, refetch } = useQuery({
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
  const updateNoteMutation = useMutation({
    mutationFn: (noteId: number) => api.updateNote(athleteId, noteId, editContent),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['journal', athleteId] })
      setEditingNoteId(null)
    },
  })
  const deleteNoteMutation = useMutation({
    mutationFn: (noteId: number) => api.deleteNote(athleteId, noteId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['journal', athleteId] }),
  })
  if (isLoading) return <Spinner />
  if (error) return <QueryError error={error} onRetry={refetch} resource="journal" />
  return (
    <div>
      <p className="text-sm text-muted-foreground mb-1">
        <Link to={`/athletes/${athleteId}`} className="hover:text-foreground">{athlete?.name ?? 'Athlete'}</Link>
        {' / Journal'}
      </p>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">Journal</h1>
        <Button variant="ghost" onClick={() => setShowNoteForm(!showNoteForm)}
          >
          {showNoteForm ? 'Cancel' : '+ Add Note'}
        </Button>
      </div>
      {/* Add note form */}
      {showNoteForm && (
        <form onSubmit={(e) => { e.preventDefault(); createNoteMutation.mutate() }}
          className="rounded-lg border border-border bg-card p-4 mb-6 space-y-3">
          <Textarea value={noteContent} onChange={e => setNoteContent(e.target.value)}
            placeholder="Write a note..." />
          <div className="flex items-center justify-between">
            <Label>
              <Checkbox checked={isPrivate} onCheckedChange={(checked) => setIsPrivate(checked)} />
              Private (coaches only)
            </Label>
            <Button type="submit" disabled={createNoteMutation.isPending || !noteContent.trim()}
              >
              {createNoteMutation.isPending ? 'Saving...' : 'Add Note'}
            </Button>
          </div>
        </form>
      )}
      {entries && entries.length === 0 ? (
        <EmptyState icon="📓" title="No journal entries yet" description="Add a note to capture context on this athlete." />
      ) : (
        <div className="space-y-2">
          {entries?.map((entry, i) => (
            <div key={`${entry.type}-${entry.id}-${i}`}
              className={`rounded-lg border bg-card p-3 ${entry.pinned ? 'border-primary/30' : 'border-border'}`}
            >
              <div className="flex items-start gap-3">
                <span className="text-lg" aria-hidden="true">{typeIcons[entry.type] ?? '📄'}</span>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <p className="text-sm font-medium">{entry.summary}</p>
                    {entry.pinned && <span className="text-xs text-primary">📌</span>}
                    {entry.is_private && <span className="text-xs text-muted-foreground">🔒</span>}
                  </div>
                  {editingNoteId === entry.id && entry.type === 'note' ? (
                    <div className="mt-2">
                      <Textarea value={editContent} onChange={e => setEditContent(e.target.value)}
                        rows={2} />
                      <div className="flex gap-2">
                        <Button variant="ghost" onClick={() => updateNoteMutation.mutate(entry.id)}
                          disabled={updateNoteMutation.isPending}
                          >Save</Button>
                        <Button variant="ghost" onClick={() => setEditingNoteId(null)}
                          className="text-xs text-muted-foreground">Cancel</Button>
                      </div>
                    </div>
                  ) : (
                    <>
                      {entry.detail && (
                        <p className="text-sm text-muted-foreground mt-0.5">{entry.detail}</p>
                      )}
                      <div className="flex items-center gap-2 mt-1">
                        <span className="text-xs text-muted-foreground">{formatDate(entry.date)}</span>
                        {entry.author && (
                          <span className="text-xs text-muted-foreground">by {entry.author}</span>
                        )}
                        {entry.type === 'note' && (
                          <>
                            <Button variant="ghost" onClick={() => { setEditingNoteId(entry.id); setEditContent(entry.detail || entry.summary) }}
                              >Edit</Button>
                            <Button variant="ghost" onClick={async () => { if (await confirm({ title: 'Delete Note', description: 'Remove this note?', confirmLabel: 'Delete', variant: 'danger' })) deleteNoteMutation.mutate(entry.id) }}
                              >Delete</Button>
                          </>
                        )}
                      </div>
                    </>
                  )}
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
      {confirmDialog()}
    </div>
  )
}