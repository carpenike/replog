import { Link } from 'react-router-dom'
import { usePageTitle } from '@/lib/usePageTitle'
import { Button } from '@/components/ui/button'

export function NotFoundPage() {
  usePageTitle('Page not found')
  return (
    <div className="flex flex-col items-center justify-center min-h-[60vh] text-center">
      <p className="text-6xl font-bold text-muted-foreground mb-2" aria-hidden="true">404</p>
      <h1 className="text-lg text-muted-foreground mb-6">Page not found</h1>
      <Button render={<Link to="/" />}>Go Home</Button>
    </div>
  )
}
