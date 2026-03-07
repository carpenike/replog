import { Link } from 'react-router-dom'

export function NotFoundPage() {
  return (
    <div className="flex flex-col items-center justify-center min-h-[60vh]">
      <h1 className="text-6xl font-bold text-muted-foreground mb-2">404</h1>
      <p className="text-lg text-muted-foreground mb-6">Page not found</p>
      <Link to="/" className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90">
        Go Home
      </Link>
    </div>
  )
}
