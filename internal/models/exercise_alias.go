package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// Exercise matching (ADR 026). A coach writes "Pull Up", "Pull-up", or
// "pullup" for the same movement, so names and aliases are compared by a
// key: lower case, letters and digits only. An alias key is unique across
// the whole catalog and never equals another exercise's name key.
const (
	matchOnNormalizedName  = "normalized_name"
	matchOnNormalizedAlias = "normalized_alias"
	exerciseNameMax        = 100
	maxExerciseSuggestions = 3
)

var (
	// ErrExerciseNameConflict is returned when a name or alias would match
	// another exercise.
	ErrExerciseNameConflict = errors.New("name matches another exercise")
	// ErrExerciseChanged is returned when a confirm finds the exercise changed
	// after its preview.
	ErrExerciseChanged = errors.New("exercise changed since the preview")
)

// ExerciseKey is the matching key for a name or alias.
func ExerciseKey(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ExerciseRef names one catalog exercise.
type ExerciseRef struct {
	ID   int64
	Name string
}

// AmbiguousExerciseError is returned when a name matches more than one
// exercise by key, so the import cannot pick one.
type AmbiguousExerciseError struct {
	Name       string
	Candidates []ExerciseRef
}

func (e *AmbiguousExerciseError) Error() string {
	names := make([]string, 0, len(e.Candidates))
	for _, c := range e.Candidates {
		names = append(names, fmt.Sprintf("%s (id %d)", c.Name, c.ID))
	}
	return fmt.Sprintf("%q matches more than one exercise: %s. Add an alias or rename one first", e.Name, strings.Join(names, ", "))
}

func (e *AmbiguousExerciseError) Unwrap() error { return ErrExerciseAmbiguous }

// ExerciseConflictError names the exercise a name or alias would collide with.
type ExerciseConflictError struct {
	Value    string
	Exercise ExerciseRef
	On       string // "name" or "alias"
}

func (e *ExerciseConflictError) Error() string {
	return fmt.Sprintf("%q matches the %s of %s (id %d)", e.Value, e.On, e.Exercise.Name, e.Exercise.ID)
}

func (e *ExerciseConflictError) Unwrap() error { return ErrExerciseNameConflict }

// ExerciseSuggestion is a likely existing match for a name that would be created.
type ExerciseSuggestion struct {
	ExerciseID int64
	Name       string
	Reason     string
}

type indexedLabel struct {
	exercise ExerciseRef
	label    string
	alias    bool
}

// exerciseIndex is a snapshot of every name and alias in the catalog.
type exerciseIndex struct {
	names   map[int64]string
	labels  []indexedLabel
	byLower map[string][]indexedLabel
	byKey   map[string][]indexedLabel
}

func loadExerciseIndex(ctx context.Context, q queryer) (*exerciseIndex, error) {
	idx := &exerciseIndex{names: map[int64]string{}, byLower: map[string][]indexedLabel{}, byKey: map[string][]indexedLabel{}}
	rows, err := q.QueryContext(ctx, `SELECT id, name FROM exercises`)
	if err != nil {
		return nil, fmt.Errorf("models: load exercise names: %w", err)
	}
	for rows.Next() {
		var ref ExerciseRef
		if err := rows.Scan(&ref.ID, &ref.Name); err != nil {
			rows.Close()
			return nil, err
		}
		idx.names[ref.ID] = ref.Name
		idx.add(indexedLabel{exercise: ref, label: ref.Name})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	rows, err = q.QueryContext(ctx, `SELECT exercise_id, alias FROM exercise_aliases`)
	if err != nil {
		return nil, fmt.Errorf("models: load exercise aliases: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var alias string
		if err := rows.Scan(&id, &alias); err != nil {
			return nil, err
		}
		idx.add(indexedLabel{exercise: ExerciseRef{ID: id, Name: idx.names[id]}, label: alias, alias: true})
	}
	return idx, rows.Err()
}

func (idx *exerciseIndex) add(l indexedLabel) {
	idx.labels = append(idx.labels, l)
	lower := strings.ToLower(l.label)
	idx.byLower[lower] = append(idx.byLower[lower], l)
	if key := ExerciseKey(l.label); key != "" {
		idx.byKey[key] = append(idx.byKey[key], l)
	}
}

// resolve finds the exercise a program name means. An exact name beats an
// exact alias, which beats a key match. A key that reaches two exercises is
// an AmbiguousExerciseError. ok is false when nothing matches.
func (idx *exerciseIndex) resolve(name string) (ExerciseRef, string, bool, error) {
	lower := strings.ToLower(strings.TrimSpace(name))
	for _, alias := range []bool{false, true} {
		for _, l := range idx.byLower[lower] {
			if l.alias == alias {
				how := matchOnName
				if alias {
					how = matchOnAlias
				}
				return l.exercise, how, true, nil
			}
		}
	}
	key := ExerciseKey(name)
	if key == "" {
		return ExerciseRef{}, "", false, nil
	}
	hits := idx.byKey[key]
	if len(hits) == 0 {
		return ExerciseRef{}, "", false, nil
	}
	distinct := map[int64]ExerciseRef{}
	how := matchOnNormalizedAlias
	for _, l := range hits {
		distinct[l.exercise.ID] = l.exercise
		if !l.alias {
			how = matchOnNormalizedName
		}
	}
	if len(distinct) > 1 {
		return ExerciseRef{}, "", false, &AmbiguousExerciseError{Name: name, Candidates: sortedRefs(distinct)}
	}
	return hits[0].exercise, how, true, nil
}

// conflict reports the first label of another exercise whose key equals value's key.
func (idx *exerciseIndex) conflict(value string, self int64) *ExerciseConflictError {
	for _, l := range idx.byKey[ExerciseKey(value)] {
		if l.exercise.ID == self {
			continue
		}
		on := "name"
		if l.alias {
			on = "alias"
		}
		return &ExerciseConflictError{Value: value, Exercise: l.exercise, On: on}
	}
	return nil
}

// suggest lists likely existing exercises for a name that matched nothing.
func (idx *exerciseIndex) suggest(name string) []ExerciseSuggestion {
	key := ExerciseKey(name)
	words := exerciseWords(name)
	type scored struct {
		s     ExerciseSuggestion
		score float64
	}
	best := map[int64]scored{}
	for _, l := range idx.labels {
		lkey := ExerciseKey(l.label)
		if lkey == "" || key == "" {
			continue
		}
		var score float64
		var reason string
		lwords := exerciseWords(l.label)
		switch {
		case strings.Join(words, "") == strings.Join(lwords, ""):
			score, reason = 1.0, "same words"
		case len(key) >= 3 && len(lkey) >= 3 && (strings.Contains(key, lkey) || strings.Contains(lkey, key)):
			score, reason = 0.8, "contains"
		default:
			if j := jaccard(words, lwords); j >= 0.5 || (j > 0 && sharesLongWord(words, lwords)) {
				score, reason = 0.4+j/2, "shared words"
			} else if r := similarity(key, lkey); r >= 0.8 {
				score, reason = r*0.9, "similar spelling"
			}
		}
		if score == 0 {
			continue
		}
		if l.alias {
			reason += " (alias " + l.label + ")"
		}
		if cur, ok := best[l.exercise.ID]; !ok || score > cur.score {
			best[l.exercise.ID] = scored{s: ExerciseSuggestion{ExerciseID: l.exercise.ID, Name: l.exercise.Name, Reason: reason}, score: score}
		}
	}
	list := make([]scored, 0, len(best))
	for _, v := range best {
		list = append(list, v)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].score != list[j].score {
			return list[i].score > list[j].score
		}
		return strings.ToLower(list[i].s.Name) < strings.ToLower(list[j].s.Name)
	})
	out := []ExerciseSuggestion{}
	for i := 0; i < len(list) && i < maxExerciseSuggestions; i++ {
		out = append(out, list[i].s)
	}
	return out
}

// exerciseWords splits a name into lower-case words with a plural s removed,
// so "Pull-Ups" and "pull up" have the same words.
func exerciseWords(s string) []string {
	fields := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if len(f) >= 3 && strings.HasSuffix(f, "s") && !strings.HasSuffix(f, "ss") {
			f = strings.TrimSuffix(f, "s")
		}
		out = append(out, f)
	}
	return out
}

// sharesLongWord reports a common word of four or more letters, so
// "Goblet Squat" suggests "Front Squat" but "DB Row" does not suggest "DB Curl".
func sharesLongWord(a, b []string) bool {
	set := map[string]struct{}{}
	for _, w := range a {
		if len(w) >= 4 {
			set[w] = struct{}{}
		}
	}
	for _, w := range b {
		if _, ok := set[w]; ok {
			return true
		}
	}
	return false
}

func jaccard(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	set := map[string]int{}
	for _, w := range a {
		set[w] |= 1
	}
	for _, w := range b {
		set[w] |= 2
	}
	both := 0
	for _, v := range set {
		if v == 3 {
			both++
		}
	}
	return float64(both) / float64(len(set))
}

// similarity is 1 minus the edit distance over the longer length.
func similarity(a, b string) float64 {
	ra, rb := []rune(a), []rune(b)
	if len(ra) == 0 || len(rb) == 0 {
		return 0
	}
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	longer := max(len(ra), len(rb))
	return 1 - float64(prev[len(rb)])/float64(longer)
}

func sortedRefs(m map[int64]ExerciseRef) []ExerciseRef {
	out := make([]ExerciseRef, 0, len(m))
	for _, r := range m {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func cleanExerciseLabel(field, value string) (string, error) {
	value = strings.Join(strings.Fields(value), " ")
	if value == "" {
		return "", fmt.Errorf("%w: %s is required", ErrInvalidInput, field)
	}
	if len([]rune(value)) > exerciseNameMax {
		return "", fmt.Errorf("%w: %s must be at most %d characters", ErrInvalidInput, field, exerciseNameMax)
	}
	if ExerciseKey(value) == "" {
		return "", fmt.Errorf("%w: %s needs a letter or digit", ErrInvalidInput, field)
	}
	return value, nil
}

// AliasPlan is a checked alias addition. Planning writes nothing.
type AliasPlan struct {
	ExerciseID   int64
	ExerciseName string
	Alias        string
	Aliases      []string // after the change
}

// PlanExerciseAlias checks that alias can be added to exerciseID.
func PlanExerciseAlias(ctx context.Context, q queryer, exerciseID int64, alias string) (*AliasPlan, error) {
	alias, err := cleanExerciseLabel("alias", alias)
	if err != nil {
		return nil, err
	}
	idx, err := loadExerciseIndex(ctx, q)
	if err != nil {
		return nil, err
	}
	name, ok := idx.names[exerciseID]
	if !ok {
		return nil, ErrNotFound
	}
	key := ExerciseKey(alias)
	if key == ExerciseKey(name) {
		return nil, fmt.Errorf("%w: alias %q is the exercise name", ErrInvalidInput, alias)
	}
	if c := idx.conflict(alias, exerciseID); c != nil {
		return nil, c
	}
	aliases := idx.aliasesOf(exerciseID)
	for _, a := range aliases {
		if ExerciseKey(a) == key {
			return nil, fmt.Errorf("%w: %q is already an alias (%s)", ErrInvalidInput, alias, a)
		}
	}
	aliases = append(aliases, alias)
	sortFold(aliases)
	return &AliasPlan{ExerciseID: exerciseID, ExerciseName: name, Alias: alias, Aliases: aliases}, nil
}

// ApplyExerciseAlias re-checks plan in one transaction and adds the alias.
// It touches only exercise_aliases.
func ApplyExerciseAlias(ctx context.Context, db *sql.DB, plan AliasPlan) (*CoachCatalogExercise, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	fresh, err := PlanExerciseAlias(ctx, tx, plan.ExerciseID, plan.Alias)
	if err != nil {
		return nil, err
	}
	if fresh.ExerciseName != plan.ExerciseName {
		return nil, ErrExerciseChanged
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO exercise_aliases (exercise_id, alias) VALUES (?, ?)`, plan.ExerciseID, fresh.Alias); err != nil {
		return nil, fmt.Errorf("models: add exercise alias: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return loadCoachExercise(ctx, db, plan.ExerciseID)
}

// RenamePlan is a checked rename. Planning writes nothing.
type RenamePlan struct {
	ExerciseID  int64
	OldName     string
	NewName     string
	AddAlias    *string // the old name, kept as an alias
	RemoveAlias *string // an alias that becomes the name
	Aliases     []string
}

// PlanExerciseRename checks renaming exerciseID to newName. The old name is
// kept as an alias unless it differs from the new name only in case or
// punctuation. Logged sets, PRs, and programs refer to the id, so they
// follow the new name and are not rewritten.
func PlanExerciseRename(ctx context.Context, q queryer, exerciseID int64, newName string) (*RenamePlan, error) {
	newName, err := cleanExerciseLabel("name", newName)
	if err != nil {
		return nil, err
	}
	idx, err := loadExerciseIndex(ctx, q)
	if err != nil {
		return nil, err
	}
	oldName, ok := idx.names[exerciseID]
	if !ok {
		return nil, ErrNotFound
	}
	if newName == oldName {
		return nil, fmt.Errorf("%w: name is unchanged", ErrInvalidInput)
	}
	if c := idx.conflict(newName, exerciseID); c != nil {
		return nil, c
	}
	plan := &RenamePlan{ExerciseID: exerciseID, OldName: oldName, NewName: newName}
	newKey := ExerciseKey(newName)
	aliases := []string{}
	for _, a := range idx.aliasesOf(exerciseID) {
		if ExerciseKey(a) == newKey {
			removed := a
			plan.RemoveAlias = &removed
			continue
		}
		aliases = append(aliases, a)
	}
	if ExerciseKey(oldName) != newKey {
		if c := idx.conflict(oldName, exerciseID); c != nil {
			return nil, fmt.Errorf("old name cannot be kept as an alias: %w", c)
		}
		keep := oldName
		plan.AddAlias = &keep
		dup := false
		for _, a := range aliases {
			if ExerciseKey(a) == ExerciseKey(oldName) {
				dup = true
			}
		}
		if dup {
			plan.AddAlias = nil
		} else {
			aliases = append(aliases, oldName)
		}
	}
	sortFold(aliases)
	plan.Aliases = aliases
	return plan, nil
}

// ApplyExerciseRename re-checks plan in one transaction and renames the
// exercise by id. Only exercises.name and exercise_aliases change.
func ApplyExerciseRename(ctx context.Context, db *sql.DB, plan RenamePlan) (*CoachCatalogExercise, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	fresh, err := PlanExerciseRename(ctx, tx, plan.ExerciseID, plan.NewName)
	if err != nil {
		return nil, err
	}
	if fresh.OldName != plan.OldName {
		return nil, ErrExerciseChanged
	}
	if fresh.RemoveAlias != nil {
		if _, err := tx.ExecContext(ctx, `DELETE FROM exercise_aliases WHERE exercise_id = ? AND alias = ?`, plan.ExerciseID, *fresh.RemoveAlias); err != nil {
			return nil, fmt.Errorf("models: rename exercise drop alias: %w", err)
		}
	}
	res, err := tx.ExecContext(ctx, `UPDATE exercises SET name = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND name = ?`, fresh.NewName, plan.ExerciseID, plan.OldName)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrDuplicateExerciseName
		}
		return nil, fmt.Errorf("models: rename exercise: %w", err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return nil, ErrExerciseChanged
	}
	if fresh.AddAlias != nil {
		if _, err := tx.ExecContext(ctx, `INSERT INTO exercise_aliases (exercise_id, alias) VALUES (?, ?)`, plan.ExerciseID, *fresh.AddAlias); err != nil {
			return nil, fmt.Errorf("models: rename exercise keep alias: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return loadCoachExercise(ctx, db, plan.ExerciseID)
}

func (idx *exerciseIndex) aliasesOf(id int64) []string {
	out := []string{}
	for _, l := range idx.labels {
		if l.alias && l.exercise.ID == id {
			out = append(out, l.label)
		}
	}
	sortFold(out)
	return out
}

func sortFold(s []string) {
	sort.Slice(s, func(i, j int) bool { return strings.ToLower(s[i]) < strings.ToLower(s[j]) })
}

// checkNameFreeOfAliases refuses an exercise name whose key equals another
// exercise's alias, so a catalog edit cannot shadow an alias.
func checkNameFreeOfAliases(ctx context.Context, q queryer, name string, self int64) error {
	idx, err := loadExerciseIndex(ctx, q)
	if err != nil {
		return err
	}
	for _, l := range idx.byKey[ExerciseKey(name)] {
		if l.alias && l.exercise.ID != self {
			return fmt.Errorf("%w: %w", ErrDuplicateExerciseName, &ExerciseConflictError{Value: name, Exercise: l.exercise, On: "alias"})
		}
	}
	return nil
}
