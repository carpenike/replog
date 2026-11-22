package models

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	// BlockStraight, BlockSuperset, and BlockCircuit are the block types
	// a coach import can store.
	BlockStraight = "straight"
	BlockSuperset = "superset"
	BlockCircuit  = "circuit"
	cycleWeekly   = "weekly"
	matchOnName   = "name"
	matchOnAlias  = "alias"
)

var (
	// ErrProgramLocked is returned when a change would edit a locked template.
	ErrProgramLocked = errors.New("program is locked")
	// ErrCoachFieldSet is returned when a gap-fill would change a coach-set field.
	ErrCoachFieldSet = errors.New("coach field is already set")
	// ErrExerciseAmbiguous is returned when a name and alias point at different exercises.
	ErrExerciseAmbiguous = errors.New("exercise name matches more than one exercise")
	// ErrVersionMismatch is returned when an assign names a different version than the template.
	ErrVersionMismatch = errors.New("version does not match")
)

// CoachFieldError names the coach-set field a gap-fill tried to change.
type CoachFieldError struct {
	Field string
}

func (e *CoachFieldError) Error() string {
	return e.Field + " was set by the coach"
}

func (e *CoachFieldError) Unwrap() error { return ErrCoachFieldSet }

// CoachProgramInput is the program document a preview accepts.
type CoachProgramInput struct {
	Name      string           `json:"name"`
	Source    string           `json:"source"`
	Author    string           `json:"author"`
	StartDate *string          `json:"start_date"`
	Weeks     *int             `json:"weeks"`
	Cycle     *CoachCycleInput `json:"cycle"`
	Days      []CoachDayInput  `json:"days"`
}

// CoachCycleInput is a repeating schedule. Upcoming is metadata, not copies of a day.
type CoachCycleInput struct {
	Repeat   string `json:"repeat"`
	Weekdays []int  `json:"weekdays"`
	Upcoming *int   `json:"upcoming"`
}

// CoachDayInput is one day in the program document.
type CoachDayInput struct {
	Label            string            `json:"label"`
	Order            int               `json:"order"`
	Week             *int              `json:"week"`
	Weekday          *int              `json:"weekday"`
	Kind             *string           `json:"kind"`
	EstimatedMinutes *int              `json:"estimated_minutes"`
	ExerciseCount    *int              `json:"exercise_count"`
	Equipment        []string          `json:"equipment"`
	Instructions     *string           `json:"instructions"`
	Blocks           []CoachBlockInput `json:"blocks"`
}

// CoachBlockInput is one straight, superset, or circuit block.
type CoachBlockInput struct {
	Type        string               `json:"type"`
	Order       int                  `json:"order"`
	Rounds      *int                 `json:"rounds"`
	Sets        *int                 `json:"sets"`
	RestSeconds *int                 `json:"rest_seconds"`
	Rest        *string              `json:"rest"`
	NonWorking  bool                 `json:"non_working"`
	Exercises   []CoachExerciseInput `json:"exercises"`
}

// CoachExerciseInput is one exercise line inside a block.
type CoachExerciseInput struct {
	Name        string  `json:"name"`
	Order       int     `json:"order"`
	Sets        *int    `json:"sets"`
	RepsMin     *int    `json:"reps_min"`
	RepsMax     *int    `json:"reps_max"`
	RIRMin      *int    `json:"rir_min"`
	RIRMax      *int    `json:"rir_max"`
	PerSide     bool    `json:"per_side"`
	RestSeconds *int    `json:"rest_seconds"`
	Rest        *string `json:"rest"`
	Notes       *string `json:"notes"`
	NonWorking  *bool   `json:"non_working"`
}

// CoachProgram is the normalized program a preview token carries and an import stores.
type CoachProgram struct {
	Name      string
	Source    string
	Author    string
	StartDate *string
	Weeks     *int
	Cycle     *CoachCycle
	Days      []CoachDay
}

// CoachCycle is the repeating schedule stored on the import.
type CoachCycle struct {
	Repeat   string
	Weekdays []int
	Upcoming *int
}

// CoachDay is one stored day. Week is the program week (1 for a weekly cycle).
// Number is the prescription day index. Order is the coach's order.
type CoachDay struct {
	Label            string
	Order            int
	Week             int
	Number           int
	Weekday          *int
	Kind             *string
	EstimatedMinutes *int
	ExerciseCount    int
	Equipment        []string
	Instructions     *string
	Blocks           []CoachBlock
}

// CoachBlock is one stored block.
type CoachBlock struct {
	Order       int
	Type        string
	Rounds      *int
	Sets        *int
	RestSeconds *int
	Rest        *string
	NonWorking  bool
	Exercises   []CoachBlockExercise
}

// CoachBlockExercise is one stored exercise line. AI is empty until a gap-fill.
type CoachBlockExercise struct {
	ID          int64
	ExerciseID  int64
	Name        string
	Order       int
	Sets        *int
	RepsMin     *int
	RepsMax     *int
	RIRMin      *int
	RIRMax      *int
	PerSide     bool
	RestSeconds *int
	Rest        *string
	Notes       *string
	NonWorking  bool
	AI          *CoachAIFields
}

// CoachAIFields holds suggestions for fields the coach left blank.
type CoachAIFields struct {
	Sets        *int
	RepsMin     *int
	RepsMax     *int
	RIRMin      *int
	RIRMax      *int
	RestSeconds *int
	Notes       *string
	Load        *float64
}

// ExerciseMatch is a catalog row a program name already identifies.
type ExerciseMatch struct {
	Name         string
	ExerciseID   int64
	ExerciseName string
	MatchedOn    string
}

// ExerciseCreate is a program name with no match, and the likely matches
// a coach may want to alias it to instead.
type ExerciseCreate struct {
	Name        string
	Suggestions []ExerciseSuggestion
}

// ProgramExerciseMatches is what a preview reports about exercise names.
type ProgramExerciseMatches struct {
	Matched     []ExerciseMatch
	WouldCreate []ExerciseCreate
	Ambiguous   []AmbiguousExerciseError
}

// CoachExerciseDraft is a create-exercise request.
type CoachExerciseDraft struct {
	Name           string
	Aliases        []string
	Equipment      []string
	PrimaryMuscles []string
	Unilateral     bool
	Notes          *string
}

// CoachCatalogExercise is a catalog row with the import fields.
type CoachCatalogExercise struct {
	ID             int64
	Name           string
	Aliases        []string
	Equipment      []string
	PrimaryMuscles []string
	Unilateral     bool
	Notes          *string
}

// StoredCoachProgram is an imported template and its days.
type StoredCoachProgram struct {
	TemplateID      int64
	AthleteID       int64
	Name            string
	Version         int
	NumWeeks        int
	LineageID       *int64
	Locked          bool
	Source          string
	Author          string
	StartDate       *string
	Cycle           *CoachCycle
	AssignmentID    *int64
	AssignmentStart *string
	Days            []CoachDay
}

// CoachImportResult is the template an import wrote and the exercises it created.
type CoachImportResult struct {
	Program          StoredCoachProgram
	CreatedExercises []string
}

// GapFillInput names one exercise line and the blank fields to suggest.
type GapFillInput struct {
	DayOrder      int
	BlockOrder    int
	ExerciseOrder int
	Sets          *int
	RepsMin       *int
	RepsMax       *int
	RIRMin        *int
	RIRMax        *int
	RestSeconds   *int
	Notes         *string
	Load          *float64
	ExerciseName  *string
}

type queryer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// NormalizeCoachProgram checks a program document and returns the form that is stored.
// Instructions and notes are kept as given. A weekly cycle's upcoming count is metadata.
func NormalizeCoachProgram(in CoachProgramInput) (CoachProgram, error) {
	var out CoachProgram
	out.Name = strings.TrimSpace(in.Name)
	out.Source = strings.TrimSpace(in.Source)
	out.Author = strings.TrimSpace(in.Author)
	if out.Name == "" || out.Source == "" || out.Author == "" {
		return out, fmt.Errorf("%w: name, source, and author are required", ErrInvalidInput)
	}
	if in.StartDate != nil {
		start := strings.TrimSpace(*in.StartDate)
		if start != "" {
			if _, err := parseISODate(start); err != nil {
				return out, fmt.Errorf("%w: start_date must be YYYY-MM-DD", ErrInvalidInput)
			}
			out.StartDate = &start
		}
	}
	if in.Weeks != nil {
		if *in.Weeks < 1 {
			return out, fmt.Errorf("%w: weeks must be at least 1", ErrInvalidInput)
		}
		weeks := *in.Weeks
		out.Weeks = &weeks
	}
	if in.Cycle != nil {
		cycle, err := normalizeCycle(*in.Cycle)
		if err != nil {
			return out, err
		}
		out.Cycle = &cycle
	}
	if out.Weeks == nil && out.Cycle == nil {
		return out, fmt.Errorf("%w: weeks or a weekly cycle is required", ErrInvalidInput)
	}
	if len(in.Days) == 0 {
		return out, fmt.Errorf("%w: at least one day is required", ErrInvalidInput)
	}
	out.Days = make([]CoachDay, 0, len(in.Days))
	seenDay := map[int]struct{}{}
	for i, day := range in.Days {
		normalized, err := normalizeDay(day, i+1)
		if err != nil {
			return out, err
		}
		if _, ok := seenDay[normalized.Order]; ok {
			return out, fmt.Errorf("%w: day order %d is repeated", ErrInvalidInput, normalized.Order)
		}
		seenDay[normalized.Order] = struct{}{}
		out.Days = append(out.Days, normalized)
	}
	sortDays(out.Days)
	for i := range out.Days {
		out.Days[i].Number = i + 1
	}
	return out, nil
}

func normalizeCycle(in CoachCycleInput) (CoachCycle, error) {
	repeat := strings.ToLower(strings.TrimSpace(in.Repeat))
	if repeat != cycleWeekly {
		return CoachCycle{}, fmt.Errorf("%w: cycle repeat must be weekly", ErrInvalidInput)
	}
	if len(in.Weekdays) == 0 {
		return CoachCycle{}, fmt.Errorf("%w: a weekly cycle needs weekdays", ErrInvalidInput)
	}
	seen := map[int]struct{}{}
	days := make([]int, 0, len(in.Weekdays))
	for _, day := range in.Weekdays {
		if day < 1 || day > 7 {
			return CoachCycle{}, fmt.Errorf("%w: weekdays must be 1 (Monday) through 7 (Sunday)", ErrInvalidInput)
		}
		if _, ok := seen[day]; ok {
			return CoachCycle{}, fmt.Errorf("%w: weekdays must not repeat", ErrInvalidInput)
		}
		seen[day] = struct{}{}
		days = append(days, day)
	}
	var upcoming *int
	if in.Upcoming != nil {
		if *in.Upcoming < 0 {
			return CoachCycle{}, fmt.Errorf("%w: upcoming must be zero or greater", ErrInvalidInput)
		}
		value := *in.Upcoming
		upcoming = &value
	}
	return CoachCycle{Repeat: repeat, Weekdays: days, Upcoming: upcoming}, nil
}

func normalizeDay(in CoachDayInput, fallbackOrder int) (CoachDay, error) {
	label := strings.TrimSpace(in.Label)
	if label == "" {
		return CoachDay{}, fmt.Errorf("%w: day label is required", ErrInvalidInput)
	}
	order := in.Order
	if order == 0 {
		order = fallbackOrder
	}
	if order < 1 {
		return CoachDay{}, fmt.Errorf("%w: day order must be at least 1", ErrInvalidInput)
	}
	week := 1
	if in.Week != nil {
		if *in.Week < 1 {
			return CoachDay{}, fmt.Errorf("%w: day week must be at least 1", ErrInvalidInput)
		}
		week = *in.Week
	}
	var weekday *int
	if in.Weekday != nil {
		if *in.Weekday < 1 || *in.Weekday > 7 {
			return CoachDay{}, fmt.Errorf("%w: day weekday must be 1 through 7", ErrInvalidInput)
		}
		value := *in.Weekday
		weekday = &value
	}
	var minutes *int
	if in.EstimatedMinutes != nil {
		if *in.EstimatedMinutes < 0 {
			return CoachDay{}, fmt.Errorf("%w: estimated_minutes must be zero or greater", ErrInvalidInput)
		}
		value := *in.EstimatedMinutes
		minutes = &value
	}
	if in.Instructions != nil && len(*in.Instructions) > 100000 {
		return CoachDay{}, fmt.Errorf("%w: instructions are too long", ErrInvalidInput)
	}
	if len(in.Blocks) == 0 {
		return CoachDay{}, fmt.Errorf("%w: day %q needs a block", ErrInvalidInput, label)
	}
	equipment := make([]string, 0, len(in.Equipment))
	for _, item := range in.Equipment {
		item = strings.TrimSpace(item)
		if item != "" {
			equipment = append(equipment, item)
		}
	}
	blocks := make([]CoachBlock, 0, len(in.Blocks))
	seen := map[int]struct{}{}
	count := 0
	for i, block := range in.Blocks {
		normalized, err := normalizeBlock(block, i+1)
		if err != nil {
			return CoachDay{}, err
		}
		if _, ok := seen[normalized.Order]; ok {
			return CoachDay{}, fmt.Errorf("%w: block order %d is repeated", ErrInvalidInput, normalized.Order)
		}
		seen[normalized.Order] = struct{}{}
		count += len(normalized.Exercises)
		blocks = append(blocks, normalized)
	}
	sortBlocks(blocks)
	if in.ExerciseCount != nil && *in.ExerciseCount != count {
		return CoachDay{}, fmt.Errorf("%w: exercise_count is %d, day has %d exercises", ErrInvalidInput, *in.ExerciseCount, count)
	}
	var kind *string
	if in.Kind != nil {
		value := strings.TrimSpace(*in.Kind)
		if value != "" {
			kind = &value
		}
	}
	var instructions *string
	if in.Instructions != nil {
		value := *in.Instructions
		instructions = &value
	}
	return CoachDay{
		Label: label, Order: order, Week: week, Weekday: weekday, Kind: kind,
		EstimatedMinutes: minutes, ExerciseCount: count, Equipment: equipment,
		Instructions: instructions, Blocks: blocks,
	}, nil
}

func normalizeBlock(in CoachBlockInput, fallbackOrder int) (CoachBlock, error) {
	blockType := strings.ToLower(strings.TrimSpace(in.Type))
	if blockType != BlockStraight && blockType != BlockSuperset && blockType != BlockCircuit {
		return CoachBlock{}, fmt.Errorf("%w: block type must be straight, superset, or circuit", ErrInvalidInput)
	}
	order := in.Order
	if order == 0 {
		order = fallbackOrder
	}
	if order < 1 {
		return CoachBlock{}, fmt.Errorf("%w: block order must be at least 1", ErrInvalidInput)
	}
	rounds, err := positive(in.Rounds, "rounds")
	if err != nil {
		return CoachBlock{}, err
	}
	sets, err := positive(in.Sets, "sets")
	if err != nil {
		return CoachBlock{}, err
	}
	restSeconds, restText, err := normalizeRest(in.RestSeconds, in.Rest)
	if err != nil {
		return CoachBlock{}, err
	}
	if len(in.Exercises) == 0 {
		return CoachBlock{}, fmt.Errorf("%w: a block needs an exercise", ErrInvalidInput)
	}
	exercises := make([]CoachBlockExercise, 0, len(in.Exercises))
	seen := map[int]struct{}{}
	for i, exercise := range in.Exercises {
		normalized, err := normalizeExercise(exercise, i+1, in.NonWorking)
		if err != nil {
			return CoachBlock{}, err
		}
		if _, ok := seen[normalized.Order]; ok {
			return CoachBlock{}, fmt.Errorf("%w: exercise order %d is repeated", ErrInvalidInput, normalized.Order)
		}
		seen[normalized.Order] = struct{}{}
		exercises = append(exercises, normalized)
	}
	sortExercises(exercises)
	return CoachBlock{
		Order: order, Type: blockType, Rounds: rounds, Sets: sets,
		RestSeconds: restSeconds, Rest: restText, NonWorking: in.NonWorking, Exercises: exercises,
	}, nil
}

func normalizeExercise(in CoachExerciseInput, fallbackOrder int, blockNonWorking bool) (CoachBlockExercise, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return CoachBlockExercise{}, fmt.Errorf("%w: exercise name is required", ErrInvalidInput)
	}
	order := in.Order
	if order == 0 {
		order = fallbackOrder
	}
	if order < 1 {
		return CoachBlockExercise{}, fmt.Errorf("%w: exercise order must be at least 1", ErrInvalidInput)
	}
	sets, err := positive(in.Sets, "sets")
	if err != nil {
		return CoachBlockExercise{}, err
	}
	repsMin, repsMax, err := normalizeRange(in.RepsMin, in.RepsMax, "reps", false)
	if err != nil {
		return CoachBlockExercise{}, err
	}
	rirMin, rirMax, err := normalizeRange(in.RIRMin, in.RIRMax, "rir", true)
	if err != nil {
		return CoachBlockExercise{}, err
	}
	restSeconds, restText, err := normalizeRest(in.RestSeconds, in.Rest)
	if err != nil {
		return CoachBlockExercise{}, err
	}
	if in.Notes != nil && len(*in.Notes) > 100000 {
		return CoachBlockExercise{}, fmt.Errorf("%w: notes are too long", ErrInvalidInput)
	}
	var notes *string
	if in.Notes != nil {
		value := *in.Notes
		notes = &value
	}
	nonWorking := blockNonWorking
	if in.NonWorking != nil {
		nonWorking = *in.NonWorking
	}
	return CoachBlockExercise{
		Name: name, Order: order, Sets: sets, RepsMin: repsMin, RepsMax: repsMax,
		RIRMin: rirMin, RIRMax: rirMax, PerSide: in.PerSide,
		RestSeconds: restSeconds, Rest: restText, Notes: notes, NonWorking: nonWorking,
	}, nil
}

func positive(v *int, name string) (*int, error) {
	if v == nil {
		return nil, nil
	}
	if *v < 1 {
		return nil, fmt.Errorf("%w: %s must be at least 1", ErrInvalidInput, name)
	}
	value := *v
	return &value, nil
}

func normalizeRange(min, max *int, name string, rir bool) (*int, *int, error) {
	if min == nil && max == nil {
		return nil, nil, nil
	}
	if min == nil {
		min = max
	}
	if max == nil {
		max = min
	}
	if *min < 0 || *max < 0 || *min > *max {
		return nil, nil, fmt.Errorf("%w: %s range is invalid", ErrInvalidInput, name)
	}
	if rir && (*min > 10 || *max > 10) {
		return nil, nil, fmt.Errorf("%w: rir must be between 0 and 10", ErrInvalidInput)
	}
	lo, hi := *min, *max
	return &lo, &hi, nil
}

func normalizeRest(seconds *int, text *string) (*int, *string, error) {
	var label *string
	if text != nil {
		value := *text
		if strings.TrimSpace(value) != "" {
			label = &value
		}
	}
	var parsed *int
	if label != nil {
		n, err := parseRest(*label)
		if err != nil {
			return nil, nil, err
		}
		parsed = &n
	}
	if seconds != nil {
		if *seconds < 0 {
			return nil, nil, fmt.Errorf("%w: rest_seconds must be zero or greater", ErrInvalidInput)
		}
		value := *seconds
		if parsed != nil && *parsed != value {
			return nil, nil, fmt.Errorf("%w: rest and rest_seconds disagree", ErrInvalidInput)
		}
		parsed = &value
	}
	return parsed, label, nil
}

func parseRest(raw string) (int, error) {
	s := strings.ToLower(strings.TrimSpace(raw))
	mult := 1
	switch {
	case strings.HasSuffix(s, "m"):
		mult = 60
		s = strings.TrimSuffix(s, "m")
	case strings.HasSuffix(s, "s"):
		s = strings.TrimSuffix(s, "s")
	default:
		return 0, fmt.Errorf("%w: rest must be a number of seconds (90s) or minutes (2m)", ErrInvalidInput)
	}
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 0 {
		return 0, fmt.Errorf("%w: rest must be a number of seconds (90s) or minutes (2m)", ErrInvalidInput)
	}
	return n * mult, nil
}

func parseISODate(s string) (string, error) {
	if len(s) != 10 || s[4] != '-' || s[7] != '-' {
		return "", fmt.Errorf("invalid date")
	}
	if _, err := strconv.Atoi(s[0:4]); err != nil {
		return "", err
	}
	month, err := strconv.Atoi(s[5:7])
	if err != nil || month < 1 || month > 12 {
		return "", fmt.Errorf("invalid date")
	}
	day, err := strconv.Atoi(s[8:10])
	if err != nil || day < 1 || day > 31 {
		return "", fmt.Errorf("invalid date")
	}
	return s, nil
}

func sortDays(days []CoachDay) {
	for i := 1; i < len(days); i++ {
		for j := i; j > 0 && dayLess(days[j], days[j-1]); j-- {
			days[j], days[j-1] = days[j-1], days[j]
		}
	}
}

func dayLess(a, b CoachDay) bool {
	if a.Week != b.Week {
		return a.Week < b.Week
	}
	return a.Order < b.Order
}

func sortBlocks(blocks []CoachBlock) {
	for i := 1; i < len(blocks); i++ {
		for j := i; j > 0 && blocks[j].Order < blocks[j-1].Order; j-- {
			blocks[j], blocks[j-1] = blocks[j-1], blocks[j]
		}
	}
}

func sortExercises(exercises []CoachBlockExercise) {
	for i := 1; i < len(exercises); i++ {
		for j := i; j > 0 && exercises[j].Order < exercises[j-1].Order; j-- {
			exercises[j], exercises[j-1] = exercises[j-1], exercises[j]
		}
	}
}

// MatchProgramExercises reports which exercise names already exist, by
// exact name, exact alias, or matching key (ADR 026), and suggests existing
// exercises for names that would be created. It writes nothing.
func MatchProgramExercises(ctx context.Context, db *sql.DB, program CoachProgram) (*ProgramExerciseMatches, error) {
	idx, err := loadExerciseIndex(ctx, db)
	if err != nil {
		return nil, err
	}
	out := &ProgramExerciseMatches{Matched: []ExerciseMatch{}, WouldCreate: []ExerciseCreate{}, Ambiguous: []AmbiguousExerciseError{}}
	seen := map[string]struct{}{}
	for _, day := range program.Days {
		for _, block := range day.Blocks {
			for _, exercise := range block.Exercises {
				key := strings.ToLower(exercise.Name)
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}
				ref, how, ok, findErr := idx.resolve(exercise.Name)
				var ambiguous *AmbiguousExerciseError
				if errors.As(findErr, &ambiguous) {
					out.Ambiguous = append(out.Ambiguous, *ambiguous)
					continue
				}
				if findErr != nil {
					return nil, findErr
				}
				if ok {
					out.Matched = append(out.Matched, ExerciseMatch{Name: exercise.Name, ExerciseID: ref.ID, ExerciseName: ref.Name, MatchedOn: how})
					continue
				}
				out.WouldCreate = append(out.WouldCreate, ExerciseCreate{Name: exercise.Name, Suggestions: idx.suggest(exercise.Name)})
			}
		}
	}
	return out, nil
}

// ImportCoachProgram stores program as a new locked version for athleteID.
// Missing exercises are created in the same transaction. An existing name
// becomes the next version and the previous row is left as it was.
func ImportCoachProgram(ctx context.Context, db *sql.DB, athleteID, coachUserID int64, sourceDate string, program CoachProgram) (*CoachImportResult, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("models: import program begin: %w", err)
	}
	defer tx.Rollback()

	created := []string{}
	ids := map[string]int64{}
	idx, err := loadExerciseIndex(ctx, tx)
	if err != nil {
		return nil, err
	}
	for _, day := range program.Days {
		for _, block := range day.Blocks {
			for _, exercise := range block.Exercises {
				key := strings.ToLower(exercise.Name)
				if _, ok := ids[key]; ok {
					continue
				}
				ref, _, found, findErr := idx.resolve(exercise.Name)
				if findErr != nil {
					return nil, findErr
				}
				id := ref.ID
				if !found {
					id, findErr = insertCatalogExercise(ctx, tx, CoachExerciseDraft{Name: exercise.Name})
					if findErr != nil {
						return nil, findErr
					}
					created = append(created, exercise.Name)
					idx.add(indexedLabel{exercise: ExerciseRef{ID: id, Name: exercise.Name}, label: exercise.Name})
				}
				ids[key] = id
			}
		}
	}

	lineage, version, err := nextProgramVersion(ctx, tx, athleteID, program.Name)
	if err != nil {
		return nil, err
	}
	weeks := 1
	if program.Weeks != nil {
		weeks = *program.Weeks
	}
	isLoop := 0
	if program.Cycle != nil && program.Cycle.Repeat == cycleWeekly {
		isLoop = 1
	}
	var lineageVal any
	if lineage != nil {
		lineageVal = *lineage
	}
	var templateID int64
	err = tx.QueryRowContext(ctx,
		`INSERT INTO program_templates
		    (athlete_id, name, num_weeks, num_days, is_loop, status, lineage_id, version)
		 VALUES (?, ?, ?, ?, ?, 'approved', ?, ?) RETURNING id`,
		athleteID, program.Name, weeks, len(program.Days), isLoop, lineageVal, version,
	).Scan(&templateID)
	if err != nil {
		return nil, fmt.Errorf("models: import program template: %w", err)
	}

	var repeat any
	var weekdays any
	var upcoming any
	var start any
	if program.Cycle != nil {
		repeat = program.Cycle.Repeat
		encoded, encErr := json.Marshal(program.Cycle.Weekdays)
		if encErr != nil {
			return nil, encErr
		}
		weekdays = string(encoded)
		if program.Cycle.Upcoming != nil {
			upcoming = *program.Cycle.Upcoming
		}
	}
	if program.StartDate != nil {
		start = *program.StartDate
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO program_imports
		    (template_id, source_name, author, repeat_cycle, schedule_weekdays, upcoming_count, locked, start_date)
		 VALUES (?, ?, ?, ?, ?, ?, 1, ?)`,
		templateID, program.Source, program.Author, repeat, weekdays, upcoming, start,
	); err != nil {
		return nil, fmt.Errorf("models: import program metadata: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO program_sources (template_id, coach_id, source_date, source_text) VALUES (?, ?, ?, ?)`,
		templateID, coachUserID, sourceDate, program.Source,
	); err != nil {
		return nil, fmt.Errorf("models: import program source: %w", err)
	}

	if err := insertProgramDays(ctx, tx, templateID, program, ids); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("models: import program commit: %w", err)
	}
	stored, err := LoadImportedProgram(ctx, db, templateID)
	if err != nil {
		return nil, err
	}
	if created == nil {
		created = []string{}
	}
	return &CoachImportResult{Program: *stored, CreatedExercises: created}, nil
}

func nextProgramVersion(ctx context.Context, q queryer, athleteID int64, name string) (*int64, int, error) {
	var id int64
	var lineage sql.NullInt64
	var version int
	err := q.QueryRowContext(ctx,
		`SELECT id, lineage_id, version FROM program_templates
		  WHERE athlete_id = ? AND name = ? AND status = 'approved'
		  ORDER BY version DESC LIMIT 1`,
		athleteID, name,
	).Scan(&id, &lineage, &version)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, 1, nil
	}
	if err != nil {
		return nil, 0, fmt.Errorf("models: lookup program version: %w", err)
	}
	root := id
	if lineage.Valid {
		root = lineage.Int64
	}
	var maxVersion int
	if err := q.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(version), 0) FROM program_templates WHERE COALESCE(lineage_id, id) = ?`,
		root,
	).Scan(&maxVersion); err != nil {
		return nil, 0, fmt.Errorf("models: max program version: %w", err)
	}
	return &root, maxVersion + 1, nil
}

func insertProgramDays(ctx context.Context, tx *sql.Tx, templateID int64, program CoachProgram, ids map[string]int64) error {
	for _, day := range program.Days {
		equipment, err := json.Marshal(day.Equipment)
		if err != nil {
			return err
		}
		var dayID int64
		err = tx.QueryRowContext(ctx,
			`INSERT INTO program_days
			    (template_id, week, day_number, day_order, label, weekday, kind, estimated_minutes, exercise_count, equipment_json, instructions)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
			templateID, day.Week, day.Number, day.Order, day.Label, sqlInt(day.Weekday), sqlText(day.Kind),
			sqlInt(day.EstimatedMinutes), day.ExerciseCount, string(equipment), sqlText(day.Instructions),
		).Scan(&dayID)
		if err != nil {
			return fmt.Errorf("models: import program day: %w", err)
		}
		sort := 0
		grouped := 0
		for _, block := range day.Blocks {
			groupKey := ""
			if block.Type == BlockSuperset || block.Type == BlockCircuit {
				groupKey = GroupKeyForIndex(grouped)
				grouped++
			}
			var blockID int64
			err = tx.QueryRowContext(ctx,
				`INSERT INTO program_blocks
				    (day_id, block_order, block_type, rounds, sets, rest_seconds, rest_text, non_working)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
				dayID, block.Order, block.Type, sqlInt(block.Rounds), sqlInt(block.Sets),
				sqlInt(block.RestSeconds), sqlText(block.Rest), bit(block.NonWorking),
			).Scan(&blockID)
			if err != nil {
				return fmt.Errorf("models: import program block: %w", err)
			}
			for _, exercise := range block.Exercises {
				exerciseID := ids[strings.ToLower(exercise.Name)]
				var lineID int64
				err = tx.QueryRowContext(ctx,
					`INSERT INTO program_block_exercises
					    (block_id, exercise_id, sort_order, sets, reps_min, reps_max, rir_min, rir_max, per_side, rest_seconds, rest_text, notes, non_working)
					 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
					blockID, exerciseID, exercise.Order, sqlInt(exercise.Sets), sqlInt(exercise.RepsMin), sqlInt(exercise.RepsMax),
					sqlInt(exercise.RIRMin), sqlInt(exercise.RIRMax), bit(exercise.PerSide), sqlInt(exercise.RestSeconds),
					sqlText(exercise.Rest), sqlText(exercise.Notes), bit(exercise.NonWorking),
				).Scan(&lineID)
				if err != nil {
					return fmt.Errorf("models: import program exercise: %w", err)
				}
				if err := expandPrescribedSets(ctx, tx, templateID, exerciseID, day, block, exercise, sort, groupKey); err != nil {
					return err
				}
				sort += 10
			}
		}
	}
	return nil
}

// expandPrescribedSets writes one prescribed_sets row per set of an
// imported exercise line. reps is the bottom of the coach's rep range and
// reps_max the top (when it differs); the RIR range, the warm-up flag,
// and the block's group key (A, B, ... for supersets and circuits) carry
// over so the prescription read shows what the coach wrote.
func expandPrescribedSets(ctx context.Context, tx *sql.Tx, templateID, exerciseID int64, day CoachDay, block CoachBlock, exercise CoachBlockExercise, sort int, groupKey string) error {
	n := 1
	switch {
	case exercise.Sets != nil:
		n = *exercise.Sets
	case block.Sets != nil:
		n = *block.Sets
	case block.Rounds != nil:
		n = *block.Rounds
	}
	repType := "reps"
	if exercise.PerSide {
		repType = "each_side"
	}
	var reps *int
	if exercise.RepsMin != nil {
		value := *exercise.RepsMin
		reps = &value
	}
	detail := PrescribedSetDetail{
		RIRMin:     exercise.RIRMin,
		RIRMax:     exercise.RIRMax,
		NonWorking: exercise.NonWorking,
		GroupKey:   groupKey,
	}
	if exercise.RepsMax != nil && (reps == nil || *exercise.RepsMax > *reps) {
		value := *exercise.RepsMax
		detail.RepsMax = &value
	}
	rest := exercise.RestSeconds
	if rest == nil {
		rest = block.RestSeconds
	}
	notes := ""
	if exercise.Notes != nil {
		notes = *exercise.Notes
	}
	for setNumber := 1; setNumber <= n; setNumber++ {
		if _, err := insertPrescribedSetRow(ctx, tx, templateID, exerciseID, day.Week, day.Number, setNumber, reps, nil, nil, rest, sort, repType, notes, nil, block.Type, detail); err != nil {
			return err
		}
	}
	return nil
}

// LoadImportedProgram reads one imported template, including days and AI fields.
func LoadImportedProgram(ctx context.Context, db *sql.DB, templateID int64) (*StoredCoachProgram, error) {
	row := db.QueryRowContext(ctx, `
		SELECT pt.id, pt.athlete_id, pt.name, pt.version, pt.num_weeks, pt.lineage_id,
		       pi.source_name, pi.author, pi.repeat_cycle, pi.schedule_weekdays, pi.upcoming_count, pi.locked, pi.start_date,
		       ap.id, ap.start_date
		  FROM program_templates pt
		  JOIN program_imports pi ON pi.template_id = pt.id
		  LEFT JOIN athlete_programs ap ON ap.template_id = pt.id AND ap.athlete_id = pt.athlete_id AND ap.active = 1
		 WHERE pt.id = ?`, templateID)
	stored, err := scanStoredProgram(row)
	if err != nil {
		return nil, err
	}
	days, err := loadProgramDays(ctx, db, templateID)
	if err != nil {
		return nil, err
	}
	stored.Days = days
	return stored, nil
}

// ListImportedPrograms returns every imported version for an athlete, oldest version first.
func ListImportedPrograms(ctx context.Context, db *sql.DB, athleteID int64) ([]StoredCoachProgram, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT pt.id
		  FROM program_templates pt
		  JOIN program_imports pi ON pi.template_id = pt.id
		 WHERE pt.athlete_id = ?
		 ORDER BY pt.name COLLATE NOCASE, pt.version, pt.id`, athleteID)
	if err != nil {
		return nil, fmt.Errorf("models: list imported programs: %w", err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]StoredCoachProgram, 0, len(ids))
	for _, id := range ids {
		stored, err := LoadImportedProgram(ctx, db, id)
		if err != nil {
			return nil, err
		}
		out = append(out, *stored)
	}
	return out, nil
}

// LoadImportedDay returns the stored day for a template week and prescription day.
// ok is false when this template has no imported day there.
func LoadImportedDay(ctx context.Context, db *sql.DB, templateID int64, week, day int) (*CoachDay, *CoachCycle, error) {
	var dayID int64
	err := db.QueryRowContext(ctx,
		`SELECT id FROM program_days WHERE template_id = ? AND week = ? AND day_number = ?`,
		templateID, week, day,
	).Scan(&dayID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("models: load imported day: %w", err)
	}
	days, err := loadProgramDays(ctx, db, templateID)
	if err != nil {
		return nil, nil, err
	}
	var found *CoachDay
	for i := range days {
		if days[i].Week == week && days[i].Number == day {
			found = &days[i]
			break
		}
	}
	if found == nil {
		return nil, nil, nil
	}
	cycle, err := loadCycle(ctx, db, templateID)
	if err != nil {
		return nil, nil, err
	}
	return found, cycle, nil
}

func loadCycle(ctx context.Context, db *sql.DB, templateID int64) (*CoachCycle, error) {
	var repeat sql.NullString
	var weekdays sql.NullString
	var upcoming sql.NullInt64
	err := db.QueryRowContext(ctx,
		`SELECT repeat_cycle, schedule_weekdays, upcoming_count FROM program_imports WHERE template_id = ?`,
		templateID,
	).Scan(&repeat, &weekdays, &upcoming)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("models: load program cycle: %w", err)
	}
	if !repeat.Valid && !weekdays.Valid && !upcoming.Valid {
		return nil, nil
	}
	cycle := &CoachCycle{}
	if repeat.Valid {
		cycle.Repeat = repeat.String
	}
	if weekdays.Valid && weekdays.String != "" {
		if err := json.Unmarshal([]byte(weekdays.String), &cycle.Weekdays); err != nil {
			return nil, fmt.Errorf("models: parse program weekdays: %w", err)
		}
	}
	if cycle.Weekdays == nil {
		cycle.Weekdays = []int{}
	}
	if upcoming.Valid {
		value := int(upcoming.Int64)
		cycle.Upcoming = &value
	}
	return cycle, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanStoredProgram(row rowScanner) (*StoredCoachProgram, error) {
	var stored StoredCoachProgram
	var athleteID sql.NullInt64
	var lineage sql.NullInt64
	var repeat sql.NullString
	var weekdays sql.NullString
	var upcoming sql.NullInt64
	var locked int
	var start sql.NullString
	var assignmentID sql.NullInt64
	var assignmentStart sql.NullString
	err := row.Scan(&stored.TemplateID, &athleteID, &stored.Name, &stored.Version, &stored.NumWeeks, &lineage,
		&stored.Source, &stored.Author, &repeat, &weekdays, &upcoming, &locked, &start,
		&assignmentID, &assignmentStart)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("models: scan imported program: %w", err)
	}
	if athleteID.Valid {
		stored.AthleteID = athleteID.Int64
	}
	if lineage.Valid {
		stored.LineageID = &lineage.Int64
	}
	stored.Locked = locked == 1
	if start.Valid {
		value := isoDate(start.String)
		stored.StartDate = &value
	}
	if assignmentID.Valid {
		stored.AssignmentID = &assignmentID.Int64
	}
	if assignmentStart.Valid {
		value := isoDate(assignmentStart.String)
		stored.AssignmentStart = &value
	}
	if repeat.Valid || weekdays.Valid || upcoming.Valid {
		cycle := &CoachCycle{}
		if repeat.Valid {
			cycle.Repeat = repeat.String
		}
		if weekdays.Valid && weekdays.String != "" {
			if err := json.Unmarshal([]byte(weekdays.String), &cycle.Weekdays); err != nil {
				return nil, fmt.Errorf("models: parse program weekdays: %w", err)
			}
		}
		if cycle.Weekdays == nil {
			cycle.Weekdays = []int{}
		}
		if upcoming.Valid {
			value := int(upcoming.Int64)
			cycle.Upcoming = &value
		}
		stored.Cycle = cycle
	}
	return &stored, nil
}

func loadProgramDays(ctx context.Context, db *sql.DB, templateID int64) ([]CoachDay, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, week, day_number, day_order, label, weekday, kind, estimated_minutes, exercise_count, equipment_json, instructions
		  FROM program_days
		 WHERE template_id = ?
		 ORDER BY week, day_number`, templateID)
	if err != nil {
		return nil, fmt.Errorf("models: list program days: %w", err)
	}
	defer rows.Close()
	var days []CoachDay
	var dayIDs []int64
	for rows.Next() {
		var id int64
		var day CoachDay
		var weekday, minutes sql.NullInt64
		var kind, instructions, equipment sql.NullString
		if err := rows.Scan(&id, &day.Week, &day.Number, &day.Order, &day.Label, &weekday, &kind, &minutes, &day.ExerciseCount, &equipment, &instructions); err != nil {
			return nil, err
		}
		if weekday.Valid {
			value := int(weekday.Int64)
			day.Weekday = &value
		}
		if kind.Valid {
			day.Kind = &kind.String
		}
		if minutes.Valid {
			value := int(minutes.Int64)
			day.EstimatedMinutes = &value
		}
		if instructions.Valid {
			day.Instructions = &instructions.String
		}
		day.Equipment = []string{}
		if equipment.Valid && equipment.String != "" {
			if err := json.Unmarshal([]byte(equipment.String), &day.Equipment); err != nil {
				return nil, fmt.Errorf("models: parse day equipment: %w", err)
			}
		}
		day.Blocks = []CoachBlock{}
		days = append(days, day)
		dayIDs = append(dayIDs, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(days) == 0 {
		return []CoachDay{}, nil
	}
	blocks, err := loadBlocks(ctx, db, dayIDs)
	if err != nil {
		return nil, err
	}
	for i, id := range dayIDs {
		days[i].Blocks = blocks[id]
		if days[i].Blocks == nil {
			days[i].Blocks = []CoachBlock{}
		}
	}
	return days, nil
}

func loadBlocks(ctx context.Context, db *sql.DB, dayIDs []int64) (map[int64][]CoachBlock, error) {
	query := `
		SELECT id, day_id, block_order, block_type, rounds, sets, rest_seconds, rest_text, non_working
		  FROM program_blocks
		 WHERE day_id IN (` + inClause(len(dayIDs)) + `)
		 ORDER BY day_id, block_order`
	args := make([]any, len(dayIDs))
	for i, id := range dayIDs {
		args[i] = id
	}
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("models: list program blocks: %w", err)
	}
	defer rows.Close()
	out := map[int64][]CoachBlock{}
	var blockIDs []int64
	index := map[int64]struct {
		day int64
		pos int
	}{}
	for rows.Next() {
		var id, dayID int64
		var block CoachBlock
		var rounds, sets, rest sql.NullInt64
		var restText sql.NullString
		var nonWorking int
		if err := rows.Scan(&id, &dayID, &block.Order, &block.Type, &rounds, &sets, &rest, &restText, &nonWorking); err != nil {
			return nil, err
		}
		block.Rounds = intFromSQL(rounds)
		block.Sets = intFromSQL(sets)
		block.RestSeconds = intFromSQL(rest)
		if restText.Valid {
			block.Rest = &restText.String
		}
		block.NonWorking = nonWorking == 1
		block.Exercises = []CoachBlockExercise{}
		out[dayID] = append(out[dayID], block)
		index[id] = struct {
			day int64
			pos int
		}{day: dayID, pos: len(out[dayID]) - 1}
		blockIDs = append(blockIDs, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(blockIDs) == 0 {
		return out, nil
	}
	lines, err := loadBlockExercises(ctx, db, blockIDs)
	if err != nil {
		return nil, err
	}
	for id, exercises := range lines {
		at := index[id]
		out[at.day][at.pos].Exercises = exercises
	}
	return out, nil
}

func loadBlockExercises(ctx context.Context, db *sql.DB, blockIDs []int64) (map[int64][]CoachBlockExercise, error) {
	query := `
		SELECT pbe.id, pbe.block_id, pbe.exercise_id, e.name, pbe.sort_order, pbe.sets, pbe.reps_min, pbe.reps_max,
		       pbe.rir_min, pbe.rir_max, pbe.per_side, pbe.rest_seconds, pbe.rest_text, pbe.notes, pbe.non_working,
		       pbe.ai_sets, pbe.ai_reps_min, pbe.ai_reps_max, pbe.ai_rir_min, pbe.ai_rir_max, pbe.ai_rest_seconds, pbe.ai_notes, pbe.ai_load
		  FROM program_block_exercises pbe
		  JOIN exercises e ON e.id = pbe.exercise_id
		 WHERE pbe.block_id IN (` + inClause(len(blockIDs)) + `)
		 ORDER BY pbe.block_id, pbe.sort_order`
	args := make([]any, len(blockIDs))
	for i, id := range blockIDs {
		args[i] = id
	}
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("models: list program exercises: %w", err)
	}
	defer rows.Close()
	out := map[int64][]CoachBlockExercise{}
	for rows.Next() {
		var blockID int64
		var exercise CoachBlockExercise
		var sets, repsMin, repsMax, rirMin, rirMax, rest sql.NullInt64
		var perSide, nonWorking int
		var restText, notes sql.NullString
		var aiSets, aiRepsMin, aiRepsMax, aiRIRMin, aiRIRMax, aiRest sql.NullInt64
		var aiNotes sql.NullString
		var aiLoad sql.NullFloat64
		if err := rows.Scan(&exercise.ID, &blockID, &exercise.ExerciseID, &exercise.Name, &exercise.Order,
			&sets, &repsMin, &repsMax, &rirMin, &rirMax, &perSide, &rest, &restText, &notes, &nonWorking,
			&aiSets, &aiRepsMin, &aiRepsMax, &aiRIRMin, &aiRIRMax, &aiRest, &aiNotes, &aiLoad); err != nil {
			return nil, err
		}
		exercise.Sets = intFromSQL(sets)
		exercise.RepsMin = intFromSQL(repsMin)
		exercise.RepsMax = intFromSQL(repsMax)
		exercise.RIRMin = intFromSQL(rirMin)
		exercise.RIRMax = intFromSQL(rirMax)
		exercise.PerSide = perSide == 1
		exercise.RestSeconds = intFromSQL(rest)
		if restText.Valid {
			exercise.Rest = &restText.String
		}
		if notes.Valid {
			exercise.Notes = &notes.String
		}
		exercise.NonWorking = nonWorking == 1
		if ai := aiFields(aiSets, aiRepsMin, aiRepsMax, aiRIRMin, aiRIRMax, aiRest, aiNotes, aiLoad); ai != nil {
			exercise.AI = ai
		}
		out[blockID] = append(out[blockID], exercise)
	}
	return out, rows.Err()
}

func aiFields(sets, repsMin, repsMax, rirMin, rirMax, rest sql.NullInt64, notes sql.NullString, load sql.NullFloat64) *CoachAIFields {
	if !sets.Valid && !repsMin.Valid && !repsMax.Valid && !rirMin.Valid && !rirMax.Valid && !rest.Valid && !notes.Valid && !load.Valid {
		return nil
	}
	ai := &CoachAIFields{
		Sets: intFromSQL(sets), RepsMin: intFromSQL(repsMin), RepsMax: intFromSQL(repsMax),
		RIRMin: intFromSQL(rirMin), RIRMax: intFromSQL(rirMax), RestSeconds: intFromSQL(rest),
	}
	if notes.Valid {
		ai.Notes = &notes.String
	}
	if load.Valid {
		value := load.Float64
		ai.Load = &value
	}
	return ai
}

// ProgramIsLocked reports whether templateID is a locked import.
// A template with no import row is not locked.
func ProgramIsLocked(ctx context.Context, db *sql.DB, templateID int64) (bool, error) {
	var locked int
	err := db.QueryRowContext(ctx, `SELECT locked FROM program_imports WHERE template_id = ?`, templateID).Scan(&locked)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("models: program lock %d: %w", templateID, err)
	}
	return locked == 1, nil
}

// TemplateIDForPrescribedSet returns the template that owns a prescribed set.
func TemplateIDForPrescribedSet(ctx context.Context, db *sql.DB, setID int64) (int64, error) {
	var templateID int64
	err := db.QueryRowContext(ctx, `SELECT template_id FROM prescribed_sets WHERE id = ?`, setID).Scan(&templateID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("models: template for set %d: %w", setID, err)
	}
	return templateID, nil
}

// AssignImportedProgram assigns one version of an imported template.
// The assignment uses the template's stored weekdays. An active primary is replaced.
func AssignImportedProgram(ctx context.Context, db *sql.DB, athleteID, templateID int64, version int, startDate string) (*AthleteProgram, *CoachCycle, error) {
	var owner sql.NullInt64
	var gotVersion int
	var status string
	err := db.QueryRowContext(ctx,
		`SELECT athlete_id, version, status FROM program_templates WHERE id = ?`, templateID,
	).Scan(&owner, &gotVersion, &status)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && (!owner.Valid || owner.Int64 != athleteID)) {
		return nil, nil, ErrNotFound
	}
	if err != nil {
		return nil, nil, fmt.Errorf("models: assign imported program lookup: %w", err)
	}
	if status != ProgramStatusApproved {
		return nil, nil, ErrNotFound
	}
	if gotVersion != version {
		return nil, nil, ErrVersionMismatch
	}
	cycle, err := loadCycle(ctx, db, templateID)
	if err != nil {
		return nil, nil, err
	}
	if cycle == nil {
		return nil, nil, ErrNotFound
	}
	schedule := ""
	if len(cycle.Weekdays) > 0 {
		encoded, encErr := json.Marshal(cycle.Weekdays)
		if encErr != nil {
			return nil, nil, encErr
		}
		schedule = string(encoded)
	}
	assigned, err := ReplaceProgram(ctx, db, athleteID, templateID, startDate, "", "", "primary", schedule)
	if err != nil {
		return nil, nil, err
	}
	return assigned, cycle, nil
}

// GapFillCoachProgram writes AI suggestions for fields the coach left blank.
// A field the coach set is refused and nothing is written.
func GapFillCoachProgram(ctx context.Context, db *sql.DB, athleteID, templateID int64, in GapFillInput) (*CoachBlockExercise, error) {
	if in.ExerciseName != nil {
		return nil, &CoachFieldError{Field: "exercise"}
	}
	if !gapFillHasField(in) {
		return nil, fmt.Errorf("%w: nothing to fill", ErrInvalidInput)
	}
	var lineID, exerciseID int64
	var sets, repsMin, repsMax, rirMin, rirMax, rest, blockRest sql.NullInt64
	var notes sql.NullString
	err := db.QueryRowContext(ctx, `
		SELECT pbe.id, pbe.exercise_id, pbe.sets, pbe.reps_min, pbe.reps_max, pbe.rir_min, pbe.rir_max,
		       pbe.rest_seconds, pbe.notes, pb.rest_seconds
		  FROM program_block_exercises pbe
		  JOIN program_blocks pb ON pb.id = pbe.block_id
		  JOIN program_days pd ON pd.id = pb.day_id
		  JOIN program_templates pt ON pt.id = pd.template_id
		  JOIN program_imports pi ON pi.template_id = pt.id AND pi.locked = 1
		 WHERE pt.id = ? AND pt.athlete_id = ? AND pd.day_order = ? AND pb.block_order = ? AND pbe.sort_order = ?`,
		templateID, athleteID, in.DayOrder, in.BlockOrder, in.ExerciseOrder,
	).Scan(&lineID, &exerciseID, &sets, &repsMin, &repsMax, &rirMin, &rirMax, &rest, &notes, &blockRest)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("models: gap-fill lookup: %w", err)
	}
	if in.Sets != nil && sets.Valid {
		return nil, &CoachFieldError{Field: "sets"}
	}
	if (in.RepsMin != nil && repsMin.Valid) || (in.RepsMax != nil && repsMax.Valid) {
		field := "reps_min"
		if in.RepsMax != nil && repsMax.Valid {
			field = "reps_max"
		}
		return nil, &CoachFieldError{Field: field}
	}
	if (in.RIRMin != nil && rirMin.Valid) || (in.RIRMax != nil && rirMax.Valid) {
		field := "rir_min"
		if in.RIRMax != nil && rirMax.Valid {
			field = "rir_max"
		}
		return nil, &CoachFieldError{Field: field}
	}
	if in.RestSeconds != nil && (rest.Valid || blockRest.Valid) {
		return nil, &CoachFieldError{Field: "rest_seconds"}
	}
	if in.Notes != nil && notes.Valid {
		return nil, &CoachFieldError{Field: "notes"}
	}
	if in.Load != nil {
		var coachLoads int
		if err := db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM prescribed_sets
			  WHERE template_id = ? AND exercise_id = ? AND (percentage IS NOT NULL OR absolute_weight IS NOT NULL)`,
			templateID, exerciseID,
		).Scan(&coachLoads); err != nil {
			return nil, fmt.Errorf("models: gap-fill load check: %w", err)
		}
		if coachLoads > 0 {
			return nil, &CoachFieldError{Field: "load"}
		}
	}
	if _, err := db.ExecContext(ctx, `
		UPDATE program_block_exercises
		   SET ai_sets = COALESCE(?, ai_sets),
		       ai_reps_min = COALESCE(?, ai_reps_min),
		       ai_reps_max = COALESCE(?, ai_reps_max),
		       ai_rir_min = COALESCE(?, ai_rir_min),
		       ai_rir_max = COALESCE(?, ai_rir_max),
		       ai_rest_seconds = COALESCE(?, ai_rest_seconds),
		       ai_notes = COALESCE(?, ai_notes),
		       ai_load = COALESCE(?, ai_load)
		 WHERE id = ?`,
		sqlInt(in.Sets), sqlInt(in.RepsMin), sqlInt(in.RepsMax), sqlInt(in.RIRMin), sqlInt(in.RIRMax),
		sqlInt(in.RestSeconds), sqlText(in.Notes), sqlFloat(in.Load), lineID,
	); err != nil {
		return nil, fmt.Errorf("models: gap-fill update: %w", err)
	}
	stored, err := LoadImportedProgram(ctx, db, templateID)
	if err != nil {
		return nil, err
	}
	for _, day := range stored.Days {
		if day.Order != in.DayOrder {
			continue
		}
		for _, block := range day.Blocks {
			if block.Order != in.BlockOrder {
				continue
			}
			for i := range block.Exercises {
				if block.Exercises[i].Order == in.ExerciseOrder {
					return &block.Exercises[i], nil
				}
			}
		}
	}
	return nil, ErrNotFound
}

func gapFillHasField(in GapFillInput) bool {
	return in.Sets != nil || in.RepsMin != nil || in.RepsMax != nil || in.RIRMin != nil || in.RIRMax != nil || in.RestSeconds != nil || in.Notes != nil || in.Load != nil
}

// FindOrCreateCoachExercise returns the catalog row for draft.
// A case-insensitive name or alias match returns the existing row and created is false.
func FindOrCreateCoachExercise(ctx context.Context, db *sql.DB, draft CoachExerciseDraft) (*CoachCatalogExercise, bool, error) {
	name := strings.TrimSpace(draft.Name)
	if name == "" {
		return nil, false, fmt.Errorf("%w: name is required", ErrInvalidInput)
	}
	draft.Name = name
	idx, err := loadExerciseIndex(ctx, db)
	if err != nil {
		return nil, false, err
	}
	ids := map[int64]struct{}{}
	if ref, _, ok, err := idx.resolve(name); err != nil {
		return nil, false, err
	} else if ok {
		ids[ref.ID] = struct{}{}
	}
	for _, alias := range draft.Aliases {
		alias = strings.TrimSpace(alias)
		if alias == "" || strings.EqualFold(alias, name) {
			continue
		}
		ref, _, ok, err := idx.resolve(alias)
		if err != nil {
			return nil, false, err
		}
		if ok {
			ids[ref.ID] = struct{}{}
		}
	}
	if len(ids) > 1 {
		return nil, false, ErrExerciseAmbiguous
	}
	if len(ids) == 1 {
		var id int64
		for found := range ids {
			id = found
		}
		row, err := loadCoachExercise(ctx, db, id)
		return row, false, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback()
	id, err := insertCatalogExercise(ctx, tx, draft)
	if err != nil {
		return nil, false, err
	}
	if err := tx.Commit(); err != nil {
		return nil, false, err
	}
	row, err := loadCoachExercise(ctx, db, id)
	return row, true, err
}

// SearchCoachExercises lists exercises whose name or alias contains q.
// An empty q lists the first matches in name order.
func SearchCoachExercises(ctx context.Context, db *sql.DB, q string) ([]CoachCatalogExercise, error) {
	q = strings.TrimSpace(q)
	pattern := "%" + escapeLike(q) + "%"
	rows, err := db.QueryContext(ctx, `
		SELECT DISTINCT e.id
		  FROM exercises e
		  LEFT JOIN exercise_aliases a ON a.exercise_id = e.id
		 WHERE ? = '' OR e.name LIKE ? ESCAPE '\' OR a.alias LIKE ? ESCAPE '\'
		 ORDER BY e.name COLLATE NOCASE
		 LIMIT 50`, q, pattern, pattern)
	if err != nil {
		return nil, fmt.Errorf("models: search exercises: %w", err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]CoachCatalogExercise, 0, len(ids))
	for _, id := range ids {
		row, err := loadCoachExercise(ctx, db, id)
		if err != nil {
			return nil, err
		}
		out = append(out, *row)
	}
	return out, nil
}

func findExercise(ctx context.Context, q queryer, name string) (int64, string, bool, error) {
	idx, err := loadExerciseIndex(ctx, q)
	if err != nil {
		return 0, "", false, err
	}
	ref, how, ok, err := idx.resolve(name)
	return ref.ID, how, ok, err
}

func insertCatalogExercise(ctx context.Context, tx *sql.Tx, draft CoachExerciseDraft) (int64, error) {
	var notes any
	if draft.Notes != nil && *draft.Notes != "" {
		notes = *draft.Notes
	}
	var id int64
	err := tx.QueryRowContext(ctx,
		`INSERT INTO exercises (name, form_notes, unilateral) VALUES (?, ?, ?) RETURNING id`,
		draft.Name, notes, bit(draft.Unilateral),
	).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			found, _, ok, findErr := findExercise(ctx, tx, draft.Name)
			if findErr != nil {
				return 0, findErr
			}
			if ok {
				return found, nil
			}
		}
		return 0, fmt.Errorf("models: create exercise %q: %w", draft.Name, err)
	}
	seen := map[string]struct{}{ExerciseKey(draft.Name): {}}
	for _, alias := range draft.Aliases {
		alias = strings.TrimSpace(alias)
		key := ExerciseKey(alias)
		if alias == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		aliasIdx, err := loadExerciseIndex(ctx, tx)
		if err != nil {
			return 0, err
		}
		if c := aliasIdx.conflict(alias, id); c != nil {
			return 0, c
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO exercise_aliases (exercise_id, alias) VALUES (?, ?)`, id, alias); err != nil {
			return 0, fmt.Errorf("models: create exercise alias: %w", err)
		}
	}
	seen = map[string]struct{}{}
	for _, muscle := range draft.PrimaryMuscles {
		muscle = strings.TrimSpace(muscle)
		key := strings.ToLower(muscle)
		if muscle == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if _, err := tx.ExecContext(ctx, `INSERT INTO exercise_muscles (exercise_id, muscle) VALUES (?, ?)`, id, muscle); err != nil {
			return 0, fmt.Errorf("models: create exercise muscle: %w", err)
		}
	}
	for _, name := range draft.Equipment {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		equipmentID, err := equipmentByName(ctx, tx, name)
		if err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO exercise_equipment (exercise_id, equipment_id, optional) VALUES (?, ?, 0)`,
			id, equipmentID,
		); err != nil && !isUniqueViolation(err) {
			return 0, fmt.Errorf("models: link exercise equipment: %w", err)
		}
	}
	return id, nil
}

func equipmentByName(ctx context.Context, tx *sql.Tx, name string) (int64, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM equipment WHERE name = ? COLLATE NOCASE`, name).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	err = tx.QueryRowContext(ctx, `INSERT INTO equipment (name) VALUES (?) RETURNING id`, name).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			if scanErr := tx.QueryRowContext(ctx, `SELECT id FROM equipment WHERE name = ? COLLATE NOCASE`, name).Scan(&id); scanErr == nil {
				return id, nil
			}
		}
		return 0, fmt.Errorf("models: create equipment %q: %w", name, err)
	}
	return id, nil
}

func loadCoachExercise(ctx context.Context, db *sql.DB, id int64) (*CoachCatalogExercise, error) {
	row := &CoachCatalogExercise{Aliases: []string{}, Equipment: []string{}, PrimaryMuscles: []string{}}
	var notes sql.NullString
	var unilateral int
	err := db.QueryRowContext(ctx,
		`SELECT id, name, form_notes, unilateral FROM exercises WHERE id = ?`, id,
	).Scan(&row.ID, &row.Name, &notes, &unilateral)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("models: load exercise %d: %w", id, err)
	}
	row.Unilateral = unilateral == 1
	if notes.Valid && notes.String != "" {
		row.Notes = &notes.String
	}
	aliasRows, err := db.QueryContext(ctx, `SELECT alias FROM exercise_aliases WHERE exercise_id = ? ORDER BY alias COLLATE NOCASE`, id)
	if err != nil {
		return nil, err
	}
	defer aliasRows.Close()
	for aliasRows.Next() {
		var alias string
		if err := aliasRows.Scan(&alias); err != nil {
			return nil, err
		}
		row.Aliases = append(row.Aliases, alias)
	}
	if err := aliasRows.Err(); err != nil {
		return nil, err
	}
	muscleRows, err := db.QueryContext(ctx, `SELECT muscle FROM exercise_muscles WHERE exercise_id = ? ORDER BY muscle COLLATE NOCASE`, id)
	if err != nil {
		return nil, err
	}
	defer muscleRows.Close()
	for muscleRows.Next() {
		var muscle string
		if err := muscleRows.Scan(&muscle); err != nil {
			return nil, err
		}
		row.PrimaryMuscles = append(row.PrimaryMuscles, muscle)
	}
	if err := muscleRows.Err(); err != nil {
		return nil, err
	}
	equipRows, err := db.QueryContext(ctx, `
		SELECT eq.name FROM exercise_equipment ee
		  JOIN equipment eq ON eq.id = ee.equipment_id
		 WHERE ee.exercise_id = ?
		 ORDER BY eq.name COLLATE NOCASE`, id)
	if err != nil {
		return nil, err
	}
	defer equipRows.Close()
	for equipRows.Next() {
		var name string
		if err := equipRows.Scan(&name); err != nil {
			return nil, err
		}
		row.Equipment = append(row.Equipment, name)
	}
	return row, equipRows.Err()
}

func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

func sqlInt(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}

func sqlText(v *string) any {
	if v == nil {
		return nil
	}
	return *v
}

func sqlFloat(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}

func intFromSQL(v sql.NullInt64) *int {
	if !v.Valid {
		return nil
	}
	value := int(v.Int64)
	return &value
}

func bit(v bool) int {
	if v {
		return 1
	}
	return 0
}
