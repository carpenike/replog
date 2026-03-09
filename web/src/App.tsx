import { Routes, Route, Navigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'
import { useTheme } from '@/lib/useTheme'
import { Layout } from '@/components/Layout'
import { Dashboard } from '@/pages/Dashboard'
import { AthletesList } from '@/pages/AthletesList'
import { AthleteDetail } from '@/pages/AthleteDetail'
import { NewAthlete } from '@/pages/NewAthlete'
import { EditAthlete } from '@/pages/EditAthlete'
import { ExercisesList } from '@/pages/ExercisesList'
import { ExerciseDetail } from '@/pages/ExerciseDetail'
import { NewExercise } from '@/pages/NewExercise'
import { EditExercise } from '@/pages/EditExercise'
import { WorkoutsList } from '@/pages/WorkoutsList'
import { WorkoutDetail } from '@/pages/WorkoutDetail'
import { NewWorkout } from '@/pages/NewWorkout'
import { BodyWeightsList } from '@/pages/BodyWeightsList'
import { TrainingMaxesList } from '@/pages/TrainingMaxesList'
import { JournalPage } from '@/pages/JournalPage'
import { AccessoryPlans } from '@/pages/AccessoryPlans'
import { ExerciseHistory } from '@/pages/ExerciseHistory'
import { PrescriptionPage } from '@/pages/PrescriptionPage'
import { AssignmentsPage } from '@/pages/AssignmentsPage'
import { TMSetup } from '@/pages/TMSetup'
import { ProgramsList } from '@/pages/ProgramsList'
import { ProgramDetail } from '@/pages/ProgramDetail'
import { NewProgram } from '@/pages/NewProgram'
import { EditProgram } from '@/pages/EditProgram'
import { NotificationsList } from '@/pages/NotificationsList'
import { EquipmentList } from '@/pages/EquipmentList'
import { CycleReview } from '@/pages/CycleReview'
import { UsersList } from '@/pages/UsersList'
import { NewUser } from '@/pages/NewUser'
import { EditUser } from '@/pages/EditUser'
import { ExportPage } from '@/pages/ExportPage'
import { PendingReviews } from '@/pages/PendingReviews'
import { AdminSettings } from '@/pages/AdminSettings'
import { PreferencesPage } from '@/pages/PreferencesPage'
import { NotFoundPage } from '@/pages/NotFoundPage'
import { Login } from '@/pages/Login'

import { LoadingPage } from '@/components/ui'

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
        <LoadingPage />
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
        <Route path="/athletes/:id/edit" element={<EditAthlete />} />
        <Route path="/athletes/:id/workouts" element={<WorkoutsList />} />
        <Route path="/athletes/:id/workouts/new" element={<NewWorkout />} />
        <Route path="/athletes/:id/workouts/:workoutId" element={<WorkoutDetail />} />
        <Route path="/athletes/:id/body-weights" element={<BodyWeightsList />} />
        <Route path="/athletes/:id/training-maxes" element={<TrainingMaxesList />} />
        <Route path="/athletes/:id/journal" element={<JournalPage />} />
        <Route path="/athletes/:id/accessories" element={<AccessoryPlans />} />
        <Route path="/athletes/:id/prescription" element={<PrescriptionPage />} />
        <Route path="/athletes/:id/assignments" element={<AssignmentsPage />} />
        <Route path="/athletes/:id/tm-setup" element={<TMSetup />} />
        <Route path="/athletes/:id/exercises/:exerciseId/history" element={<ExerciseHistory />} />
        <Route path="/exercises" element={<ExercisesList user={user} />} />
        <Route path="/exercises/new" element={<NewExercise />} />
        <Route path="/exercises/:id" element={<ExerciseDetail />} />
        <Route path="/exercises/:id/edit" element={<EditExercise />} />
        <Route path="/programs" element={<ProgramsList user={user} />} />
        <Route path="/programs/new" element={<NewProgram />} />
        <Route path="/programs/:id" element={<ProgramDetail />} />
        <Route path="/programs/:id/edit" element={<EditProgram />} />
        <Route path="/equipment" element={<EquipmentList />} />
        <Route path="/athletes/:id/cycle-review" element={<CycleReview />} />
        <Route path="/notifications" element={<NotificationsList />} />
        <Route path="/reviews/pending" element={<PendingReviews />} />
        <Route path="/users" element={<UsersList />} />
        <Route path="/users/new" element={<NewUser />} />
        <Route path="/users/:userId/edit" element={<EditUser />} />
        <Route path="/athletes/:id/export" element={<ExportPage />} />
        <Route path="/admin/settings" element={<AdminSettings />} />
        <Route path="/preferences" element={<PreferencesPage />} />
        <Route path="/login" element={<Navigate to="/" replace />} />
        <Route path="*" element={<NotFoundPage />} />
      </Routes>
    </Layout>
  )
}
