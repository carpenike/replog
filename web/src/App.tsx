import { Routes, Route, Navigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'
import { Layout } from '@/components/Layout'
import { Dashboard } from '@/pages/Dashboard'
import { AthletesList } from '@/pages/AthletesList'
import { AthleteDetail } from '@/pages/AthleteDetail'
import { ExercisesList } from '@/pages/ExercisesList'
import { WorkoutsList } from '@/pages/WorkoutsList'
import { WorkoutDetail } from '@/pages/WorkoutDetail'
import { ProgramsList } from '@/pages/ProgramsList'
import { NotificationsList } from '@/pages/NotificationsList'
import { UsersList } from '@/pages/UsersList'
import { Login } from '@/pages/Login'

export function App() {
  const { data: user, isLoading, error } = useQuery({
    queryKey: ['me'],
    queryFn: () => api.me(),
    retry: false,
  })

  if (isLoading) {
    return (
      <div className="flex items-center justify-center min-h-screen bg-background text-foreground">
        <p className="text-muted-foreground">Loading...</p>
      </div>
    )
  }

  if (error || !user) {
    return <Login />
  }

  return (
    <Layout user={user}>
      <Routes>
        <Route path="/" element={<Dashboard user={user} />} />
        <Route path="/athletes" element={<AthletesList />} />
        <Route path="/athletes/:id" element={<AthleteDetail />} />
        <Route path="/athletes/:id/workouts" element={<WorkoutsList />} />
        <Route path="/athletes/:id/workouts/:workoutId" element={<WorkoutDetail />} />
        <Route path="/exercises" element={<ExercisesList />} />
        <Route path="/programs" element={<ProgramsList />} />
        <Route path="/notifications" element={<NotificationsList />} />
        <Route path="/users" element={<UsersList />} />
        <Route path="/login" element={<Navigate to="/" replace />} />
      </Routes>
    </Layout>
  )
}
