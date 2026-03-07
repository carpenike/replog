import { Routes, Route, Navigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'
import { useTheme } from '@/lib/useTheme'
import { Layout } from '@/components/Layout'
import { Dashboard } from '@/pages/Dashboard'
import { AthletesList } from '@/pages/AthletesList'
import { AthleteDetail } from '@/pages/AthleteDetail'
import { NewAthlete } from '@/pages/NewAthlete'
import { ExercisesList } from '@/pages/ExercisesList'
import { ExerciseDetail } from '@/pages/ExerciseDetail'
import { NewExercise } from '@/pages/NewExercise'
import { WorkoutsList } from '@/pages/WorkoutsList'
import { WorkoutDetail } from '@/pages/WorkoutDetail'
import { NewWorkout } from '@/pages/NewWorkout'
import { BodyWeightsList } from '@/pages/BodyWeightsList'
import { TrainingMaxesList } from '@/pages/TrainingMaxesList'
import { JournalPage } from '@/pages/JournalPage'
import { ProgramsList } from '@/pages/ProgramsList'
import { NotificationsList } from '@/pages/NotificationsList'
import { UsersList } from '@/pages/UsersList'
import { PreferencesPage } from '@/pages/PreferencesPage'
import { NotFoundPage } from '@/pages/NotFoundPage'
import { Login } from '@/pages/Login'

export function App() {
  const { theme, toggleTheme } = useTheme()
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
    <Layout user={user} theme={theme} onToggleTheme={toggleTheme}>
      <Routes>
        <Route path="/" element={<Dashboard user={user} />} />
        <Route path="/athletes" element={<AthletesList user={user} />} />
        <Route path="/athletes/new" element={<NewAthlete />} />
        <Route path="/athletes/:id" element={<AthleteDetail />} />
        <Route path="/athletes/:id/workouts" element={<WorkoutsList />} />
        <Route path="/athletes/:id/workouts/new" element={<NewWorkout />} />
        <Route path="/athletes/:id/workouts/:workoutId" element={<WorkoutDetail />} />
        <Route path="/athletes/:id/body-weights" element={<BodyWeightsList />} />
        <Route path="/athletes/:id/training-maxes" element={<TrainingMaxesList />} />
        <Route path="/athletes/:id/journal" element={<JournalPage />} />
        <Route path="/exercises" element={<ExercisesList user={user} />} />
        <Route path="/exercises/new" element={<NewExercise />} />
        <Route path="/exercises/:id" element={<ExerciseDetail />} />
        <Route path="/programs" element={<ProgramsList />} />
        <Route path="/notifications" element={<NotificationsList />} />
        <Route path="/users" element={<UsersList />} />
        <Route path="/preferences" element={<PreferencesPage />} />
        <Route path="/login" element={<Navigate to="/" replace />} />
        <Route path="*" element={<NotFoundPage />} />
      </Routes>
    </Layout>
  )
}
