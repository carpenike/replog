import { Suspense, lazy } from 'react'
import { Routes, Route, Navigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'
import { useTheme } from '@/lib/useTheme'
import { Layout } from '@/components/Layout'
import { Login } from '@/pages/Login'
import { LoadingPage, Spinner } from '@/components/ui'

// Lazy-load all page components for code-splitting
const Dashboard = lazy(() => import('@/pages/Dashboard').then(m => ({ default: m.Dashboard })))
const AthletesList = lazy(() => import('@/pages/AthletesList').then(m => ({ default: m.AthletesList })))
const AthleteDetail = lazy(() => import('@/pages/AthleteDetail').then(m => ({ default: m.AthleteDetail })))
const NewAthlete = lazy(() => import('@/pages/NewAthlete').then(m => ({ default: m.NewAthlete })))
const EditAthlete = lazy(() => import('@/pages/EditAthlete').then(m => ({ default: m.EditAthlete })))
const ExercisesList = lazy(() => import('@/pages/ExercisesList').then(m => ({ default: m.ExercisesList })))
const ExerciseDetail = lazy(() => import('@/pages/ExerciseDetail').then(m => ({ default: m.ExerciseDetail })))
const NewExercise = lazy(() => import('@/pages/NewExercise').then(m => ({ default: m.NewExercise })))
const EditExercise = lazy(() => import('@/pages/EditExercise').then(m => ({ default: m.EditExercise })))
const WorkoutsList = lazy(() => import('@/pages/WorkoutsList').then(m => ({ default: m.WorkoutsList })))
const WorkoutDetail = lazy(() => import('@/pages/WorkoutDetail').then(m => ({ default: m.WorkoutDetail })))
const NewWorkout = lazy(() => import('@/pages/NewWorkout').then(m => ({ default: m.NewWorkout })))
const BodyWeightsList = lazy(() => import('@/pages/BodyWeightsList').then(m => ({ default: m.BodyWeightsList })))
const TrainingMaxesList = lazy(() => import('@/pages/TrainingMaxesList').then(m => ({ default: m.TrainingMaxesList })))
const JournalPage = lazy(() => import('@/pages/JournalPage').then(m => ({ default: m.JournalPage })))
const AccessoryPlans = lazy(() => import('@/pages/AccessoryPlans').then(m => ({ default: m.AccessoryPlans })))
const ExerciseHistory = lazy(() => import('@/pages/ExerciseHistory').then(m => ({ default: m.ExerciseHistory })))
const PrescriptionPage = lazy(() => import('@/pages/PrescriptionPage').then(m => ({ default: m.PrescriptionPage })))
const AssignmentsPage = lazy(() => import('@/pages/AssignmentsPage').then(m => ({ default: m.AssignmentsPage })))
const TMSetup = lazy(() => import('@/pages/TMSetup').then(m => ({ default: m.TMSetup })))
const ImportPage = lazy(() => import('@/pages/ImportPage').then(m => ({ default: m.ImportPage })))
const GeneratePage = lazy(() => import('@/pages/GeneratePage').then(m => ({ default: m.GeneratePage })))
const ProgramsList = lazy(() => import('@/pages/ProgramsList').then(m => ({ default: m.ProgramsList })))
const ProgramDetail = lazy(() => import('@/pages/ProgramDetail').then(m => ({ default: m.ProgramDetail })))
const NewProgram = lazy(() => import('@/pages/NewProgram').then(m => ({ default: m.NewProgram })))
const EditProgram = lazy(() => import('@/pages/EditProgram').then(m => ({ default: m.EditProgram })))
const NotificationsList = lazy(() => import('@/pages/NotificationsList').then(m => ({ default: m.NotificationsList })))
const EquipmentList = lazy(() => import('@/pages/EquipmentList').then(m => ({ default: m.EquipmentList })))
const CycleReview = lazy(() => import('@/pages/CycleReview').then(m => ({ default: m.CycleReview })))
const UsersList = lazy(() => import('@/pages/UsersList').then(m => ({ default: m.UsersList })))
const NewUser = lazy(() => import('@/pages/NewUser').then(m => ({ default: m.NewUser })))
const EditUser = lazy(() => import('@/pages/EditUser').then(m => ({ default: m.EditUser })))
const ExportPage = lazy(() => import('@/pages/ExportPage').then(m => ({ default: m.ExportPage })))
const PendingReviews = lazy(() => import('@/pages/PendingReviews').then(m => ({ default: m.PendingReviews })))
const AdminSettings = lazy(() => import('@/pages/AdminSettings').then(m => ({ default: m.AdminSettings })))
const CatalogAdmin = lazy(() => import('@/pages/CatalogAdmin').then(m => ({ default: m.CatalogAdmin })))
const PreferencesPage = lazy(() => import('@/pages/PreferencesPage').then(m => ({ default: m.PreferencesPage })))
const NotFoundPage = lazy(() => import('@/pages/NotFoundPage').then(m => ({ default: m.NotFoundPage })))

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
      <Suspense fallback={<Spinner />}>
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
        <Route path="/athletes/:id/import" element={<ImportPage />} />
        <Route path="/athletes/:id/generate" element={<GeneratePage />} />
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
        <Route path="/admin/catalog" element={<CatalogAdmin />} />
        <Route path="/preferences" element={<PreferencesPage />} />
        <Route path="/login" element={<Navigate to="/" replace />} />
        <Route path="*" element={<NotFoundPage />} />
      </Routes>
      </Suspense>
    </Layout>
  )
}
