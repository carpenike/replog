import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { LucideIcon } from 'lucide-react'
import { CheckCircle2, Dumbbell, Flag, ClipboardList, FileText, Lock, NotebookPen, Pencil, Pin, Plus, Scale, Target, Trash2, TrendingUp, X } from 'lucide-react'
import { PageHeader } from '@/components/PageHeader'
import { QueryState } from '@/components/QueryState'
import { useConfirm } from '@/lib/useConfirm'
import { useFriendlyDate } from '@/lib/dates'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
const typeIcons: Record<string, LucideIcon> = {
  workout: Dumbbell,
  body_weight: Scale,
  training_max: TrendingUp,
  goal_change: Target,
  tier_change: TrendingUp,
  program_start: ClipboardList,
  program_end: Flag,
  review: CheckCircle2,
  note: NotebookPen,
}

function EntryIcon({ type }: { type: string }) {
  const Icon = typeIcons[type] ?? FileText
  return <Icon className="mt-0.5 size-5 shrink-0 text-muted-foreground" aria-hidden="true" />
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
  const friendlyDate = useFriendlyDate()
  const { data: athlete } = useQuery({
    queryKey: ['athlete', athleteId],
    queryFn: () => api.getAthlete(athleteId),
    enabled: !isNaN(athleteId),
  })
  const query = useQuery({
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
  return (
    <div>
      <PageHeader
        title="Journal"
        back={{ to: `/athletes/${athleteId}`, label: athlete?.name ?? 'Athlete' }}
        actions={
          <Button variant={showNoteForm ? 'ghost' : 'default'} onClick={() => setShowNoteForm(!showNoteForm)} aria-expanded={showNoteForm}>
            {showNoteForm ? <><X aria-hidden="true" /> Cancel</> : <><Plus aria-hidden="true" /> Add Note</>}
          </Button>
        }
      />
      {/* Add note form */}
      {showNoteForm && (
        <form onSubmit={(e) => { e.preventDefault(); createNoteMutation.mutate() }}
          className="rounded-lg border border-border bg-card p-4 mb-6 space-y-3">
          <Label htmlFor="journal-note" className="sr-only">Note</Label>
          <Textarea id="journal-note" value={noteContent} onChange={e => setNoteContent(e.target.value)}
            placeholder="Write a note..." />
          <div className="flex items-center justify-between">
            <Label className="min-h-11 md:min-h-0">
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
      <QueryState
        query={query}
        resource="journal"
        empty={{ icon: '📓', title: 'No journal entries yet', description: 'Add a note to capture context on this athlete.', action: showNoteForm ? undefined : 'Add Note', onAction: () => setShowNoteForm(true) }}
      >
        {entries => (
        <div className="space-y-2">
          {entries.map((entry, i) => (
            <div key={`${entry.type}-${entry.id}-${i}`}
              className={`rounded-lg border bg-card p-3 ${entry.pinned ? 'border-primary/30' : 'border-border'}`}
            >
              <div className="flex items-start gap-3">
                <EntryIcon type={entry.type} />
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <p className="text-sm font-medium">{entry.summary}</p>
                    {entry.pinned && <Pin className="size-3.5 text-primary" aria-label="Pinned" />}
                    {entry.is_private && <Lock className="size-3.5 text-muted-foreground" aria-label="Private" />}
                  </div>
                  {editingNoteId === entry.id && entry.type === 'note' ? (
                    <div className="mt-2">
                      <Textarea aria-label="Edit note" value={editContent} onChange={e => setEditContent(e.target.value)}
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
                        <span className="text-xs text-muted-foreground">{friendlyDate(entry.date)}</span>
                        {entry.author && (
                          <span className="text-xs text-muted-foreground">by {entry.author}</span>
                        )}
                        {entry.type === 'note' && (
                          <>
                            <Button variant="ghost" size="icon-sm" aria-label="Edit note" onClick={() => { setEditingNoteId(entry.id); setEditContent(entry.detail || entry.summary) }}>
                              <Pencil aria-hidden="true" />
                            </Button>
                            <Button variant="ghost" size="icon-sm" aria-label="Delete note" className="text-muted-foreground hover:text-destructive" onClick={async () => { if (await confirm({ title: 'Delete Note', description: 'Remove this note?', confirmLabel: 'Delete', variant: 'danger' })) deleteNoteMutation.mutate(entry.id) }}>
                              <Trash2 aria-hidden="true" />
                            </Button>
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
      </QueryState>
      {confirmDialog()}
    </div>
  )
}