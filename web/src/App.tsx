import { Suspense, lazy, type ReactNode } from 'react'
import { Routes, Route, Navigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'
import { useTheme } from '@/lib/useTheme'
import { Layout } from '@/components/Layout'
import { Login } from '@/pages/Login'
import { LoadingPage, Spinner } from '@/components/ui'
import type { User } from '@/api/types'

/** Route guard — redirects to / if the user doesn't have the required role. */
function RequireRole({ user, role, children }: { user: User; role: 'coach' | 'admin'; children: ReactNode }) {
  if (role === 'admin' && !user.is_admin) return <Navigate to="/" replace />
  if (role === 'coach' && !user.is_coach && !user.is_admin) return <Navigate to="/" replace />
  return <>{children}</>
}

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
const TokenLoginPage = lazy(() => import('@/pages/TokenLoginPage').then(m => ({ default: m.TokenLoginPage })))
const ThrowingSessions = lazy(() => import('@/pages/ThrowingSessions').then(m => ({ default: m.ThrowingSessions })))
const ConditioningSessions = lazy(() => import('@/pages/ConditioningSessions').then(m => ({ default: m.ConditioningSessions })))
const SkillSessions = lazy(() => import('@/pages/SkillSessions').then(m => ({ default: m.SkillSessions })))
const RecoveryCheckins = lazy(() => import('@/pages/RecoveryCheckins').then(m => ({ default: m.RecoveryCheckins })))
const SeasonPhases = lazy(() => import('@/pages/SeasonPhases').then(m => ({ default: m.SeasonPhases })))
const LoadDashboard = lazy(() => import('@/pages/LoadDashboard').then(m => ({ default: m.LoadDashboard })))

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
    return (
      <Suspense fallback={<LoadingPage />}>
        <Routes>
          <Route path="/auth/token/:token" element={<TokenLoginPage />} />
          <Route path="*" element={<Login />} />
        </Routes>
      </Suspense>
    )
  }

  return (
    <Layout user={user} theme={theme} onToggleTheme={toggleTheme}>
      <Suspense fallback={<Spinner />}>
      <Routes>
        <Route path="/" element={<Dashboard user={user} />} />
        <Route path="/athletes" element={<AthletesList user={user} />} />
        <Route path="/athletes/new" element={<RequireRole user={user} role="coach"><NewAthlete /></RequireRole>} />
        <Route path="/athletes/:id" element={<AthleteDetail />} />
        <Route path="/athletes/:id/edit" element={<RequireRole user={user} role="coach"><EditAthlete /></RequireRole>} />
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
        <Route path="/athletes/:id/import" element={<RequireRole user={user} role="coach"><ImportPage /></RequireRole>} />
        <Route path="/athletes/:id/generate" element={<RequireRole user={user} role="coach"><GeneratePage /></RequireRole>} />
        <Route path="/athletes/:id/exercises/:exerciseId/history" element={<ExerciseHistory />} />
        <Route path="/athletes/:id/throwing-sessions" element={<ThrowingSessions />} />
        <Route path="/athletes/:id/conditioning-sessions" element={<ConditioningSessions />} />
        <Route path="/athletes/:id/skill-sessions" element={<SkillSessions />} />
        <Route path="/athletes/:id/recovery-checkins" element={<RecoveryCheckins />} />
        <Route path="/athletes/:id/season-phases" element={<SeasonPhases />} />
        <Route path="/athletes/:id/load" element={<LoadDashboard />} />
        <Route path="/exercises" element={<ExercisesList user={user} />} />
        <Route path="/exercises/new" element={<RequireRole user={user} role="coach"><NewExercise /></RequireRole>} />
        <Route path="/exercises/:id" element={<ExerciseDetail />} />
        <Route path="/exercises/:id/edit" element={<RequireRole user={user} role="coach"><EditExercise /></RequireRole>} />
        <Route path="/programs" element={<ProgramsList user={user} />} />
        <Route path="/programs/new" element={<RequireRole user={user} role="coach"><NewProgram /></RequireRole>} />
        <Route path="/programs/:id" element={<ProgramDetail />} />
        <Route path="/programs/:id/edit" element={<RequireRole user={user} role="coach"><EditProgram /></RequireRole>} />
        <Route path="/equipment" element={<RequireRole user={user} role="coach"><EquipmentList /></RequireRole>} />
        <Route path="/athletes/:id/cycle-review" element={<CycleReview />} />
        <Route path="/notifications" element={<NotificationsList />} />
        <Route path="/reviews/pending" element={<RequireRole user={user} role="coach"><PendingReviews /></RequireRole>} />
        <Route path="/users" element={<RequireRole user={user} role="admin"><UsersList /></RequireRole>} />
        <Route path="/users/new" element={<RequireRole user={user} role="admin"><NewUser /></RequireRole>} />
        <Route path="/users/:userId/edit" element={<RequireRole user={user} role="admin"><EditUser /></RequireRole>} />
        <Route path="/athletes/:id/export" element={<ExportPage />} />
        <Route path="/admin/settings" element={<RequireRole user={user} role="admin"><AdminSettings /></RequireRole>} />
        <Route path="/admin/catalog" element={<RequireRole user={user} role="admin"><CatalogAdmin /></RequireRole>} />
        <Route path="/preferences" element={<PreferencesPage />} />
        <Route path="/login" element={<Navigate to="/" replace />} />
        <Route path="*" element={<NotFoundPage />} />
      </Routes>
      </Suspense>
    </Layout>
  )
}
