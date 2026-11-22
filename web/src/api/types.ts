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
  avatar_url?: string;
  linked_user_id?: number | null;
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
  avatar_url?: string;
}

export interface User {
  id: number;
  username: string;
  name?: string | null;
  email?: string | null;
  athlete_id?: number | null;
  is_coach: boolean;
  is_admin: boolean;
  /**
   * Stored users.mcp_enabled column. The in-process MCP server that
   * authorized on this flag has been removed, and the admin toggle is
   * gone. The field remains on the user payload so existing rows still
   * round-trip. Default false.
   */
  mcp_enabled: boolean;
  avatar_url?: string;
  impersonating?: boolean;
  real_user_id?: number;
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
  /** working | warmup | drop | failure. Warm-ups are excluded from volume, PRs, and counts server-side. */
  set_type?: SetType;
  notes?: string | null;
  created_at: string;
  updated_at: string;
  exercise_name?: string;
  reps_label?: string;
  /** Form-check clip on this set (workout read only). */
  video?: SetVideo | null;
}

/** A form-check clip on a logged set. `url` streams it with the session cookie. */
export interface SetVideo {
  id: number;
  set_id: number;
  workout_id: number;
  url: string;
  content_type: string;
  size_bytes: number;
  uploaded_by?: string | null;
  coach_comment?: string | null;
  commented_by?: string | null;
  commented_at?: string | null;
  created_at: string;
}

export type SetType = 'working' | 'warmup' | 'drop' | 'failure';

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
  status: string;
  lineage_id?: number | null;
  version: number;
  created_at: string;
  updated_at: string;
  athlete_count?: number;
  athlete_name?: string;
  /** True for a coach-imported template: edit/delete return 409; re-import a new version instead. */
  locked?: boolean;
  /** Import version of a coach-imported template; absent for hand-built templates. */
  import_version?: number | null;
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
  low_readiness?: boolean;
  swap_count?: number;
  /** Sets with a form-check video to watch. */
  video_count?: number;
}

export interface SettingValueData {
  key: string;
  value: string;
  source: string;
  masked: string;
  read_only: boolean;
  field_type: string;
  options?: string[];
  description: string;
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
  group_key?: string | null;
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
  deactivated_at?: string | null;
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
  set_type?: SetType;
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

export interface LoginTokenData {
  id: number;
  label: string | null;
  expires_at: string | null;
  created_at: string;
  expired: boolean;
}

export interface CreatedLoginToken {
  id: number;
  token: string;
  url: string;
}

export interface ServiceTokenData {
  id: number;
  user_id: number;
  label: string | null;
  created_at: string;
  last_used_at: string | null;
  revoked_at: string | null;
}

export interface IssuedServiceToken {
  id: number;
  user_id: number;
  label: string | null;
  token: string;
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
  rest_seconds?: number | null;
  rep_type: string;
  notes?: string | null;
  /** Top of a rep range; reps is the bottom ("8–12"). */
  reps_max?: number | null;
  rir_min?: number | null;
  rir_max?: number | null;
  prescribed_rpe?: number | null;
  /** Warm-up set: shown, but not counted toward progress or PRs. */
  non_working?: boolean;
  /** Superset/circuit group (A, B, ...). */
  group_key?: string | null;
  group_label?: string | null;
}

export interface PrescriptionLineData {
  exercise_name: string;
  exercise_id: number;
  training_max?: number | null;
  group_key?: string | null;
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


// --- Multi-modal logbook (ADR 018 / HOF-011) ---
// These interfaces mirror the Go DTOs in internal/api/responses.go.
// Nullable Go pointers (*T) map to `?: T | null`. Slice fields are never
// null (the constructors use make()), so they are typed `T[]`.

export interface ThrowingSession {
  id: number;
  workout_id: number;
  athlete_id: number;
  date: string;
  throw_type: string;
  throw_count?: number | null;
  max_intent?: number | null;
  velocity?: number | null;
  fatigue: boolean;
  pain: boolean;
  source: string;
  team?: string | null;
  notes?: string | null;
  created_at: string;
  updated_at: string;
}

export interface ConditioningInterval {
  id: number;
  interval_number: number;
  work_seconds?: number | null;
  work_distance?: number | null;
  rest_seconds?: number | null;
  notes?: string | null;
}

export interface ConditioningSession {
  id: number;
  workout_id: number;
  athlete_id: number;
  date: string;
  modality: string;
  session_type: string;
  total_distance?: number | null;
  distance_unit?: string | null;
  duration_seconds?: number | null;
  avg_hr?: number | null;
  rpe?: number | null;
  notes?: string | null;
  intervals: ConditioningInterval[];
  created_at: string;
  updated_at: string;
}

export interface SkillSession {
  id: number;
  workout_id: number;
  athlete_id: number;
  date: string;
  skill_type: string;
  rep_count?: number | null;
  load_kg?: number | null;
  velocity?: number | null;
  duration_seconds?: number | null;
  notes?: string | null;
  created_at: string;
  updated_at: string;
}

export interface RecoveryCheckin {
  id: number;
  workout_id: number;
  athlete_id: number;
  date: string;
  sleep_hours?: number | null;
  soreness?: number | null;
  energy?: number | null;
  mood?: number | null;
  notes?: string | null;
  /** Sleep under 6 h, soreness 8+, or mood/energy 3 or less. Flags the session for the coach; never changes the plan. */
  low_readiness?: boolean;
  created_at: string;
  updated_at: string;
}

/** An athlete's swap of a prescribed exercise for one with the same movement pattern. The program is unchanged. */
export interface ExerciseSwap {
  id: number;
  workout_id: number;
  from_exercise_id: number;
  from_exercise_name: string;
  to_exercise_id: number;
  to_exercise_name: string;
  reason?: string | null;
  created_at: string;
}

export interface WorkoutDetailData {
  workout: Workout;
  groups: ExerciseGroup[];
  swaps?: ExerciseSwap[];
  /** That day's readiness check-in, when one was logged. */
  readiness?: RecoveryCheckin | null;
}

export interface SeasonPhase {
  id: number;
  athlete_id: number;
  sport?: string | null;
  phase: string;
  start_date: string;
  end_date?: string | null;
  notes?: string | null;
  created_at: string;
  updated_at: string;
}

export interface DisciplineLoad {
  discipline: string;
  unit: string;
  acute_7d: number;
  chronic_28d: number;
  // null on cold start (insufficient chronic history); pair with `marker`.
  acwr: number | null;
  // "insufficient_history" when acwr is null.
  marker?: string;
}

export interface LoadSummary {
  athlete_id: number;
  as_of: string;
  disciplines: DisciplineLoad[];
}

export interface PitchSmartStatus {
  age_bracket: string;
  daily_max: number;
  last_session_date?: string;
  last_throw_count?: number;
  over_daily_max: boolean;
  rest_days_required: number;
  rest_days_owed: number;
  next_eligible_date?: string;
  advisory: string;
}

// --- Records board (GET /api/athletes/{id}/records) ---

export interface RecordSetData {
  workout_id: number;
  date: string;
  reps: number;
  /** null/absent for a bodyweight set */
  weight?: number | null;
}

export interface RepMaxData extends RecordSetData {
  /** Heaviest load lifted for at least this many reps. */
  target: number;
}

export interface E1RMRecordData extends RecordSetData {
  e1rm: number;
}

export interface ExerciseRecordsData {
  exercise_id: number;
  name: string;
  session_count: number;
  last_date: string;
  heaviest?: RecordSetData | null;
  best_e1rm?: E1RMRecordData | null;
  most_reps?: RecordSetData | null;
  rep_maxes: RepMaxData[];
}

export interface RecordsData {
  athlete_id: number;
  rep_targets: number[];
  e1rm_source: string;
  e1rm_max_reps: number;
  exercises: ExerciseRecordsData[];
}

// --- Milestones (GET /api/athletes/{id}/milestones) ---

export type MilestoneKind = 'tier' | 'first' | 'consistency';

export interface MilestoneData {
  key: string;
  kind: MilestoneKind;
  title: string;
  detail: string;
  achieved_on: string;
}

export interface MilestoneGoalData {
  key: string;
  title: string;
  current: number;
  target: number;
}

export interface MilestonesData {
  athlete_id: number;
  total_sessions: number;
  current_streak: number;
  best_streak: number;
  pr_count: number;
  /** Newest first. */
  achieved: MilestoneData[];
  next: MilestoneGoalData[];
}

// --- Progress read (GET /api/athletes/{id}/coach/progress) ---

export interface ProgressLoad {
  value: number | null;
  unit: string;
}

export interface ProgressTopSet {
  session_id: number;
  date: string;
  reps: number;
  load: ProgressLoad;
  e1rm: number | null;
  rpe?: number | null;
}

export interface ProgressPoint {
  session_id: number;
  date: string;
  e1rm: number | null;
  top_set: ProgressTopSet;
}

export interface ProgressPR {
  session_id: number;
  date: string;
  e1rm: number;
  reps: number;
  load: ProgressLoad;
}

export interface ProgressExercise {
  exercise_id: number;
  name: string;
  sessions: ProgressPoint[];
  prs: ProgressPR[];
  last_session_top_set?: ProgressTopSet | null;
}

export interface ProgressData {
  athlete_id: number;
  start?: string | null;
  end?: string | null;
  e1rm_source: string;
  exercises: ProgressExercise[];
}

// --- Coach compliance grid (GET /api/coach/compliance) ---

export type ComplianceStatus = 'rest' | 'prescribed' | 'done' | 'partial' | 'skipped' | 'extra';

export interface ComplianceDay {
  date: string;
  /** ISO weekday: 1 = Monday … 7 = Sunday. */
  weekday: number;
  status: ComplianceStatus;
  session_id: number | null;
  program_name: string | null;
  program_day_label: string | null;
}

export interface ComplianceAthlete {
  athlete_id: number;
  name: string;
  planned_count: number;
  done_count: number;
  partial_count: number;
  skipped_count: number;
  prescribed_count: number;
  extra_count: number;
  summary: string;
  days: ComplianceDay[];
}

export interface ComplianceData {
  week_start: string;
  week_end: string;
  timezone: string;
  athletes: ComplianceAthlete[];
}

// --- Athlete calendar (GET /api/athletes/{id}/calendar) ---

export type CalendarDiscipline = 'resistance' | 'throwing' | 'conditioning' | 'skill' | 'recovery';

export interface CalendarSession {
  workout_id: number;
  discipline: CalendarDiscipline | string;
  label: string | null;
  set_count: number;
}

export interface CalendarDay {
  date: string;
  scheduled: boolean;
  program_id: number | null;
  program_name: string | null;
  /** done | partial | missed | upcoming for a scheduled day, else null. */
  status: 'done' | 'partial' | 'missed' | 'upcoming' | null;
  sessions: CalendarSession[];
}

export interface CalendarPhase {
  id: number;
  sport: string | null;
  phase: 'off' | 'pre' | 'in' | string;
  start_date: string;
  /** null = ongoing. */
  end_date: string | null;
}

export interface CalendarData {
  month: string;
  start: string;
  end: string;
  today: string;
  days: CalendarDay[];
  phases: CalendarPhase[];
}

// --- Web Push (GET /api/push/config etc.) ---

export interface PushConfig {
  enabled: boolean;
  public_key: string;
  subscriptions: number;
}

export interface RestAlertResult {
  scheduled: boolean;
  fires_at: string | null;
}

// Whiteboard / TV mode: read-only board of today's session.
export interface WhiteboardLine {
  exercise_id: number;
  exercise_name: string;
  group_key?: string | null;
  prescribed: PrescriptionSetData[];
  logged: WorkoutSet[];
  working_target: number;
  working_done: number;
}

export interface WhiteboardAthlete {
  athlete_id: number;
  name: string;
  tier?: string | null;
  program_name?: string | null;
  current_week?: number;
  current_day?: number;
  workout_id?: number | null;
  lines: WhiteboardLine[];
  working_target: number;
  working_done: number;
}

export interface WhiteboardData {
  date: string;
  athletes: WhiteboardAthlete[];
  available: { id: number; name: string }[];
  truncated: boolean;
}
