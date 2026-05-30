// Package api provides JSON-serializable response types for the REST API.
// These DTOs convert internal model structs (which use sql.Null* types for
// database scanning) into clean JSON representations with pointer types for
// nullable fields.
package api

import "time"

// Athlete is the JSON representation of a models.Athlete.
type Athlete struct {
	ID              int64   `json:"id"`
	Name            string  `json:"name"`
	Tier            *string `json:"tier"`
	Notes           *string `json:"notes"`
	Goal            *string `json:"goal"`
	DateOfBirth     *string `json:"date_of_birth,omitempty"`
	Grade           *string `json:"grade,omitempty"`
	Gender          *string `json:"gender,omitempty"`
	CoachID         *int64  `json:"coach_id,omitempty"`
	TrackBodyWeight bool    `json:"track_body_weight"`
	AvatarURL       string  `json:"avatar_url,omitempty"`
	LinkedUserID    *int64  `json:"linked_user_id,omitempty"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}

// AthleteCard is the JSON representation of models.AthleteCardInfo.
type AthleteCard struct {
	ID                int64   `json:"id"`
	Name              string  `json:"name"`
	Tier              *string `json:"tier"`
	ActiveAssignments int     `json:"active_assignments"`
	LastWorkoutDate   *string `json:"last_workout_date,omitempty"`
	WeekStreak        int     `json:"week_streak"`
	BWTrend           string  `json:"bw_trend,omitempty"`
	TrackBodyWeight   bool    `json:"track_body_weight"`
	AvatarURL         *string `json:"avatar_url,omitempty"`
}

// User is the JSON representation of models.User.
// PasswordHash is never included in API responses.
type User struct {
	ID        int64   `json:"id"`
	Username  string  `json:"username"`
	Name      *string `json:"name"`
	Email     *string `json:"email"`
	AthleteID *int64  `json:"athlete_id,omitempty"`
	IsCoach   bool    `json:"is_coach"`
	IsAdmin   bool    `json:"is_admin"`
	// MCPEnabled mirrors models.User.MCPEnabled (HOF-004): when true,
	// the bearer middleware on /api-mcp/* accepts JWTs that resolve to
	// this user; the webui's scs cookie auth ignores the flag entirely.
	MCPEnabled    bool    `json:"mcp_enabled"`
	AvatarURL     string  `json:"avatar_url,omitempty"`
	Impersonating bool    `json:"impersonating,omitempty"`
	RealUserID    *int64  `json:"real_user_id,omitempty"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

// UserWithAthlete extends User with the linked athlete's name.
type UserWithAthlete struct {
	User
	AthleteName *string `json:"athlete_name,omitempty"`
}

// Exercise is the JSON representation of models.Exercise.
type Exercise struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Tier        *string `json:"tier,omitempty"`
	FormNotes   *string `json:"form_notes,omitempty"`
	DemoURL     *string `json:"demo_url,omitempty"`
	RestSeconds *int64  `json:"rest_seconds,omitempty"`
	Featured    bool    `json:"featured"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

// Workout is the JSON representation of models.Workout.
type Workout struct {
	ID           int64   `json:"id"`
	AthleteID    int64   `json:"athlete_id"`
	Date         string  `json:"date"`
	Discipline   string  `json:"discipline"`
	AssignmentID *int64  `json:"assignment_id,omitempty"`
	Notes        *string `json:"notes,omitempty"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
	AthleteName  string  `json:"athlete_name,omitempty"`
	SetCount     int     `json:"set_count"`
	ReviewStatus *string `json:"review_status,omitempty"`
	ProgramName  string  `json:"program_name,omitempty"`
}

// WorkoutPage wraps a paginated list of workouts.
type WorkoutPage struct {
	Workouts []*Workout `json:"workouts"`
	HasMore  bool       `json:"has_more"`
}

// WorkoutSet is the JSON representation of models.WorkoutSet.
type WorkoutSet struct {
	ID           int64    `json:"id"`
	WorkoutID    int64    `json:"workout_id"`
	ExerciseID   int64    `json:"exercise_id"`
	SetNumber    int      `json:"set_number"`
	Reps         int      `json:"reps"`
	Weight       *float64 `json:"weight,omitempty"`
	RPE          *float64 `json:"rpe,omitempty"`
	RepType      string   `json:"rep_type"`
	Category     string   `json:"category"`
	Notes        *string  `json:"notes,omitempty"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at"`
	ExerciseName string   `json:"exercise_name,omitempty"`
	RepsLabel    string   `json:"reps_label,omitempty"`
}

// ExerciseGroup represents sets grouped by exercise in a workout.
type ExerciseGroup struct {
	ExerciseID   int64         `json:"exercise_id"`
	ExerciseName string        `json:"exercise_name"`
	Sets         []*WorkoutSet `json:"sets"`
}

// TrainingMax is the JSON representation of models.TrainingMax.
type TrainingMax struct {
	ID            int64   `json:"id"`
	AthleteID     int64   `json:"athlete_id"`
	ExerciseID    int64   `json:"exercise_id"`
	Weight        float64 `json:"weight"`
	EffectiveDate string  `json:"effective_date"`
	Notes         *string `json:"notes,omitempty"`
	CreatedAt     string  `json:"created_at"`
	ExerciseName  string  `json:"exercise_name,omitempty"`
}

// BodyWeight is the JSON representation of models.BodyWeight.
type BodyWeight struct {
	ID        int64   `json:"id"`
	AthleteID int64   `json:"athlete_id"`
	Date      string  `json:"date"`
	Weight    float64 `json:"weight"`
	Notes     *string `json:"notes,omitempty"`
	CreatedAt string  `json:"created_at"`
}

// BodyWeightPage wraps a paginated list of body weight entries.
type BodyWeightPage struct {
	Entries []*BodyWeight `json:"entries"`
	HasMore bool          `json:"has_more"`
}

// ThrowingSession is the JSON representation of models.ThrowingSession (ADR 018).
type ThrowingSession struct {
	ID         int64    `json:"id"`
	WorkoutID  int64    `json:"workout_id"`
	AthleteID  int64    `json:"athlete_id"`
	Date       string   `json:"date"`
	ThrowType  string   `json:"throw_type"`
	ThrowCount *int64   `json:"throw_count,omitempty"`
	MaxIntent  *int64   `json:"max_intent,omitempty"`
	Velocity   *float64 `json:"velocity,omitempty"`
	Fatigue    bool     `json:"fatigue"`
	Pain       bool     `json:"pain"`
	Source     string   `json:"source"`
	Team       *string  `json:"team,omitempty"`
	Notes      *string  `json:"notes,omitempty"`
	CreatedAt  string   `json:"created_at"`
	UpdatedAt  string   `json:"updated_at"`
}

// SeasonPhase is the JSON representation of models.SeasonPhase (ADR 018).
type SeasonPhase struct {
	ID        int64   `json:"id"`
	AthleteID int64   `json:"athlete_id"`
	Sport     *string `json:"sport,omitempty"`
	Phase     string  `json:"phase"`
	StartDate string  `json:"start_date"`
	EndDate   *string `json:"end_date,omitempty"`
	Notes     *string `json:"notes,omitempty"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

// BioSample is the JSON representation of models.BioSample (ADR 018).
type BioSample struct {
	ID         int64   `json:"id"`
	AthleteID  int64   `json:"athlete_id"`
	RecordedAt string  `json:"recorded_at"`
	Metric     string  `json:"metric"`
	Value      float64 `json:"value"`
	Unit       *string `json:"unit,omitempty"`
	Source     string  `json:"source"`
	Notes      *string `json:"notes,omitempty"`
	CreatedAt  string  `json:"created_at"`
}

// PitchSmartStatus is the coach-facing, read-only Pitch Smart advisory for an
// athlete (ADR 007/018). It is guidance only — never an automated action.
type PitchSmartStatus struct {
	AgeBracket       string `json:"age_bracket"`
	DailyMax         int    `json:"daily_max"`
	LastSessionDate  string `json:"last_session_date,omitempty"`
	LastThrowCount   int    `json:"last_throw_count,omitempty"`
	OverDailyMax     bool   `json:"over_daily_max"`
	RestDaysRequired int    `json:"rest_days_required"`
	RestDaysOwed     int    `json:"rest_days_owed"`
	NextEligibleDate string `json:"next_eligible_date,omitempty"`
	Advisory         string `json:"advisory"`
}

// ConditioningInterval is the JSON representation of models.ConditioningInterval (ADR 018).
type ConditioningInterval struct {
	ID             int64    `json:"id"`
	IntervalNumber int64    `json:"interval_number"`
	WorkSeconds    *int64   `json:"work_seconds,omitempty"`
	WorkDistance   *float64 `json:"work_distance,omitempty"`
	RestSeconds    *int64   `json:"rest_seconds,omitempty"`
	Notes          *string  `json:"notes,omitempty"`
}

// ConditioningSession is the JSON representation of models.ConditioningSession (ADR 018).
type ConditioningSession struct {
	ID              int64                   `json:"id"`
	WorkoutID       int64                   `json:"workout_id"`
	AthleteID       int64                   `json:"athlete_id"`
	Date            string                  `json:"date"`
	Modality        string                  `json:"modality"`
	SessionType     string                  `json:"session_type"`
	TotalDistance   *float64                `json:"total_distance,omitempty"`
	DistanceUnit    *string                 `json:"distance_unit,omitempty"`
	DurationSeconds *int64                  `json:"duration_seconds,omitempty"`
	AvgHR           *int64                  `json:"avg_hr,omitempty"`
	RPE             *float64                `json:"rpe,omitempty"`
	Notes           *string                 `json:"notes,omitempty"`
	Intervals       []*ConditioningInterval `json:"intervals"`
	CreatedAt       string                  `json:"created_at"`
	UpdatedAt       string                  `json:"updated_at"`
}

// SkillSession is the JSON representation of models.SkillSession (ADR 018).
type SkillSession struct {
	ID              int64    `json:"id"`
	WorkoutID       int64    `json:"workout_id"`
	AthleteID       int64    `json:"athlete_id"`
	Date            string   `json:"date"`
	SkillType       string   `json:"skill_type"`
	RepCount        *int64   `json:"rep_count,omitempty"`
	LoadKg          *float64 `json:"load_kg,omitempty"`
	Velocity        *float64 `json:"velocity,omitempty"`
	DurationSeconds *int64   `json:"duration_seconds,omitempty"`
	Notes           *string  `json:"notes,omitempty"`
	CreatedAt       string   `json:"created_at"`
	UpdatedAt       string   `json:"updated_at"`
}

// RecoveryCheckin is the JSON representation of models.RecoveryCheckin (ADR 018).
type RecoveryCheckin struct {
	ID         int64    `json:"id"`
	WorkoutID  int64    `json:"workout_id"`
	AthleteID  int64    `json:"athlete_id"`
	Date       string   `json:"date"`
	SleepHours *float64 `json:"sleep_hours,omitempty"`
	Soreness   *int64   `json:"soreness,omitempty"`
	Energy     *int64   `json:"energy,omitempty"`
	Notes      *string  `json:"notes,omitempty"`
	CreatedAt  string   `json:"created_at"`
	UpdatedAt  string   `json:"updated_at"`
}

// DisciplineLoad is the per-discipline load rollup within a LoadSummary (ADR 018).
type DisciplineLoad struct {
	Discipline string   `json:"discipline"`
	Unit       string   `json:"unit"`
	Acute7     float64  `json:"acute_7d"`
	Chronic28  float64  `json:"chronic_28d"`
	ACWR       *float64 `json:"acwr"`
	Marker     string   `json:"marker,omitempty"`
}

// LoadSummary is the read-only, advisory per-discipline training-load view for
// an athlete (ADR 018). It is computed on read and never gates or auto-actions.
type LoadSummary struct {
	AthleteID   int64             `json:"athlete_id"`
	AsOf        string            `json:"as_of"`
	Disciplines []*DisciplineLoad `json:"disciplines"`
}

// ProgramTemplate is the JSON representation of models.ProgramTemplate.
type ProgramTemplate struct {
	ID           int64   `json:"id"`
	AthleteID    *int64  `json:"athlete_id,omitempty"`
	Name         string  `json:"name"`
	Description  *string `json:"description,omitempty"`
	NumWeeks     int     `json:"num_weeks"`
	NumDays      int     `json:"num_days"`
	IsLoop       bool    `json:"is_loop"`
	Audience     *string `json:"audience,omitempty"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
	AthleteCount int     `json:"athlete_count,omitempty"`
	AthleteName  string  `json:"athlete_name,omitempty"`
}

// PrescribedSet is the JSON representation of models.PrescribedSet.
type PrescribedSet struct {
	ID             int64    `json:"id"`
	TemplateID     int64    `json:"template_id"`
	ExerciseID     int64    `json:"exercise_id"`
	Week           int      `json:"week"`
	Day            int      `json:"day"`
	SetNumber      int      `json:"set_number"`
	Reps           *int64   `json:"reps"`
	Percentage     *float64 `json:"percentage,omitempty"`
	AbsoluteWeight *float64 `json:"absolute_weight,omitempty"`
	SortOrder      int      `json:"sort_order"`
	RepType        string   `json:"rep_type"`
	Notes          *string  `json:"notes,omitempty"`
	ExerciseName   string   `json:"exercise_name,omitempty"`
	TargetWeight   *float64 `json:"target_weight,omitempty"`
}

// AthleteProgram is the JSON representation of models.AthleteProgram.
type AthleteProgram struct {
	ID           int64   `json:"id"`
	AthleteID    int64   `json:"athlete_id"`
	TemplateID   int64   `json:"template_id"`
	StartDate    string  `json:"start_date"`
	Active       bool    `json:"active"`
	DeactivatedAt *string `json:"deactivated_at,omitempty"`
	Role         string  `json:"role"`
	Schedule     *string `json:"schedule,omitempty"`
	Notes        *string `json:"notes,omitempty"`
	Goal         *string `json:"goal,omitempty"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
	TemplateName string  `json:"template_name,omitempty"`
	NumWeeks     int     `json:"num_weeks,omitempty"`
	NumDays      int     `json:"num_days,omitempty"`
	IsLoop       bool    `json:"is_loop,omitempty"`
}

// AthleteExercise is the JSON representation of models.AthleteExercise (assignment).
type AthleteExercise struct {
	ID            int64      `json:"id"`
	AthleteID     int64      `json:"athlete_id"`
	ExerciseID    int64      `json:"exercise_id"`
	Active        bool       `json:"active"`
	AssignedAt    string     `json:"assigned_at"`
	DeactivatedAt *time.Time `json:"deactivated_at,omitempty"`
	ExerciseName  string     `json:"exercise_name,omitempty"`
	ExerciseTier  *string    `json:"exercise_tier,omitempty"`
	TargetReps    *int64     `json:"target_reps,omitempty"`
}

// Equipment is the JSON representation of models.Equipment.
type Equipment struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

// ExerciseEquipment links an exercise to equipment.
type ExerciseEquipment struct {
	ID            int64  `json:"id"`
	ExerciseID    int64  `json:"exercise_id"`
	EquipmentID   int64  `json:"equipment_id"`
	EquipmentName string `json:"equipment_name"`
	Optional      bool   `json:"optional"`
}

// Notification is the JSON representation of models.Notification.
type Notification struct {
	ID        int64   `json:"id"`
	UserID    int64   `json:"user_id"`
	Type      string  `json:"type"`
	Title     string  `json:"title"`
	Message   *string `json:"message,omitempty"`
	Link      *string `json:"link,omitempty"`
	Read      bool    `json:"read"`
	AthleteID *int64  `json:"athlete_id,omitempty"`
	CreatedAt string  `json:"created_at"`
}

// WorkoutReview is the JSON representation of models.WorkoutReview.
type WorkoutReview struct {
	ID            int64   `json:"id"`
	WorkoutID     int64   `json:"workout_id"`
	CoachID       *int64  `json:"coach_id,omitempty"`
	Status        string  `json:"status"`
	Notes         *string `json:"notes,omitempty"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
	CoachUsername *string `json:"coach_username,omitempty"`
}

// JournalEntry is the JSON representation of models.JournalEntry.
type JournalEntry struct {
	Date      string `json:"date"`
	Type      string `json:"type"`
	Summary   string `json:"summary"`
	ID        int64  `json:"id"`
	Detail    string `json:"detail,omitempty"`
	IsPrivate bool   `json:"is_private"`
	Pinned    bool   `json:"pinned"`
	SecondID  int64  `json:"second_id,omitempty"`
	Author    string `json:"author,omitempty"`
	AuthorID  int64  `json:"author_id,omitempty"`
}

// UserPreferences is the JSON representation of models.UserPreferences.
type UserPreferences struct {
	ID         int64  `json:"id"`
	UserID     int64  `json:"user_id"`
	WeightUnit string `json:"weight_unit"`
	Timezone   string `json:"timezone"`
	DateFormat string `json:"date_format"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// AccessoryPlan is the JSON representation of models.AccessoryPlan.
type AccessoryPlan struct {
	ID           int64    `json:"id"`
	AthleteID    int64    `json:"athlete_id"`
	Day          int      `json:"day"`
	ExerciseID   int64    `json:"exercise_id"`
	TargetSets   *int64   `json:"target_sets,omitempty"`
	TargetRepMin *int64   `json:"target_rep_min,omitempty"`
	TargetRepMax *int64   `json:"target_rep_max,omitempty"`
	TargetWeight *float64 `json:"target_weight,omitempty"`
	Notes        *string  `json:"notes,omitempty"`
	SortOrder    int      `json:"sort_order"`
	Active       bool     `json:"active"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at"`
	ExerciseName string   `json:"exercise_name,omitempty"`
}

// AthleteNote is the JSON representation of models.AthleteNote.
type AthleteNote struct {
	ID        int64  `json:"id"`
	AthleteID int64  `json:"athlete_id"`
	AuthorID  int64  `json:"author_id"`
	Content   string `json:"content"`
	IsPrivate bool   `json:"is_private"`
	Pinned    bool   `json:"pinned"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	Author    string `json:"author,omitempty"`
}

// LoginToken is the JSON representation of models.LoginToken.
type LoginToken struct {
	ID        int64  `json:"id"`
	UserID    int64  `json:"user_id"`
	ExpiresAt string `json:"expires_at"`
	CreatedAt string `json:"created_at"`
	URL       string `json:"url,omitempty"`
}

// AppSetting is the JSON representation of models.AppSetting.
type AppSetting struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	UpdatedAt string `json:"updated_at"`
}

// ExerciseHistoryEntry is the JSON representation of models.ExerciseHistoryEntry.
type ExerciseHistoryEntry struct {
	WorkoutID   int64    `json:"workout_id"`
	WorkoutDate string   `json:"workout_date"`
	SetNumber   int      `json:"set_number"`
	Reps        int      `json:"reps"`
	Weight      *float64 `json:"weight,omitempty"`
	RPE         *float64 `json:"rpe,omitempty"`
	Notes       *string  `json:"notes,omitempty"`
}

// ExerciseHistoryDay groups exercise history entries by workout date.
type ExerciseHistoryDay struct {
	WorkoutID   int64                   `json:"workout_id"`
	WorkoutDate string                  `json:"workout_date"`
	Sets        []*ExerciseHistoryEntry `json:"sets"`
}

// ExerciseHistoryPage wraps a paginated list of exercise history days.
type ExerciseHistoryPage struct {
	Days    []*ExerciseHistoryDay `json:"days"`
	HasMore bool                  `json:"has_more"`
}

// UnreviewedWorkout is the JSON representation of models.UnreviewedWorkout.
type UnreviewedWorkout struct {
	WorkoutID   int64   `json:"workout_id"`
	AthleteID   int64   `json:"athlete_id"`
	AthleteName string  `json:"athlete_name"`
	Date        string  `json:"date"`
	SetCount    int     `json:"set_count"`
	Notes       *string `json:"notes,omitempty"`
}

// ReviewStats is the JSON representation of models.ReviewStats.
type ReviewStats struct {
	PendingCount  int `json:"pending_count"`
	ApprovedCount int `json:"approved_count"`
	NeedsWork     int `json:"needs_work"`
}

// DashboardStats is the JSON representation of models.DashboardStats.
type DashboardStats struct {
	WeekSessions     int     `json:"week_sessions"`
	WeekVolume       float64 `json:"week_volume"`
	TotalAthletes    int     `json:"total_athletes"`
	TrainedThisWeek  int     `json:"trained_this_week"`
	ConsecutiveWeeks int     `json:"consecutive_weeks"`
}

// DashboardResponse is the combined dashboard data.
type DashboardResponse struct {
	Stats       *DashboardStats `json:"stats"`
	ReviewStats *ReviewStats    `json:"review_stats"`
	Athletes    []*AthleteCard  `json:"athletes"`
}

// APIError is the standard error response envelope.
type APIError struct {
	Error   string            `json:"error"`
	Code    int               `json:"code"`
	Details map[string]string `json:"details,omitempty"`
}
