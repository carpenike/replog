import { useRef, useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Film, MessageSquare, RefreshCw, Trash2, Upload } from 'lucide-react'
import { api } from '@/api/client'
import type { SetVideo } from '@/api/types'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { checkClip, formatBytes, FORM_CHECK_ACCEPT } from '@/lib/formCheck'
import { useConfirm } from '@/lib/useConfirm'

interface SetVideoProps {
  athleteId: number
  workoutId: number
  setId: number
  setNumber: number
  exerciseName: string
  video?: SetVideo | null
  /** Coaches (and admins) can leave a comment for the review. */
  canComment: boolean
}

function useWorkoutInvalidate(athleteId: number, workoutId: number) {
  const queryClient = useQueryClient()
  return () => {
    queryClient.invalidateQueries({ queryKey: ['workout', athleteId, workoutId] })
    queryClient.invalidateQueries({ queryKey: ['pending-reviews'] })
  }
}

/** Uploads a picked clip after the quick client-side check. */
function useUploadClip(athleteId: number, workoutId: number, setId: number) {
  const invalidate = useWorkoutInvalidate(athleteId, workoutId)
  return useMutation({
    mutationFn: (file: File) => api.uploadSetVideo(athleteId, workoutId, setId, file),
    onMutate: () => ({ toastId: toast.loading('Uploading form check…') }),
    onSuccess: (_v, _f, ctx) => toast.success('Form check attached', { id: ctx?.toastId }),
    onError: (err, _f, ctx) => toast.error(err instanceof Error ? err.message : 'Upload failed', { id: ctx?.toastId }),
    onSettled: invalidate,
  })
}

/** A hidden file input plus the button that opens it. */
function ClipPicker({ onPick, disabled, label, icon, className, variant = 'ghost' }: {
  onPick: (file: File) => void
  disabled: boolean
  label: string
  icon: 'upload' | 'replace'
  className?: string
  variant?: 'ghost' | 'outline'
}) {
  const ref = useRef<HTMLInputElement>(null)
  const Icon = icon === 'upload' ? Upload : RefreshCw
  return (
    <>
      <input
        ref={ref}
        type="file"
        accept={FORM_CHECK_ACCEPT}
        className="sr-only"
        tabIndex={-1}
        aria-hidden="true"
        onChange={e => {
          const file = e.target.files?.[0]
          e.target.value = ''
          if (!file) return
          const problem = checkClip(file)
          if (problem) {
            toast.error(problem)
            return
          }
          onPick(file)
        }}
      />
      <Button variant={variant} size={variant === 'ghost' ? 'icon-touch' : 'touch'} className={className}
        aria-label={variant === 'ghost' ? label : undefined} title={label} disabled={disabled}
        onClick={() => ref.current?.click()}>
        <Icon aria-hidden="true" />{variant !== 'ghost' && label}
      </Button>
    </>
  )
}

function CommentForm({ athleteId, workoutId, setId, video }: { athleteId: number; workoutId: number; setId: number; video: SetVideo }) {
  const [comment, setComment] = useState(() => video.coach_comment ?? '')
  const invalidate = useWorkoutInvalidate(athleteId, workoutId)
  const save = useMutation({
    mutationFn: () => api.commentSetVideo(athleteId, workoutId, setId, comment),
    onSuccess: () => toast.success(comment.trim() ? 'Comment saved' : 'Comment cleared'),
    onError: err => toast.error(err instanceof Error ? err.message : 'Could not save the comment'),
    onSettled: invalidate,
  })
  const id = `video-comment-${setId}`
  return (
    <form className="space-y-2" onSubmit={e => { e.preventDefault(); save.mutate() }}>
      <Label htmlFor={id}>Coach comment</Label>
      <Textarea id={id} rows={3} maxLength={2000} value={comment} onChange={e => setComment(e.target.value)}
        placeholder="What you saw and the cue for next time" />
      <Button type="submit" size="touch" disabled={save.isPending || comment === (video.coach_comment ?? '')}>
        <MessageSquare aria-hidden="true" /> {save.isPending ? 'Saving…' : 'Save comment'}
      </Button>
    </form>
  )
}

/**
 * The form-check control on a logged set: an upload button when the set has
 * no clip, otherwise a button that opens the clip with the coach's comment.
 * The clip streams from the authenticated set route; it is never public.
 */
export function SetVideoControl({ athleteId, workoutId, setId, setNumber, exerciseName, video, canComment }: SetVideoProps) {
  const [open, setOpen] = useState(false)
  const upload = useUploadClip(athleteId, workoutId, setId)
  const invalidate = useWorkoutInvalidate(athleteId, workoutId)
  const { confirm, dialog } = useConfirm()
  const remove = useMutation({
    mutationFn: () => api.deleteSetVideo(athleteId, workoutId, setId),
    onSuccess: () => { toast.success('Form check removed'); setOpen(false) },
    onError: err => toast.error(err instanceof Error ? err.message : 'Could not remove the clip'),
    onSettled: invalidate,
  })

  if (!video) {
    return (
      <ClipPicker
        onPick={f => upload.mutate(f)}
        disabled={upload.isPending}
        label={`Attach form-check video to set ${setNumber}`}
        icon="upload"
        className="text-muted-foreground"
      />
    )
  }

  const comment = video.coach_comment
  return (
    <>
      <Button variant="ghost" size="icon-touch" onClick={() => setOpen(true)}
        aria-label={`Watch form check for set ${setNumber}${comment ? ' (coach commented)' : ''}`}
        className="relative text-primary">
        <Film aria-hidden="true" />
        {comment && <span aria-hidden="true" className="absolute right-1.5 top-1.5 size-2 rounded-full bg-primary" />}
      </Button>
      <Sheet open={open} onOpenChange={setOpen}>
        <SheetContent side="bottom" className="max-h-[92dvh] overflow-y-auto pb-[calc(1rem+env(safe-area-inset-bottom))] md:mx-auto md:max-w-2xl md:rounded-t-xl">
          <SheetHeader>
            <SheetTitle>Form check · {exerciseName}, set {setNumber}</SheetTitle>
            <SheetDescription>
              {formatBytes(video.size_bytes)}{video.uploaded_by ? ` · uploaded by ${video.uploaded_by}` : ''}
            </SheetDescription>
          </SheetHeader>
          <div className="space-y-4 px-4">
            {open && (
              <video controls playsInline preload="metadata" src={video.url}
                className="w-full max-h-[55dvh] rounded-lg bg-black">
                <a href={video.url}>Download the clip</a>
              </video>
            )}
            <p className="text-xs text-muted-foreground">
              Won't play? Some browsers can't play iPhone HEVC clips — <a className="underline" href={video.url} download>download it</a> instead.
            </p>
            {canComment ? (
              <CommentForm key={video.id} athleteId={athleteId} workoutId={workoutId} setId={setId} video={video} />
            ) : comment ? (
              <div className="rounded-lg border border-border bg-muted/40 p-3 text-sm">
                <p className="font-medium">Coach{video.commented_by ? ` ${video.commented_by}` : ''}</p>
                <p className="mt-1 whitespace-pre-wrap">{comment}</p>
              </div>
            ) : (
              <p className="text-sm text-muted-foreground">No coach comment yet.</p>
            )}
            <div className="flex flex-wrap gap-2">
              <ClipPicker onPick={f => upload.mutate(f)} disabled={upload.isPending} label="Replace clip" icon="replace" variant="outline" />
              <Button variant="ghost" size="touch" className="text-muted-foreground hover:text-destructive" disabled={remove.isPending}
                onClick={async () => {
                  if (await confirm({ title: 'Remove form check', description: 'Delete this clip and its coach comment? The set stays logged.', confirmLabel: 'Remove', variant: 'danger' }))
                    remove.mutate()
                }}>
                <Trash2 aria-hidden="true" /> Remove clip
              </Button>
            </div>
          </div>
        </SheetContent>
      </Sheet>
      {dialog()}
    </>
  )
}
