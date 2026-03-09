import { useState, type ReactNode } from 'react'
import { ConfirmDialog } from '@/components/ConfirmDialog'

export function useConfirm() {
  const [state, setState] = useState<{
    open: boolean
    title: string
    description?: string
    confirmLabel?: string
    variant?: 'danger' | 'default'
    resolve?: (value: boolean) => void
  }>({ open: false, title: '' })

  function confirm(opts: { title: string; description?: string; confirmLabel?: string; variant?: 'danger' | 'default' }): Promise<boolean> {
    return new Promise((resolve) => {
      setState({ ...opts, open: true, resolve })
    })
  }

  function dialog(): ReactNode {
    if (!state.open) return null
    return (
      <ConfirmDialog
        open={state.open}
        title={state.title}
        description={state.description}
        confirmLabel={state.confirmLabel}
        variant={state.variant}
        onConfirm={() => { state.resolve?.(true); setState({ open: false, title: '' }) }}
        onCancel={() => { state.resolve?.(false); setState({ open: false, title: '' }) }}
      />
    )
  }

  return { confirm, dialog }
}
