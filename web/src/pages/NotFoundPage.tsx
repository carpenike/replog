import { useNavigate } from 'react-router-dom'
import { Button } from '@/components/ui/button'

export function NotFoundPage() {
  const navigate = useNavigate()
  return (
    <div className="flex flex-col items-center justify-center min-h-[60vh]">
      <h1 className="text-6xl font-bold text-muted-foreground mb-2">404</h1>
      <p className="text-lg text-muted-foreground mb-6">Page not found</p>
      <Button onClick={() => navigate('/')}>
        Go Home
      </Button>
    </div>
  )
}
