import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider, MutationCache } from '@tanstack/react-query'
import { BrowserRouter } from 'react-router-dom'
import { Toaster, toast } from 'sonner'
import { ErrorBoundary } from './components/ErrorBoundary'
import { App } from './App'
import './index.css'
import './pwa'

// Allow mutations to opt out of the global error toast when they render their
// own inline error UI (e.g. WodPage's 409 replace/cancel flow).
declare module '@tanstack/react-query' {
  interface Register {
    mutationMeta: {
      skipGlobalError?: boolean
    }
  }
}

// Global mutation error handling: every failed mutation surfaces a toast so
// nothing fails silently in the core logging loop. Per-mutation onError still
// runs and can layer richer handling on top of this.
const mutationCache = new MutationCache({
  onError: (error, _vars, _ctx, mutation) => {
    if (mutation.meta?.skipGlobalError) return
    const message = error instanceof Error ? error.message : 'Something went wrong.'
    toast.error(message)
  },
})

const queryClient = new QueryClient({
  mutationCache,
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      retry: 1,
    },
  },
})

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ErrorBoundary>
      <QueryClientProvider client={queryClient}>
        <BrowserRouter>
          <App />
          <Toaster
            position="bottom-right"
            toastOptions={{
              className: 'bg-card text-foreground border-border',
            }}
          />
        </BrowserRouter>
      </QueryClientProvider>
    </ErrorBoundary>
  </StrictMode>,
)
