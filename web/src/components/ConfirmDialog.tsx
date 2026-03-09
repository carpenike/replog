import { useEffect, useRef } from 'react'

interface ConfirmDialogProps {
  open: boolean
  title: string
  description?: string
  confirmLabel?: string
  variant?: 'danger' | 'default'
  onConfirm: () => void
  onCancel: () => void
}

export function ConfirmDialog({ open, title, description, confirmLabel = 'Confirm', variant = 'default', onConfirm, onCancel }: ConfirmDialogProps) {
  const cancelRef = useRef<HTMLButtonElement>(null)

  useEffect(() => {
    if (!open) return
    cancelRef.current?.focus()
    function handleKey(e: KeyboardEvent) {
      if (e.key === 'Escape') onCancel()
    }
    document.addEventListener('keydown', handleKey)
    return () => document.removeEventListener('keydown', handleKey)
  }, [open, onCancel])

  if (!open) return null

  return (
    <div className="fixed inset-0 z-[100] flex items-center justify-center">
      <div className="fixed inset-0 bg-black/50" onClick={onCancel} />
      <div role="dialog" aria-modal="true" aria-labelledby="confirm-title"
        className="relative bg-card border border-border rounded-lg p-6 max-w-sm w-full mx-4 shadow-lg">
        <h3 id="confirm-title" className="text-lg font-semibold mb-1">{title}</h3>
        {description && (
          <p className="text-sm text-muted-foreground mb-4">{description}</p>
        )}
        <div className="flex justify-end gap-2">
          <button ref={cancelRef} onClick={onCancel}
            className="rounded-md border border-border px-4 py-2 text-sm hover:bg-accent transition-colors">
            Cancel
          </button>
          <button onClick={onConfirm}
            className={`rounded-md px-4 py-2 text-sm font-medium transition-colors ${
              variant === 'danger'
                ? 'bg-destructive text-white hover:bg-destructive/90'
                : 'bg-primary text-primary-foreground hover:bg-primary/90'
            }`}>
            {confirmLabel}
          </button>
        </div>
      </div>
    </div>
  )
}
