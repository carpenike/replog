import { Toaster } from 'sonner'
import { toasterPlacement } from '@/lib/chrome'
import { DESKTOP_QUERY, useMediaQuery } from '@/lib/useMediaQuery'

/** App-wide toaster, placed per toasterPlacement for the current viewport. */
export function AppToaster() {
  const isDesktop = useMediaQuery(DESKTOP_QUERY)
  return (
    <Toaster
      {...toasterPlacement(isDesktop)}
      toastOptions={{
        className: 'bg-card text-foreground border-border',
      }}
    />
  )
}
