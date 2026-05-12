package api

import (
	"database/sql"
	"time"

	"github.com/carpenike/replog/internal/models"
)

func nullStr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	return &ns.String
}

func nullInt(ni sql.NullInt64) *int64 {
	if !ni.Valid {
		return nil
	}
	return &ni.Int64
}

func nullFloat(nf sql.NullFloat64) *float64 {
	if !nf.Valid {
		return nil
	}
	return &nf.Float64
}

func fmtTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

// fmtDate normalizes a date string from the model layer to YYYY-MM-DD.
//
// SQLite columns declared as DATE are returned by modernc.org/sqlite as
// time.Time values; when scanned into a Go string they format as RFC3339
// timestamps (e.g. "2026-05-12T00:00:00Z"). The API contract for date-only
// fields is YYYY-MM-DD, so we trim the time portion here. Already-trimmed
// strings (e.g. literal "2026-05-12" passed in by the caller) pass through
// unchanged.
func fmtDate(s string) string {
	if len(s) >= 10 && s[4] == '-' && s[7] == '-' {
		return s[:10]
	}
	return s
}

func fmtNullTime(nt sql.NullTime) *time.Time {
	if !nt.Valid {
		return nil
	}
	return &nt.Time
}

func fmtNullTimeStr(nt sql.NullTime) *string {
	if !nt.Valid {
		return nil
	}
	s := nt.Time.Format(time.RFC3339)
	return &s
}

// fmtNullDate normalizes a nullable date string from the model layer.
func fmtNullDate(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	s := fmtDate(ns.String)
	return &s
}

// AthleteFromModel converts a models.Athlete to an API Athlete.
func AthleteFromModel(m *models.Athlete) *Athlete {
	return &Athlete{
		ID:              m.ID,
		Name:            m.Name,
		Tier:            nullStr(m.Tier),
		Notes:           nullStr(m.Notes),
		Goal:            nullStr(m.Goal),
		DateOfBirth:     nullStr(m.DateOfBirth),
		Grade:           nullStr(m.Grade),
		Gender:          nullStr(m.Gender),
		CoachID:         nullInt(m.CoachID),
		TrackBodyWeight: m.TrackBodyWeight,
		CreatedAt:       fmtTime(m.CreatedAt),
		UpdatedAt:       fmtTime(m.UpdatedAt),
	}
}

// AthleteCardFromModel converts a models.AthleteCardInfo to an API AthleteCard.
func AthleteCardFromModel(m *models.AthleteCardInfo) *AthleteCard {
	return &AthleteCard{
		ID:                m.ID,
		Name:              m.Name,
		Tier:              nullStr(m.Tier),
		ActiveAssignments: m.ActiveAssignments,
		LastWorkoutDate:   fmtNullDate(m.LastWorkoutDate),
		WeekStreak:        m.WeekStreak,
		BWTrend:           m.BWTrend,
		TrackBodyWeight:   m.TrackBodyWeight,
		AvatarURL:         nullStr(m.AvatarURL),
	}
}

// UserFromModel converts a models.User to an API User.
func UserFromModel(m *models.User) *User {
	return &User{
		ID:        m.ID,
		Username:  m.Username,
		Name:      nullStr(m.Name),
		Email:     nullStr(m.Email),
		AthleteID: nullInt(m.AthleteID),
		IsCoach:   m.IsCoach,
		IsAdmin:   m.IsAdmin,
		AvatarURL: m.AvatarURL(),
		CreatedAt: fmtTime(m.CreatedAt),
		UpdatedAt: fmtTime(m.UpdatedAt),
	}
}

// ExerciseFromModel converts a models.Exercise to an API Exercise.
func ExerciseFromModel(m *models.Exercise) *Exercise {
	return &Exercise{
		ID:          m.ID,
		Name:        m.Name,
		Tier:        nullStr(m.Tier),
		FormNotes:   nullStr(m.FormNotes),
		DemoURL:     nullStr(m.DemoURL),
		RestSeconds: nullInt(m.RestSeconds),
		Featured:    m.Featured,
		CreatedAt:   fmtTime(m.CreatedAt),
		UpdatedAt:   fmtTime(m.UpdatedAt),
	}
}

// WorkoutFromModel converts a models.Workout to an API Workout.
func WorkoutFromModel(m *models.Workout) *Workout {
	return &Workout{
		ID:           m.ID,
		AthleteID:    m.AthleteID,
		Date:         fmtDate(m.Date),
		AssignmentID: nullInt(m.AssignmentID),
		Notes:        nullStr(m.Notes),
		CreatedAt:    fmtTime(m.CreatedAt),
		UpdatedAt:    fmtTime(m.UpdatedAt),
		AthleteName:  m.AthleteName,
		SetCount:     m.SetCount,
		ReviewStatus: nullStr(m.ReviewStatus),
		ProgramName:  m.ProgramName,
	}
}

// WorkoutPageFromModel converts a models.WorkoutPage to an API WorkoutPage.
func WorkoutPageFromModel(m *models.WorkoutPage) *WorkoutPage {
	workouts := make([]*Workout, len(m.Workouts))
	for i, w := range m.Workouts {
		workouts[i] = WorkoutFromModel(w)
	}
	return &WorkoutPage{
		Workouts: workouts,
		HasMore:  m.HasMore,
	}
}

// WorkoutSetFromModel converts a models.WorkoutSet to an API WorkoutSet.
func WorkoutSetFromModel(m *models.WorkoutSet) *WorkoutSet {
	return &WorkoutSet{
		ID:           m.ID,
		WorkoutID:    m.WorkoutID,
		ExerciseID:   m.ExerciseID,
		SetNumber:    m.SetNumber,
		Reps:         m.Reps,
		Weight:       nullFloat(m.Weight),
		RPE:          nullFloat(m.RPE),
		RepType:      m.RepType,
		Category:     m.Category,
		Notes:        nullStr(m.Notes),
		CreatedAt:    fmtTime(m.CreatedAt),
		UpdatedAt:    fmtTime(m.UpdatedAt),
		ExerciseName: m.ExerciseName,
		RepsLabel:    m.RepsLabel(),
	}
}

// TrainingMaxFromModel converts a models.TrainingMax to an API TrainingMax.
func TrainingMaxFromModel(m *models.TrainingMax) *TrainingMax {
	return &TrainingMax{
		ID:            m.ID,
		AthleteID:     m.AthleteID,
		ExerciseID:    m.ExerciseID,
		Weight:        m.Weight,
		EffectiveDate: fmtDate(m.EffectiveDate),
		Notes:         nullStr(m.Notes),
		CreatedAt:     fmtTime(m.CreatedAt),
		ExerciseName:  m.ExerciseName,
	}
}

// BodyWeightFromModel converts a models.BodyWeight to an API BodyWeight.
func BodyWeightFromModel(m *models.BodyWeight) *BodyWeight {
	return &BodyWeight{
		ID:        m.ID,
		AthleteID: m.AthleteID,
		Date:      fmtDate(m.Date),
		Weight:    m.Weight,
		Notes:     nullStr(m.Notes),
		CreatedAt: fmtTime(m.CreatedAt),
	}
}

// BodyWeightPageFromModel converts a models.BodyWeightPage to an API BodyWeightPage.
func BodyWeightPageFromModel(m *models.BodyWeightPage) *BodyWeightPage {
	entries := make([]*BodyWeight, len(m.Entries))
	for i, e := range m.Entries {
		entries[i] = BodyWeightFromModel(e)
	}
	return &BodyWeightPage{
		Entries: entries,
		HasMore: m.HasMore,
	}
}

// ProgramTemplateFromModel converts a models.ProgramTemplate to an API ProgramTemplate.
func ProgramTemplateFromModel(m *models.ProgramTemplate) *ProgramTemplate {
	return &ProgramTemplate{
		ID:           m.ID,
		AthleteID:    m.AthleteID,
		Name:         m.Name,
		Description:  nullStr(m.Description),
		NumWeeks:     m.NumWeeks,
		NumDays:      m.NumDays,
		IsLoop:       m.IsLoop,
		Audience:     nullStr(m.Audience),
		CreatedAt:    fmtTime(m.CreatedAt),
		UpdatedAt:    fmtTime(m.UpdatedAt),
		AthleteCount: m.AthleteCount,
		AthleteName:  m.AthleteName,
	}
}

// PrescribedSetFromModel converts a models.PrescribedSet to an API PrescribedSet.
func PrescribedSetFromModel(m *models.PrescribedSet) *PrescribedSet {
	return &PrescribedSet{
		ID:             m.ID,
		TemplateID:     m.TemplateID,
		ExerciseID:     m.ExerciseID,
		Week:           m.Week,
		Day:            m.Day,
		SetNumber:      m.SetNumber,
		Reps:           nullInt(m.Reps),
		Percentage:     nullFloat(m.Percentage),
		AbsoluteWeight: nullFloat(m.AbsoluteWeight),
		SortOrder:      m.SortOrder,
		RepType:        m.RepType,
		Notes:          nullStr(m.Notes),
		ExerciseName:   m.ExerciseName,
		TargetWeight:   m.TargetWeight,
	}
}

// AthleteProgramFromModel converts a models.AthleteProgram to an API AthleteProgram.
func AthleteProgramFromModel(m *models.AthleteProgram) *AthleteProgram {
	return &AthleteProgram{
		ID:           m.ID,
		AthleteID:    m.AthleteID,
		TemplateID:   m.TemplateID,
		StartDate:    fmtDate(m.StartDate),
		Active:       m.Active,		DeactivatedAt: fmtNullTimeStr(m.DeactivatedAt),		Role:         m.Role,
		Schedule:     nullStr(m.Schedule),
		Notes:        nullStr(m.Notes),
		Goal:         nullStr(m.Goal),
		CreatedAt:    fmtTime(m.CreatedAt),
		UpdatedAt:    fmtTime(m.UpdatedAt),
		TemplateName: m.TemplateName,
		NumWeeks:     m.NumWeeks,
		NumDays:      m.NumDays,
		IsLoop:       m.IsLoop,
	}
}

// AthleteExerciseFromModel converts a models.AthleteExercise to an API AthleteExercise.
func AthleteExerciseFromModel(m *models.AthleteExercise) *AthleteExercise {
	return &AthleteExercise{
		ID:            m.ID,
		AthleteID:     m.AthleteID,
		ExerciseID:    m.ExerciseID,
		Active:        m.Active,
		AssignedAt:    fmtTime(m.AssignedAt),
		DeactivatedAt: fmtNullTime(m.DeactivatedAt),
		ExerciseName:  m.ExerciseName,
		ExerciseTier:  nullStr(m.ExerciseTier),
		TargetReps:    nullInt(m.TargetReps),
	}
}

// EquipmentFromModel converts a models.Equipment to an API Equipment.
func EquipmentFromModel(m *models.Equipment) *Equipment {
	return &Equipment{
		ID:          m.ID,
		Name:        m.Name,
		Description: nullStr(m.Description),
		CreatedAt:   fmtTime(m.CreatedAt),
		UpdatedAt:   fmtTime(m.UpdatedAt),
	}
}

// NotificationFromModel converts a models.Notification to an API Notification.
func NotificationFromModel(m *models.Notification) *Notification {
	return &Notification{
		ID:        m.ID,
		UserID:    m.UserID,
		Type:      m.Type,
		Title:     m.Title,
		Message:   nullStr(m.Message),
		Link:      nullStr(m.Link),
		Read:      m.Read,
		AthleteID: nullInt(m.AthleteID),
		CreatedAt: fmtTime(m.CreatedAt),
	}
}

// WorkoutReviewFromModel converts a models.WorkoutReview to an API WorkoutReview.
func WorkoutReviewFromModel(m *models.WorkoutReview) *WorkoutReview {
	return &WorkoutReview{
		ID:            m.ID,
		WorkoutID:     m.WorkoutID,
		CoachID:       nullInt(m.CoachID),
		Status:        m.Status,
		Notes:         nullStr(m.Notes),
		CreatedAt:     fmtTime(m.CreatedAt),
		UpdatedAt:     fmtTime(m.UpdatedAt),
		CoachUsername: nullStr(m.CoachUsername),
	}
}

// JournalEntryFromModel converts a models.JournalEntry to an API JournalEntry.
func JournalEntryFromModel(m *models.JournalEntry) *JournalEntry {
	return &JournalEntry{
		Date:      fmtDate(m.Date),
		Type:      m.Type,
		Summary:   m.Summary,
		ID:        m.ID,
		Detail:    m.Detail,
		IsPrivate: m.IsPrivate,
		Pinned:    m.Pinned,
		SecondID:  m.SecondID,
		Author:    m.Author,
		AuthorID:  m.AuthorID,
	}
}

// UserPreferencesFromModel converts a models.UserPreferences to an API UserPreferences.
func UserPreferencesFromModel(m *models.UserPreferences) *UserPreferences {
	return &UserPreferences{
		ID:         m.ID,
		UserID:     m.UserID,
		WeightUnit: m.WeightUnit,
		Timezone:   m.Timezone,
		DateFormat: m.DateFormat,
		CreatedAt:  fmtTime(m.CreatedAt),
		UpdatedAt:  fmtTime(m.UpdatedAt),
	}
}
