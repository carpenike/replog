// API types matching the Go internal/api package DTOs.

export interface Athlete {
  id: number;
  name: string;
  tier?: string | null;
  notes?: string | null;
  goal?: string | null;
  date_of_birth?: string | null;
  grade?: string | null;
  gender?: string | null;
  coach_id?: number | null;
  track_body_weight: boolean;
  created_at: string;
  updated_at: string;
}

export interface AthleteCard {
  id: number;
  name: string;
  tier?: string | null;
  active_assignments: number;
  last_workout_date?: string | null;
  week_streak: number;
  bw_trend?: string;
  track_body_weight: boolean;
}

export interface User {
  id: number;
  username: string;
  name?: string | null;
  email?: string | null;
  athlete_id?: number | null;
  is_coach: boolean;
  is_admin: boolean;
  avatar_url?: string;
  created_at: string;
  updated_at: string;
}

export interface Exercise {
  id: number;
  name: string;
  tier?: string | null;
  form_notes?: string | null;
  demo_url?: string | null;
  rest_seconds?: number | null;
  featured: boolean;
  created_at: string;
  updated_at: string;
}

export interface Workout {
  id: number;
  athlete_id: number;
  date: string;
  assignment_id?: number | null;
  notes?: string | null;
  created_at: string;
  updated_at: string;
  athlete_name?: string;
  set_count: number;
  review_status?: string | null;
  program_name?: string;
}

export interface WorkoutPage {
  workouts: Workout[];
  has_more: boolean;
}

export interface WorkoutSet {
  id: number;
  workout_id: number;
  exercise_id: number;
  set_number: number;
  reps: number;
  weight?: number | null;
  rpe?: number | null;
  rep_type: string;
  category: string;
  notes?: string | null;
  created_at: string;
  updated_at: string;
  exercise_name?: string;
  reps_label?: string;
}

export interface TrainingMax {
  id: number;
  athlete_id: number;
  exercise_id: number;
  weight: number;
  effective_date: string;
  notes?: string | null;
  created_at: string;
  exercise_name?: string;
}

export interface BodyWeight {
  id: number;
  athlete_id: number;
  date: string;
  weight: number;
  notes?: string | null;
  created_at: string;
}

export interface BodyWeightPage {
  entries: BodyWeight[];
  has_more: boolean;
}

export interface ProgramTemplate {
  id: number;
  athlete_id?: number | null;
  name: string;
  description?: string | null;
  num_weeks: number;
  num_days: number;
  is_loop: boolean;
  audience?: string | null;
  created_at: string;
  updated_at: string;
  athlete_count?: number;
  athlete_name?: string;
}

export interface Notification {
  id: number;
  user_id: number;
  type: string;
  title: string;
  message?: string | null;
  link?: string | null;
  read: boolean;
  athlete_id?: number | null;
  created_at: string;
}

export interface APIError {
  error: string;
  code: number;
  details?: Record<string, string>;
}

export interface UserPreferences {
  id: number;
  user_id: number;
  weight_unit: string;
  timezone: string;
  date_format: string;
  created_at: string;
  updated_at: string;
}

export interface ExerciseGroup {
  exercise_id: number;
  exercise_name: string;
  sets: WorkoutSet[];
}

export interface BodyWeight {
  id: number;
  athlete_id: number;
  date: string;
  weight: number;
  notes?: string | null;
  created_at: string;
}

export interface BodyWeightPage {
  entries: BodyWeight[];
  has_more: boolean;
}

export interface TrainingMax {
  id: number;
  athlete_id: number;
  exercise_id: number;
  weight: number;
  effective_date: string;
  notes?: string | null;
  created_at: string;
  exercise_name?: string;
}

export interface JournalEntry {
  date: string;
  type: string;
  summary: string;
  id: number;
  detail?: string;
  is_private: boolean;
  pinned: boolean;
  second_id?: number;
  author?: string;
  author_id?: number;
}

export interface UserWithAthlete extends User {
  athlete_name?: string | null;
}

export interface UnreviewedWorkoutData {
  workout_id: number;
  athlete_id: number;
  athlete_name: string;
  date: string;
  set_count: number;
  notes?: string | null;
}

export interface SettingValueData {
  key: string;
  value: string;
  source: string;
  masked: string;
  read_only: boolean;
}

export interface SettingCategoryData {
  category: string;
  settings: SettingValueData[];
}

export interface AccessoryPlanData {
  id: number;
  athlete_id: number;
  day: number;
  exercise_id: number;
  target_sets?: number | null;
  target_rep_min?: number | null;
  target_rep_max?: number | null;
  target_weight?: number | null;
  notes?: string | null;
  sort_order: number;
  active: boolean;
  created_at: string;
  updated_at: string;
  exercise_name?: string;
}

export interface AthleteProgram {
  id: number;
  athlete_id: number;
  template_id: number;
  start_date: string;
  active: boolean;
  role: string;
  schedule?: string | null;
  notes?: string | null;
  goal?: string | null;
  created_at: string;
  updated_at: string;
  template_name?: string;
  num_weeks?: number;
  num_days?: number;
  is_loop?: boolean;
}

export interface ExerciseHistoryEntryData {
  workout_id: number;
  workout_date: string;
  set_number: number;
  reps: number;
  weight?: number | null;
  rpe?: number | null;
  notes?: string | null;
}

export interface ExerciseHistoryDayData {
  workout_id: number;
  workout_date: string;
  sets: ExerciseHistoryEntryData[];
}

export interface ExerciseHistoryPageData {
  days: ExerciseHistoryDayData[];
  has_more: boolean;
}

export interface ProgressionRuleData {
  id: number;
  template_id: number;
  exercise_id: number;
  increment: number;
  exercise_name?: string;
}

export interface EquipmentData {
  id: number;
  name: string;
  description?: string | null;
  created_at: string;
  updated_at: string;
}

export interface ExerciseEquipmentData {
  ID: number;
  ExerciseID: number;
  EquipmentID: number;
  EquipmentName: string;
  Optional: boolean;
}

export interface AthleteEquipmentData {
  ID: number;
  AthleteID: number;
  EquipmentID: number;
  EquipmentName: string;
}

export interface TMSuggestion {
  exercise_id: number;
  exercise_name: string;
  current_tm: number;
  increment: number;
  suggested_tm: number;
}

export interface CycleReviewData {
  cycle_number: number;
  cycle_start: string;
  cycle_end: string;
  suggestions: TMSuggestion[];
}

export interface PrescriptionSetData {
  set_number: number;
  reps?: number | null;
  percentage?: number | null;
  target_weight?: number | null;
  absolute_weight?: number | null;
  rep_type: string;
  notes?: string | null;
}

export interface PrescriptionLineData {
  exercise_name: string;
  exercise_id: number;
  training_max?: number | null;
  sets: PrescriptionSetData[];
}

export interface PrescriptionData {
  program_name: string;
  current_week: number;
  current_day: number;
  cycle_number: number;
  lines: PrescriptionLineData[];
}

export interface AthleteExerciseData {
  id: number;
  athlete_id: number;
  exercise_id: number;
  active: boolean;
  assigned_at: string;
  deactivated_at?: string | null;
  exercise_name?: string;
  exercise_tier?: string | null;
  target_reps?: number | null;
}

export interface ProgramCompatibilityData {
  template_id: number;
  template_name: string;
  ready: boolean;
  ready_count: number;
  total_count: number;
  exercises: { exercise_id: number; exercise_name: string; has_required: boolean }[];
}

export interface MissingTMData {
  exercise_id: number;
  exercise_name: string;
}


export interface PasskeyData {
  id: number;
  label?: string | null;
  created_at: string;
  last_used_at?: string | null;
  use_count: number;
  backup_state: boolean;
}

// WebAuthn browser API types (subset used by our registration flow)
export interface PublicKeyCredentialCreationOptionsJSON {
  publicKey: {
    rp: { name: string; id: string };
    user: { id: string; name: string; displayName: string };
    challenge: string;
    pubKeyCredParams: { type: string; alg: number }[];
    timeout?: number;
    excludeCredentials?: { type: string; id: string; transports?: string[] }[];
    authenticatorSelection?: {
      authenticatorAttachment?: string;
      requireResidentKey?: boolean;
      residentKey?: string;
      userVerification?: string;
    };
    attestation?: string;
  };
}

export interface PublicKeyCredentialJSON {
  id: string;
  rawId: string;
  type: string;
  response: {
    attestationObject: string;
    clientDataJSON: string;
  };
}
