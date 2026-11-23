import { useState } from 'react'
import { ExternalLink, Info } from 'lucide-react'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle, SheetTrigger } from '@/components/ui/sheet'
import { Button } from '@/components/ui/button'
import { parseDemoVideo } from '@/lib/video'
import type { Exercise } from '@/api/types'

interface ExerciseCuesSheetProps {
  name: string
  exercise?: Exercise
}

/**
 * The ⓘ on a session exercise header: the coach's form cues and the demo
 * video, embedded when it is a YouTube or Vimeo link and linked otherwise.
 */
export function ExerciseCuesSheet({ name, exercise }: ExerciseCuesSheetProps) {
  const [open, setOpen] = useState(false)
  const cues = exercise?.form_notes?.trim()
  const video = parseDemoVideo(exercise?.demo_url)

  return (
    <Sheet open={open} onOpenChange={setOpen}>
      <SheetTrigger
        render={
          <Button
            variant="ghost"
            size="icon-touch"
            className="text-muted-foreground"
            aria-label={`Form cues and demo for ${name}`}
          />
        }
      >
        <Info aria-hidden="true" />
      </SheetTrigger>
      <SheetContent side="bottom" className="max-h-[85dvh] overflow-y-auto pb-[calc(1rem+env(safe-area-inset-bottom))] md:mx-auto md:max-w-2xl md:rounded-t-xl">
        <SheetHeader>
          <SheetTitle>{name}</SheetTitle>
          <SheetDescription>Form cues and demo</SheetDescription>
        </SheetHeader>
        <div className="space-y-4 px-4">
          {/* Only mount the player while the sheet is open so nothing loads in the background. */}
          {open && video && video.kind !== 'link' && (
            <div className="aspect-video overflow-hidden rounded-lg bg-muted">
              <iframe
                src={video.embedUrl}
                title={`${name} demo video`}
                className="size-full"
                allow="encrypted-media; picture-in-picture; fullscreen"
                allowFullScreen
                referrerPolicy="strict-origin-when-cross-origin"
                loading="lazy"
              />
            </div>
          )}
          {cues ? (
            <div>
              <h3 className="mb-1 text-sm font-medium">Cues</h3>
              <p className="whitespace-pre-line text-sm text-muted-foreground">{cues}</p>
            </div>
          ) : (
            !video && <p className="text-sm text-muted-foreground">No cues or demo for this lift yet. Your coach can add them on the exercise.</p>
          )}
          {video && (
            <a
              href={video.url}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex h-11 items-center gap-1.5 text-sm text-primary underline-offset-4 hover:underline"
            >
              <ExternalLink className="size-4" aria-hidden="true" />
              {video.kind === 'link' ? 'Open demo video' : 'Open video in a new tab'}
            </a>
          )}
        </div>
      </SheetContent>
    </Sheet>
  )
}
