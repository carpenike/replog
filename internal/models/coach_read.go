package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrInvalidCursor is returned when a history cursor does not name a
// session belonging to the athlete being read.
var ErrInvalidCursor = errors.New("invalid cursor")

// CoachSessionPageSize is the page size for coach session history.
const CoachSessionPageSize = 20

// CoachLoggedSet is one logged set inside a coach session read.
type CoachLoggedSet struct {
	ID         int64
	SetNumber  int
	Reps       int
	Weight     sql.NullFloat64
	RPE        sql.NullFloat64
	Category   string
	SetType    string
	Notes      sql.NullString
	ExerciseID int64
	Exercise   string
}

// CoachExercise is one exercise's ordered sets inside a session.
type CoachExercise struct {
	ExerciseID int64
	Name       string
	Sets       []CoachLoggedSet
}

// CoachSession is one resistance session in a coach read.
// StartedAt and Duration are not stored; callers leave them empty.
// ProgramDayLabel is derived from the assignment sequence when the
// session has an assignment. It is empty when that cannot be derived.
type CoachSession struct {
	ID              int64
	AthleteID       int64
	Date            string
	Notes           sql.NullString
	ProgramDayLabel string
	TemplateID      int64
	Week            int
	Day             int
	Exercises       []CoachExercise
}

// ProgramDayLabel is the "Week N Day D" label for the seq-th session of a
// program (seq is 1-based). It uses the same week/day split as
// GetPrescription. An empty string means the label cannot be derived.
// ProgramPosition is the week and day for the seq-th session of a program.
// Zero means the label cannot be derived.
func ProgramPosition(seq, numWeeks, numDays int) (week, day int) {
	if seq < 1 || numWeeks < 1 || numDays < 1 {
		return 0, 0
	}
	cycle := numWeeks * numDays
	position := (seq - 1) % cycle
	return position/numDays + 1, position%numDays + 1
}

func ProgramDayLabel(seq, numWeeks, numDays int) string {
	week, day := ProgramPosition(seq, numWeeks, numDays)
	if week == 0 {
		return ""
	}
	return fmt.Sprintf("Week %d Day %d", week, day)
}

// ListCoachSessions returns one page of resistance sessions for an athlete
// in [start, end] (inclusive, YYYY-MM-DD), newest first. exerciseID 0 means
// every exercise. cursorID 0 is the first page; otherwise it is the last
// session id of the previous page and must belong to this athlete.
func ListCoachSessions(ctx context.Context, db *sql.DB, athleteID int64, start, end string, exerciseID, cursorID int64) ([]CoachSession, bool, error) {
	cursorDate := ""
	if cursorID != 0 {
		var owner int64
		var rawDate string
		err := db.QueryRowContext(ctx,
			`SELECT athlete_id, date FROM workouts WHERE id = ?`, cursorID,
		).Scan(&owner, &rawDate)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && owner != athleteID) {
			return nil, false, ErrInvalidCursor
		}
		if err != nil {
			return nil, false, fmt.Errorf("models: coach session cursor: %w", err)
		}
		cursorDate = isoDate(rawDate)
	}

	limit := CoachSessionPageSize + 1
	rows, err := db.QueryContext(ctx, `
		SELECT w.id, w.athlete_id, w.date, w.notes,
		       CASE WHEN w.assignment_id IS NULL OR COALESCE(pt.num_days, 0) < 1 OR COALESCE(pt.num_weeks, 0) < 1
		            THEN 0
		            ELSE (
		              SELECT COUNT(*) FROM workouts p
		               WHERE p.assignment_id = w.assignment_id
		                 AND p.discipline = 'resistance'
		                 AND (date(p.date) < date(w.date)
		                      OR (date(p.date) = date(w.date) AND p.id <= w.id))
		            )
		       END,
		       COALESCE(pt.id, 0), COALESCE(pt.num_weeks, 0), COALESCE(pt.num_days, 0)
		  FROM workouts w
		  LEFT JOIN athlete_programs ap ON ap.id = w.assignment_id
		  LEFT JOIN program_templates pt ON pt.id = ap.template_id
		 WHERE w.athlete_id = ?
		   AND w.discipline = 'resistance'
		   AND date(w.date) >= date(?)
		   AND date(w.date) <= date(?)
		   AND (? = 0 OR EXISTS (
		         SELECT 1 FROM workout_sets ws
		          WHERE ws.workout_id = w.id AND ws.exercise_id = ?))
		   AND (? = 0 OR date(w.date) < date(?)
		        OR (date(w.date) = date(?) AND w.id < ?))
		 ORDER BY date(w.date) DESC, w.id DESC
		 LIMIT ?`,
		athleteID, start, end, exerciseID, exerciseID,
		cursorID, cursorDate, cursorDate, cursorID, limit,
	)
	if err != nil {
		return nil, false, fmt.Errorf("models: list coach sessions for athlete %d: %w", athleteID, err)
	}
	defer rows.Close()

	var sessions []CoachSession
	for rows.Next() {
		var s CoachSession
		var seq, weeks, days int
		if err := rows.Scan(&s.ID, &s.AthleteID, &s.Date, &s.Notes, &seq, &s.TemplateID, &weeks, &days); err != nil {
			return nil, false, fmt.Errorf("models: scan coach session: %w", err)
		}
		s.Date = isoDate(s.Date)
		s.Week, s.Day = ProgramPosition(seq, weeks, days)
		s.ProgramDayLabel = ProgramDayLabel(seq, weeks, days)
		sessions = append(sessions, s)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}

	hasMore := len(sessions) > CoachSessionPageSize
	if hasMore {
		sessions = sessions[:CoachSessionPageSize]
	}
	if err := attachCoachSets(ctx, db, sessions); err != nil {
		return nil, false, err
	}
	return sessions, hasMore, nil
}

func attachCoachSets(ctx context.Context, db *sql.DB, sessions []CoachSession) error {
	if len(sessions) == 0 {
		return nil
	}
	ids := make([]int64, len(sessions))
	index := make(map[int64]int, len(sessions))
	for i := range sessions {
		ids[i] = sessions[i].ID
		index[sessions[i].ID] = i
		sessions[i].Exercises = []CoachExercise{}
	}

	query := `
		SELECT ws.workout_id, ws.id, ws.set_number, ws.reps, ws.weight, ws.rpe,
		       ws.category, ws.set_type, ws.notes, ws.exercise_id, e.name
		  FROM workout_sets ws
		  JOIN exercises e ON e.id = ws.exercise_id
		 WHERE ws.workout_id IN (` + inClause(len(ids)) + `)
		 ORDER BY ws.workout_id, ws.id`
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("models: list coach session sets: %w", err)
	}
	defer rows.Close()

	// exercisePos tracks the exercise group already opened for a session,
	// keyed by session id then exercise id, in first-seen order.
	type groupKey struct{ session, exercise int64 }
	groups := map[groupKey]int{}
	for rows.Next() {
		var workoutID int64
		var set CoachLoggedSet
		if err := rows.Scan(&workoutID, &set.ID, &set.SetNumber, &set.Reps, &set.Weight, &set.RPE,
			&set.Category, &set.SetType, &set.Notes, &set.ExerciseID, &set.Exercise); err != nil {
			return fmt.Errorf("models: scan coach set: %w", err)
		}
		si, ok := index[workoutID]
		if !ok {
			continue
		}
		key := groupKey{workoutID, set.ExerciseID}
		gi, seen := groups[key]
		if !seen {
			sessions[si].Exercises = append(sessions[si].Exercises, CoachExercise{
				ExerciseID: set.ExerciseID,
				Name:       set.Exercise,
			})
			gi = len(sessions[si].Exercises) - 1
			groups[key] = gi
		}
		sessions[si].Exercises[gi].Sets = append(sessions[si].Exercises[gi].Sets, set)
	}
	return rows.Err()
}

func inClause(n int) string {
	if n <= 0 {
		return "NULL"
	}
	b := make([]byte, 0, n*2)
	for i := 0; i < n; i++ {
		if i > 0 {
			b = append(b, ',')
		}
		b = append(b, '?')
	}
	return string(b)
}

func isoDate(s string) string {
	if len(s) >= 10 && s[4] == '-' && s[7] == '-' {
		return s[:10]
	}
	return s
}

// CoachProgressPoint is one session's best estimated 1RM and top set.
// E1RMReps and E1RMWeight describe the set the estimate came from.
// Reps, Weight, and RPE describe the top set (heaviest load, then most reps).
type CoachProgressPoint struct {
	SessionID  int64
	Date       string
	E1RM       sql.NullFloat64
	E1RMReps   int
	E1RMWeight sql.NullFloat64
	Reps       int
	Weight     sql.NullFloat64
	RPE        sql.NullFloat64
}

// CoachPR is a session whose estimated 1RM beat every earlier session.
type CoachPR struct {
	SessionID int64
	Date      string
	E1RM      float64
	Reps      int
	Weight    float64
}

// CoachProgressExercise is one exercise's series, PRs, and last top set.
type CoachProgressExercise struct {
	ExerciseID int64
	Name       string
	Sessions   []CoachProgressPoint
	PRs        []CoachPR
	LastTopSet *CoachProgressPoint
}

// ListCoachProgress returns per-exercise progress for an athlete.
// exerciseID 0 means every exercise the athlete has logged. start and end
// are inclusive YYYY-MM-DD bounds; empty means unbounded on that side.
// Estimated 1RM uses Epley1RM and is null when that formula does not apply.
// PRs are judged against earlier sessions, including those before start.
// LastTopSet is the latest session inside the window.
func ListCoachProgress(ctx context.Context, db *sql.DB, athleteID, exerciseID int64, start, end string) ([]CoachProgressExercise, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT w.id, w.date, e.id, e.name, ws.reps, ws.weight, ws.rpe
		  FROM workout_sets ws
		  JOIN workouts w ON w.id = ws.workout_id
		  JOIN exercises e ON e.id = ws.exercise_id
		 WHERE w.athlete_id = ?
		   AND w.discipline = 'resistance'
		   AND `+countedSet+`
		   AND (? = 0 OR e.id = ?)
		   AND (? = '' OR date(w.date) <= date(?))
		 ORDER BY e.name COLLATE NOCASE, e.id, date(w.date), w.id, ws.id`,
		athleteID, exerciseID, exerciseID, end, end,
	)
	if err != nil {
		return nil, fmt.Errorf("models: coach progress for athlete %d: %w", athleteID, err)
	}
	defer rows.Close()

	var flat []progressSet
	for rows.Next() {
		var row progressSet
		if err := rows.Scan(&row.sessionID, &row.date, &row.exerciseID, &row.name, &row.reps, &row.weight, &row.rpe); err != nil {
			return nil, fmt.Errorf("models: scan coach progress: %w", err)
		}
		row.date = isoDate(row.date)
		flat = append(flat, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	type sess struct {
		id   int64
		date string
		sets []progressSet
	}
	type ex struct {
		id    int64
		name  string
		order []int64
		byID  map[int64]*sess
	}
	var order []int64
	byEx := map[int64]*ex{}
	for _, row := range flat {
		group, ok := byEx[row.exerciseID]
		if !ok {
			group = &ex{id: row.exerciseID, name: row.name, byID: map[int64]*sess{}}
			byEx[row.exerciseID] = group
			order = append(order, row.exerciseID)
		}
		s, ok := group.byID[row.sessionID]
		if !ok {
			s = &sess{id: row.sessionID, date: row.date}
			group.byID[row.sessionID] = s
			group.order = append(group.order, row.sessionID)
		}
		s.sets = append(s.sets, row)
	}

	var out []CoachProgressExercise
	for _, eid := range order {
		group := byEx[eid]
		item := CoachProgressExercise{ExerciseID: group.id, Name: group.name, Sessions: []CoachProgressPoint{}, PRs: []CoachPR{}}
		var best float64
		var haveBest bool
		for _, sid := range group.order {
			s := group.byID[sid]
			point := sessionPoint(s.id, s.date, s.sets)
			inWindow := start == "" || s.date >= start
			if point.E1RM.Valid && (!haveBest || point.E1RM.Float64 > best) {
				if inWindow && point.E1RMWeight.Valid {
					item.PRs = append(item.PRs, CoachPR{
						SessionID: s.id,
						Date:      s.date,
						E1RM:      point.E1RM.Float64,
						Reps:      point.E1RMReps,
						Weight:    point.E1RMWeight.Float64,
					})
				}
				best = point.E1RM.Float64
				haveBest = true
			}
			if inWindow {
				item.Sessions = append(item.Sessions, point)
			}
		}
		if n := len(item.Sessions); n > 0 {
			last := item.Sessions[n-1]
			item.LastTopSet = &last
		}
		out = append(out, item)
	}
	return out, nil
}

type progressSet struct {
	sessionID  int64
	date       string
	exerciseID int64
	name       string
	reps       int
	weight     sql.NullFloat64
	rpe        sql.NullFloat64
}

func sessionPoint(id int64, date string, sets []progressSet) CoachProgressPoint {
	point := CoachProgressPoint{SessionID: id, Date: date}
	var bestE float64
	var haveE bool
	var topReps int
	var topWeight float64
	var haveTop bool
	var topRPE sql.NullFloat64
	for _, set := range sets {
		if e1rm, ok := Epley1RM(set.weight.Float64, set.reps); ok && set.weight.Valid && (!haveE || e1rm > bestE) {
			bestE = e1rm
			haveE = true
			point.E1RMReps = set.reps
			point.E1RMWeight = set.weight
		}
		weighted := set.weight.Valid && set.weight.Float64 > 0
		better := !haveTop
		if !better && weighted && point.Weight.Valid {
			better = set.weight.Float64 > topWeight || (set.weight.Float64 == topWeight && set.reps > topReps)
		}
		if !better && weighted && !point.Weight.Valid {
			better = true
		}
		if !better && !weighted && !point.Weight.Valid && set.reps > topReps {
			better = true
		}
		if better {
			haveTop = true
			topReps = set.reps
			topRPE = set.rpe
			if weighted {
				topWeight = set.weight.Float64
				point.Weight = sql.NullFloat64{Float64: topWeight, Valid: true}
			} else {
				point.Weight = sql.NullFloat64{}
				topWeight = 0
			}
		}
	}
	point.Reps = topReps
	point.RPE = topRPE
	if haveE {
		point.E1RM = sql.NullFloat64{Float64: bestE, Valid: true}
	}
	return point
}

// CoachLastSession is the most recent resistance session that logged one exercise.
type CoachLastSession struct {
	SessionID int64
	AthleteID int64
	Date      string
	Sets      []CoachLoggedSet
}

// LastCoachSession returns the newest resistance session in which this
// athlete logged exerciseID, and only that exercise's sets. ok is false
// when the athlete has no such session.
func LastCoachSession(ctx context.Context, db *sql.DB, athleteID, exerciseID int64) (*CoachLastSession, error) {
	var sessionID int64
	var date string
	err := db.QueryRowContext(ctx, `
		SELECT w.id, w.date
		  FROM workouts w
		  JOIN workout_sets ws ON ws.workout_id = w.id
		 WHERE w.athlete_id = ?
		   AND w.discipline = 'resistance'
		   AND ws.exercise_id = ?
		 ORDER BY date(w.date) DESC, w.id DESC
		 LIMIT 1`, athleteID, exerciseID,
	).Scan(&sessionID, &date)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("models: last coach session athlete %d exercise %d: %w", athleteID, exerciseID, err)
	}

	rows, err := db.QueryContext(ctx, `
		SELECT ws.id, ws.set_number, ws.reps, ws.weight, ws.rpe, ws.category, ws.set_type, ws.notes, ws.exercise_id, e.name
		  FROM workout_sets ws
		  JOIN exercises e ON e.id = ws.exercise_id
		 WHERE ws.workout_id = ? AND ws.exercise_id = ?
		 ORDER BY ws.set_number, ws.id`, sessionID, exerciseID,
	)
	if err != nil {
		return nil, fmt.Errorf("models: last coach session sets: %w", err)
	}
	defer rows.Close()

	session := &CoachLastSession{SessionID: sessionID, AthleteID: athleteID, Date: isoDate(date), Sets: []CoachLoggedSet{}}
	for rows.Next() {
		var set CoachLoggedSet
		if err := rows.Scan(&set.ID, &set.SetNumber, &set.Reps, &set.Weight, &set.RPE, &set.Category, &set.SetType, &set.Notes, &set.ExerciseID, &set.Exercise); err != nil {
			return nil, fmt.Errorf("models: scan last coach set: %w", err)
		}
		session.Sets = append(session.Sets, set)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return session, nil
}

// PlannedDay is one scheduled day in a week plan.
type PlannedDay struct {
	Date             string
	Weekday          int
	ProgramID        int64
	TemplateID       int64
	ProgramName      string
	ProgramDayLabel  string
	Week             int
	Day              int
	SessionID        sql.NullInt64
	Status           string
	CompletedOn      string
	PlannedExercises int
	LoggedExercises  int
}

// WeekPlan is planned-versus-done for one Monday–Sunday week.
type WeekPlan struct {
	WeekStart string
	WeekEnd   string
	AthleteID int64
	Planned   int
	Done      int
	Partial   int
	Missed    int
	Upcoming  int
	Days      []PlannedDay
}

// WeekPlanForAthlete lists explicitly scheduled sessions in the week that
// starts on weekStart (a Monday, YYYY-MM-DD). today is the caller's
// calendar date in the same zone as loc. Days with no stored weekday
// schedule are not invented as planned sessions. This function only reads.
func WeekPlanForAthlete(ctx context.Context, db *sql.DB, athleteID int64, weekStart, today string, loc *time.Location) (*WeekPlan, error) {
	start, err := time.ParseInLocation("2006-01-02", weekStart, loc)
	if err != nil {
		return nil, fmt.Errorf("models: week plan start: %w", err)
	}
	plan := &WeekPlan{
		WeekStart: weekStart,
		WeekEnd:   start.AddDate(0, 0, 6).Format("2006-01-02"),
		AthleteID: athleteID,
		Days:      []PlannedDay{},
	}
	tz := loc.String()
	for i := 0; i < 7; i++ {
		day := start.AddDate(0, 0, i)
		noon := time.Date(day.Year(), day.Month(), day.Day(), 12, 0, 0, 0, loc)
		program, err := ResolveAssignment(ctx, db, athleteID, noon, tz)
		if err != nil {
			return nil, err
		}
		if program == nil || !scheduleIncludes(program, isoWeekday(noon)) {
			continue
		}
		rx, err := GetPrescription(ctx, db, program, noon)
		if err != nil {
			return nil, err
		}
		if rx == nil {
			continue
		}
		date := day.Format("2006-01-02")
		workout, err := GetWorkoutByAthleteDate(ctx, db, athleteID, date)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return nil, err
		}
		plannedIDs := map[int64]struct{}{}
		if !rx.CycleComplete {
			for _, line := range rx.Lines {
				// Warm-up-only lines are not planned work: warm-up sets are
				// not counted as logged, so they would hold a day at partial.
				if lineHasWorkingSet(line) {
					plannedIDs[line.ExerciseID] = struct{}{}
				}
			}
		}
		logged, setCount, err := loggedExerciseIDs(ctx, db, workout)
		if err != nil {
			return nil, err
		}
		loggedPlanned := 0
		for id := range plannedIDs {
			if _, ok := logged[id]; ok {
				loggedPlanned++
			}
		}
		status := plannedStatus(date, today, workout != nil, setCount, len(plannedIDs), loggedPlanned)
		item := PlannedDay{
			Date:             date,
			Weekday:          isoWeekday(noon),
			ProgramID:        program.ID,
			TemplateID:       program.TemplateID,
			ProgramName:      program.TemplateName,
			Status:           status,
			PlannedExercises: len(plannedIDs),
			LoggedExercises:  loggedPlanned,
		}
		if !rx.CycleComplete {
			item.Week = rx.CurrentWeek
			item.Day = rx.CurrentDay
			item.ProgramDayLabel = fmt.Sprintf("Week %d Day %d", rx.CurrentWeek, rx.CurrentDay)
		}
		if workout != nil {
			item.SessionID = sql.NullInt64{Int64: workout.ID, Valid: true}
			item.CompletedOn = isoDate(workout.Date)
		}
		plan.Days = append(plan.Days, item)
		plan.Planned++
		switch status {
		case "done":
			plan.Done++
		case "partial":
			plan.Partial++
		case "missed":
			plan.Missed++
		case "upcoming":
			plan.Upcoming++
		}
	}
	return plan, nil
}

func lineHasWorkingSet(line *PrescriptionLine) bool {
	for _, s := range line.Sets {
		if !s.NonWorking {
			return true
		}
	}
	return false
}

func scheduleIncludes(program *AthleteProgram, weekday int) bool {
	days := program.ScheduleDays()
	if days == nil {
		return false
	}
	for _, d := range days {
		if d == weekday {
			return true
		}
	}
	return false
}

func isoWeekday(t time.Time) int {
	d := int(t.Weekday())
	if d == 0 {
		return 7
	}
	return d
}

func loggedExerciseIDs(ctx context.Context, db *sql.DB, workout *Workout) (map[int64]struct{}, int, error) {
	logged := map[int64]struct{}{}
	if workout == nil {
		return logged, 0, nil
	}
	rows, err := db.QueryContext(ctx,
		`SELECT ws.exercise_id FROM workout_sets ws WHERE ws.workout_id = ? AND `+countedSet, workout.ID)
	if err != nil {
		return nil, 0, fmt.Errorf("models: week plan logged sets: %w", err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, 0, fmt.Errorf("models: scan week plan set: %w", err)
		}
		logged[id] = struct{}{}
		n++
	}
	return logged, n, rows.Err()
}

func plannedStatus(date, today string, hasWorkout bool, setCount, planned, loggedPlanned int) string {
	if !hasWorkout {
		if date < today {
			return "missed"
		}
		return "upcoming"
	}
	if planned == 0 {
		if setCount > 0 {
			return "done"
		}
		return "partial"
	}
	if loggedPlanned >= planned {
		return "done"
	}
	return "partial"
}

// CoachTargetSet is one prescribed set on the next session. Target is nil
// when the prescription does not resolve a weight.
type CoachTargetSet struct {
	SetNumber  int
	Reps       sql.NullInt64
	Target     *float64
	Percentage sql.NullFloat64
	RPE        sql.NullFloat64
}

// CoachTargetExercise is one exercise on the next session.
type CoachTargetExercise struct {
	ExerciseID int64
	Name       string
	Sets       []CoachTargetSet
}

// CoachNextSession is the next session the program would show. It is not created.
type CoachNextSession struct {
	Date            string
	ProgramDayLabel string
	Week            int
	Day             int
	TemplateID      int64
	Exercises       []CoachTargetExercise
}

// CoachPosition is the read-only program position for an athlete.
// Block is never filled: the schema has no block column.
type CoachPosition struct {
	AthleteID       int64
	ProgramID       int64
	TemplateID      int64
	ProgramName     string
	HasProgram      bool
	CycleNumber     int
	Week            int
	Day             int
	ProgramDayLabel string
	CycleComplete   bool
	Next            *CoachNextSession
}

// CoachProgramPosition reports the current program position and the next
// scheduled session. It does not insert or advance a workout.
func CoachProgramPosition(ctx context.Context, db *sql.DB, athleteID int64, today string, loc *time.Location) (*CoachPosition, error) {
	todayTime, err := time.ParseInLocation("2006-01-02", today, loc)
	if err != nil {
		return nil, fmt.Errorf("models: program position today: %w", err)
	}
	noon := time.Date(todayTime.Year(), todayTime.Month(), todayTime.Day(), 12, 0, 0, 0, loc)
	pos := &CoachPosition{AthleteID: athleteID}
	program, err := ResolveAssignment(ctx, db, athleteID, noon, loc.String())
	if err != nil {
		return nil, err
	}
	// A rest day does not resolve a program. The athlete's active primary
	// is still the current block of work; the next loop finds the next
	// scheduled date. Nothing is created.
	if program == nil {
		program, err = GetActiveProgram(ctx, db, athleteID)
		if err != nil {
			return nil, err
		}
	}
	if program == nil {
		return pos, nil
	}
	rx, err := GetPrescription(ctx, db, program, noon)
	if err != nil {
		return nil, err
	}
	if rx == nil {
		return pos, nil
	}
	pos.HasProgram = true
	pos.ProgramID = program.ID
	pos.TemplateID = program.TemplateID
	pos.ProgramName = program.TemplateName
	pos.CycleNumber = rx.CycleNumber
	pos.Week = rx.CurrentWeek
	pos.Day = rx.CurrentDay
	pos.CycleComplete = rx.CycleComplete
	if !rx.CycleComplete {
		pos.ProgramDayLabel = fmt.Sprintf("Week %d Day %d", rx.CurrentWeek, rx.CurrentDay)
	}

	nextDate, ok := nextScheduledDate(ctx, db, athleteID, noon, rx.HasWorkout, loc)
	if !ok {
		return pos, nil
	}
	nextNoon, err := time.ParseInLocation("2006-01-02", nextDate, loc)
	if err != nil {
		return pos, nil
	}
	nextNoon = time.Date(nextNoon.Year(), nextNoon.Month(), nextNoon.Day(), 12, 0, 0, 0, loc)
	nextProgram, err := ResolveAssignment(ctx, db, athleteID, nextNoon, loc.String())
	if err != nil {
		return nil, err
	}
	if nextProgram == nil {
		return pos, nil
	}
	nextRx, err := GetPrescription(ctx, db, nextProgram, nextNoon)
	if err != nil {
		return nil, err
	}
	if nextRx == nil || nextRx.CycleComplete {
		return pos, nil
	}
	next := &CoachNextSession{
		Date:            nextDate,
		Week:            nextRx.CurrentWeek,
		Day:             nextRx.CurrentDay,
		TemplateID:      nextProgram.TemplateID,
		ProgramDayLabel: fmt.Sprintf("Week %d Day %d", nextRx.CurrentWeek, nextRx.CurrentDay),
		Exercises:       targetExercises(nextRx),
	}
	pos.Next = next
	return pos, nil
}

// nextScheduledDate finds the next calendar date that an explicit weekday
// schedule claims, or today when today's resolved program is a catch-all
// and today is not already logged. It does not invent a future date for an
// unscheduled primary.
func nextScheduledDate(ctx context.Context, db *sql.DB, athleteID int64, today time.Time, hasWorkout bool, loc *time.Location) (string, bool) {
	start := 0
	if hasWorkout {
		start = 1
	}
	for i := start; i < 21; i++ {
		day := today.AddDate(0, 0, i)
		noon := time.Date(day.Year(), day.Month(), day.Day(), 12, 0, 0, 0, loc)
		program, err := ResolveAssignment(ctx, db, athleteID, noon, loc.String())
		if err != nil || program == nil {
			continue
		}
		if program.ScheduleDays() == nil {
			if i == 0 {
				return noon.Format("2006-01-02"), true
			}
			continue
		}
		if scheduleIncludes(program, isoWeekday(noon)) {
			return noon.Format("2006-01-02"), true
		}
	}
	return "", false
}

func targetExercises(rx *Prescription) []CoachTargetExercise {
	if rx == nil {
		return []CoachTargetExercise{}
	}
	out := make([]CoachTargetExercise, 0, len(rx.Lines))
	for _, line := range rx.Lines {
		item := CoachTargetExercise{ExerciseID: line.ExerciseID, Name: line.ExerciseName, Sets: []CoachTargetSet{}}
		for _, set := range line.Sets {
			item.Sets = append(item.Sets, CoachTargetSet{
				SetNumber:  set.SetNumber,
				Reps:       set.Reps,
				Target:     set.TargetWeight,
				Percentage: set.Percentage,
				RPE:        set.PrescribedRPE,
			})
		}
		out = append(out, item)
	}
	return out
}
