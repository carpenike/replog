import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useQueryClient } from '@tanstack/react-query'
import { api, ApiError } from '@/api/client'
import { Loader2 } from 'lucide-react'
import { usePageTitle } from '@/lib/usePageTitle'
import { Alert } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'

export function TokenLoginPage() {
  usePageTitle('Signing in')
  const { token } = useParams<{ token: string }>()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  // Initialize error synchronously when token is missing so we never
  // call setState during an effect on first render.
  const [error, setError] = useState(token ? '' : 'Invalid login link')

  useEffect(() => {
    if (!token) return

    async function authenticate() {
      try {
        const result = await api.tokenLogin(token!)
        await queryClient.invalidateQueries({ queryKey: ['me'] })
        navigate(result.redirect, { replace: true })
      } catch (err) {
        if (err instanceof ApiError) {
          setError(err.message)
        } else {
          setError('Login failed. The link may be expired.')
        }
      }
    }

    authenticate()
  }, [token, navigate, queryClient])

  if (error) {
    return (
      <div className="flex items-center justify-center min-h-screen bg-background">
        <div className="max-w-sm w-full space-y-4 p-6">
          <Alert variant="destructive">{error}</Alert>
          <Button className="w-full" onClick={() => navigate('/login', { replace: true })}>
            Go to Login
          </Button>
        </div>
      </div>
    )
  }

  return (
    <div className="flex items-center justify-center min-h-screen bg-background">
      <div role="status" className="flex flex-col items-center text-center">
        <Loader2 className="size-6 animate-spin text-primary" aria-hidden="true" />
        <h1 className="text-sm text-muted-foreground mt-2">Signing you in...</h1>
      </div>
    </div>
  )
}
