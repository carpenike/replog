import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Ban, Check, Copy, Link2 } from 'lucide-react'
import { api } from '@/api/client'
import type { CreatedVideoShare, VideoShare } from '@/api/types'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Checkbox } from '@/components/ui/checkbox'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Field } from '@/components/Field'
import { useConfirm } from '@/lib/useConfirm'
import {
  DEFAULT_SHARE_EXPIRY_HOURS,
  SHARE_EXPIRY_OPTIONS,
  SHARE_LABEL_MAX,
  absoluteShareURL,
  shareStatusText,
  shareViewsText,
} from '@/lib/videoShare'

interface ClipRef {
  athleteId: number
  workoutId: number
  setId: number
}

interface VideoShareSheetProps extends ClipRef {
  open: boolean
  onOpenChange: (open: boolean) => void
  setNumber: number
  exerciseName: string
}

const sharesKey = ({ athleteId, workoutId, setId }: ClipRef) => ['video-shares', athleteId, workoutId, setId] as const

function errorText(err: unknown, fallback: string): string {
  return err instanceof Error && err.message ? err.message : fallback
}

/** Expiry choice, optional label and the Create button. */
function CreateShareForm({ clip, disabled, onCreated }: { clip: ClipRef; disabled: boolean; onCreated: (c: CreatedVideoShare) => void }) {
  const [hours, setHours] = useState(DEFAULT_SHARE_EXPIRY_HOURS)
  const [label, setLabel] = useState('')
  const queryClient = useQueryClient()
  const create = useMutation({
    mutationFn: () => api.createVideoShare(clip.athleteId, clip.workoutId, clip.setId, {
      expires_in_hours: hours,
      ...(label.trim() ? { label: label.trim() } : {}),
    }),
    meta: { skipGlobalError: true },
    onSuccess: created => {
      setLabel('')
      onCreated(created)
      toast.success('Share link created')
    },
    onError: err => toast.error(errorText(err, 'Could not create the link')),
    onSettled: () => queryClient.invalidateQueries({ queryKey: sharesKey(clip) }),
  })
  const groupId = `share-expiry-${clip.setId}`
  return (
    <form className="space-y-4" onSubmit={e => { e.preventDefault(); create.mutate() }}>
      <div role="group" aria-labelledby={groupId} className="space-y-1.5">
        <p id={groupId} className="text-sm font-medium">Link expires after</p>
        <RadioGroup
          aria-labelledby={groupId}
          value={hours}
          onValueChange={(v: number | null) => { if (v) setHours(v) }}
          className="grid grid-cols-3 gap-2"
          disabled={disabled}
        >
          {SHARE_EXPIRY_OPTIONS.map(o => (
            <Label key={o.hours}
              className="flex min-h-11 cursor-pointer items-center justify-center gap-2 rounded-lg border border-input px-2 font-normal has-data-checked:border-primary has-data-checked:bg-primary/5">
              <RadioGroupItem value={o.hours} aria-label={o.label} />
              <span aria-hidden="true">{o.label}</span>
            </Label>
          ))}
        </RadioGroup>
      </div>
      <Field label="Label (optional)" hint="Only you see this, for example who you sent it to.">
        {control => (
          <Input {...control} value={label} maxLength={SHARE_LABEL_MAX} disabled={disabled}
            onChange={e => setLabel(e.target.value)} placeholder="Grandpa" autoComplete="off" />
        )}
      </Field>
      <Button type="submit" size="touch" disabled={disabled || create.isPending}>
        <Link2 aria-hidden="true" /> {create.isPending ? 'Creating…' : 'Create link'}
      </Button>
    </form>
  )
}

/** The new link, shown once, with a Copy button. */
function CreatedShareLink({ created }: { created: CreatedVideoShare }) {
  const [copied, setCopied] = useState(false)
  const url = absoluteShareURL(created.url, window.location.origin)
  const inputId = `share-link-${created.share.id}`
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(url)
      setCopied(true)
      toast.success('Link copied')
    } catch {
      const input = document.getElementById(inputId) as HTMLInputElement | null
      input?.select()
      toast.error('Could not copy. Select the link and copy it.')
    }
  }
  return (
    <div className="space-y-2 rounded-lg border border-primary/40 bg-primary/5 p-3" aria-live="polite">
      <Label htmlFor={inputId}>Your new link</Label>
      <div className="flex gap-2">
        <Input id={inputId} readOnly value={url} className="font-mono text-xs md:text-xs" onFocus={e => e.currentTarget.select()} />
        <Button type="button" size="touch" variant={copied ? 'outline' : 'default'} onClick={copy} aria-label="Copy share link">
          {copied ? <Check aria-hidden="true" /> : <Copy aria-hidden="true" />} {copied ? 'Copied' : 'Copy'}
        </Button>
      </div>
      <p className="text-xs text-muted-foreground">
        Copy it now. This link won't be shown again. Anyone who has it can watch this clip until it expires or you revoke it.
      </p>
    </div>
  )
}

/** One link in the owner's list, with Revoke for an active link. */
function ShareRow({ share, onRevoke, revoking }: { share: VideoShare; onRevoke: (s: VideoShare) => void; revoking: boolean }) {
  const name = share.label || 'Unlabeled link'
  const active = share.status === 'active'
  return (
    <li className="flex items-center gap-3 py-2">
      <div className="min-w-0 flex-1">
        <p className={active ? 'truncate font-medium' : 'truncate font-medium text-muted-foreground'}>{name}</p>
        <p className="text-xs text-muted-foreground">
          {shareStatusText(share)} · {shareViewsText(share.access_count)}
          {share.created_by ? ` · by ${share.created_by}` : ''}
        </p>
      </div>
      {active && (
        <Button type="button" variant="ghost" size="touch" className="text-muted-foreground hover:text-destructive"
          aria-label={`Revoke ${name}`} disabled={revoking} onClick={() => onRevoke(share)}>
          <Ban aria-hidden="true" /> Revoke
        </Button>
      )}
    </li>
  )
}

/** The links on the clip, active by default, with Revoke (confirmed). */
function ShareList({ clip, open }: { clip: ClipRef; open: boolean }) {
  const [showInactive, setShowInactive] = useState(false)
  const queryClient = useQueryClient()
  const { confirm, dialog } = useConfirm()
  const list = useQuery({
    queryKey: [...sharesKey(clip), showInactive],
    queryFn: () => api.listVideoShares(clip.athleteId, clip.workoutId, clip.setId, showInactive),
    enabled: open,
    staleTime: 0,
  })
  const revoke = useMutation({
    mutationFn: (share: VideoShare) => api.revokeVideoShare(clip.athleteId, clip.workoutId, clip.setId, share.id),
    meta: { skipGlobalError: true },
    onSuccess: () => toast.success('Link revoked'),
    onError: err => toast.error(errorText(err, 'Could not revoke the link')),
    onSettled: () => queryClient.invalidateQueries({ queryKey: sharesKey(clip) }),
  })
  const onRevoke = async (share: VideoShare) => {
    const ok = await confirm({
      title: 'Revoke link',
      description: `Revoke "${share.label || 'Unlabeled link'}"? It stops working right away for anyone who has it.`,
      confirmLabel: 'Revoke',
      variant: 'danger',
    })
    if (ok) revoke.mutate(share)
  }
  const checkboxId = `share-inactive-${clip.setId}`
  const shares = list.data?.shares ?? []
  return (
    <section className="space-y-2" aria-labelledby={`share-list-${clip.setId}`}>
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h3 id={`share-list-${clip.setId}`} className="text-sm font-medium">{showInactive ? 'All links' : 'Active links'}</h3>
        <Label htmlFor={checkboxId} className="flex min-h-11 items-center gap-2 font-normal text-muted-foreground">
          <Checkbox id={checkboxId} checked={showInactive} onCheckedChange={v => setShowInactive(v === true)} />
          Show expired and revoked
        </Label>
      </div>
      {list.isLoading ? (
        <p className="text-sm text-muted-foreground">Loading links…</p>
      ) : list.error ? (
        <p className="text-sm text-destructive">{errorText(list.error, 'Could not load links')}</p>
      ) : shares.length === 0 ? (
        <p className="text-sm text-muted-foreground">{showInactive ? 'No links yet.' : 'No active links.'}</p>
      ) : (
        <ul className="divide-y divide-border">
          {shares.map(s => <ShareRow key={s.id} share={s} onRevoke={onRevoke} revoking={revoke.isPending} />)}
        </ul>
      )}
      {dialog()}
    </section>
  )
}

/** Whether sharing is on, from the list query (true until it loads). */
function useSharingEnabled(clip: ClipRef, open: boolean): boolean {
  const { data } = useQuery({
    queryKey: [...sharesKey(clip), false],
    queryFn: () => api.listVideoShares(clip.athleteId, clip.workoutId, clip.setId, false),
    enabled: open,
    staleTime: 0,
  })
  return data?.sharing_enabled ?? true
}

/** The sheet's contents. Mounted only while the sheet is open, so the new
 * link's token (held only in this state) is dropped when it closes. */
function ShareSheetBody({ clip }: { clip: ClipRef }) {
  const [created, setCreated] = useState<CreatedVideoShare | null>(null)
  const enabled = useSharingEnabled(clip, true)
  return (
    <div className="space-y-5 px-4">
      {!enabled && (
        <p role="status" className="rounded-lg border border-border bg-muted/40 p-3 text-sm">
          Sharing is turned off on this server. Existing links don't open; you can still revoke them.
        </p>
      )}
      {created ? (
        <div className="space-y-3">
          <CreatedShareLink created={created} />
          <Button type="button" variant="outline" size="touch" onClick={() => setCreated(null)}>
            <Link2 aria-hidden="true" /> Create another link
          </Button>
        </div>
      ) : (
        <CreateShareForm clip={clip} disabled={!enabled} onCreated={setCreated} />
      )}
      <ShareList clip={clip} open />
    </div>
  )
}

/**
 * The Share sheet for a form-check clip: create a no-login link that opens
 * just this clip until it expires (shown once, with Copy), and list or
 * revoke the clip's links.
 */
export function VideoShareSheet({ open, onOpenChange, athleteId, workoutId, setId, setNumber, exerciseName }: VideoShareSheetProps) {
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="bottom" className="max-h-[92dvh] overflow-y-auto pb-[calc(1rem+env(safe-area-inset-bottom))] md:mx-auto md:max-w-2xl md:rounded-t-xl">
        <SheetHeader>
          <SheetTitle>Share form check · {exerciseName}, set {setNumber}</SheetTitle>
          <SheetDescription>
            Anyone with the link can watch this one clip without logging in, until it expires. They see the exercise and the date, nothing else.
          </SheetDescription>
        </SheetHeader>
        {open && <ShareSheetBody clip={{ athleteId, workoutId, setId }} />}
      </SheetContent>
    </Sheet>
  )
}
