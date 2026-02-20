// Package importers provides parsers for importing workout data from external
// fitness tracking applications (Strong, Hevy) and RepLog's own JSON format.
package importers

// Format identifies the source format of an import file.
type Format string

const (
	FormatRepLogJSON   Format = "replog_json"
	FormatCatalogJSON  Format = "catalog_json"
	FormatStrongCSV    Format = "strong_csv"
	FormatHevyCSV      Format = "hevy_csv"
)

// ParsedFile is the unified output from any parser. It contains all entities
// extracted from the import file, using string names (not IDs) for references.
type ParsedFile struct {
	Format Format

	// Common fields (all formats).
	Exercises     []ParsedExercise
	Workouts      []ParsedWorkout
	BodyWeights   []ParsedBodyWeight
	TrainingMaxes []ParsedTrainingMax

	// RepLog JSON-only fields.
	Athlete          *ParsedAthlete
	WeightUnit       string // "lbs" or "kg"
	Equipment        []ParsedEquipment
	AthleteEquipment []string // equipment names the athlete has
	Assignments      []ParsedAssignment
	Programs         []ParsedProgram
}

// ParsedAthlete is an athlete profile from a RepLog JSON export.
type ParsedAthlete struct {
	Name            string  `json:"name"`
	Tier            *string `json:"tier"`
	Notes           *string `json:"notes"`
	Goal            *string `json:"goal"`
	TrackBodyWeight *bool   `json:"track_body_weight"`
}

// ParsedEquipment is a piece of equipment from a RepLog JSON export.
type ParsedEquipment struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

// ParsedExercise is an exercise from a RepLog JSON export.
type ParsedExercise struct {
	Name        string                    `json:"name"`
	Tier        *string                   `json:"tier"`
	FormNotes   *string                   `json:"form_notes"`
	DemoURL     *string                   `json:"demo_url"`
	RestSeconds *int                      `json:"rest_seconds"`
	Featured    bool                      `json:"featured"`
	Equipment   []ParsedExerciseEquipment `json:"equipment"`
}

// ParsedExerciseEquipment describes required/optional equipment for an exercise.
type ParsedExerciseEquipment struct {
	Name     string `json:"name"`
	Optional bool   `json:"optional"`
}

// ParsedAssignment is an exercise assignment from a RepLog JSON export.
type ParsedAssignment struct {
	Exercise      string  `json:"exercise"`
	TargetReps    *int    `json:"target_reps"`
	Active        bool    `json:"active"`
	AssignedAt    string  `json:"assigned_at"`
	DeactivatedAt *string `json:"deactivated_at"`
}

// ParsedTrainingMax is a training max entry (RepLog JSON).
type ParsedTrainingMax struct {
	Exercise      string  `json:"exercise"`
	Weight        float64 `json:"weight"`
	EffectiveDate string  `json:"effective_date"`
	Notes         *string `json:"notes"`
}

// ParsedBodyWeight is a body weight entry (RepLog JSON).
type ParsedBodyWeight struct {
	Date   string  `json:"date"`
	Weight float64 `json:"weight"`
	Notes  *string `json:"notes"`
}

// ParsedWorkout is a workout with its sets.
type ParsedWorkout struct {
	Date   string             `json:"date"`
	Notes  *string            `json:"notes"`
	Review *ParsedReview      `json:"review"`
	Sets   []ParsedWorkoutSet `json:"sets"`
}

// ParsedReview is a workout review from a RepLog JSON export.
type ParsedReview struct {
	Status string  `json:"status"`
	Notes  *string `json:"notes"`
}

// ParsedWorkoutSet is a single set within a workout.
type ParsedWorkoutSet struct {
	Exercise  string   `json:"exercise"`
	SetNumber int      `json:"set_number"`
	Reps      int      `json:"reps"`
	RepType   string   `json:"rep_type"`
	Weight    *float64 `json:"weight"`
	RPE       *float64 `json:"rpe"`
	Notes     *string  `json:"notes"`
}

// ParsedProgram is a program assignment with its template from a RepLog JSON export.
type ParsedProgram struct {
	Template  ParsedProgramTemplate `json:"template"`
	StartDate string                `json:"start_date"`
	Active    bool                  `json:"active"`
	Notes     *string               `json:"notes"`
	Goal      *string               `json:"goal"`
}

// ParsedProgramTemplate is a program template definition.
type ParsedProgramTemplate struct {
	Name             string                  `json:"name"`
	Description      *string                 `json:"description"`
	NumWeeks         int                     `json:"num_weeks"`
	NumDays          int                     `json:"num_days"`
	IsLoop           bool                    `json:"is_loop"`
	Audience         *string                 `json:"audience"`
	PrescribedSets   []ParsedPrescribedSet   `json:"prescribed_sets"`
	ProgressionRules []ParsedProgressionRule `json:"progression_rules"`
}

// ParsedPrescribedSet is a prescribed set within a program template.
type ParsedPrescribedSet struct {
	Exercise       string   `json:"exercise"`
	Week           int      `json:"week"`
	Day            int      `json:"day"`
	SetNumber      int      `json:"set_number"`
	Reps           *int     `json:"reps"` // nil = AMRAP
	RepType        string   `json:"rep_type"`
	Percentage     *float64 `json:"percentage"`
	AbsoluteWeight *float64 `json:"absolute_weight"`
	SortOrder      int      `json:"sort_order"`
	Notes          *string  `json:"notes"`
}

// ParsedProgressionRule is a TM increment rule for a program template.
type ParsedProgressionRule struct {
	Exercise  string  `json:"exercise"`
	Increment float64 `json:"increment"`
}

// DetectFormat guesses the import format from file content.
// It returns FormatRepLogJSON for JSON, and attempts to identify
// Strong vs Hevy CSV from headers. Returns empty string if unknown.
func DetectFormat(data []byte) Format {
	trimmed := data

	// Trim BOM and whitespace.
	if len(trimmed) >= 3 && trimmed[0] == 0xEF && trimmed[1] == 0xBB && trimmed[2] == 0xBF {
		trimmed = trimmed[3:]
	}
	for len(trimmed) > 0 && (trimmed[0] == ' ' || trimmed[0] == '\t' || trimmed[0] == '\n' || trimmed[0] == '\r') {
		trimmed = trimmed[1:]
	}

	if len(trimmed) > 0 && trimmed[0] == '{' {
		// Peek at the JSON to distinguish catalog from athlete export.
		if containsAll(string(trimmed[:min(len(trimmed), 200)]), `"type"`, `"catalog"`) {
			return FormatCatalogJSON
		}
		return FormatRepLogJSON
	}

	// Read the first line to determine CSV type.
	firstLine := firstLineOf(trimmed)
	if containsAll(firstLine, "Exercise Name", "Set Order", "Weight", "Reps") {
		return FormatStrongCSV
	}
	if containsAll(firstLine, "exercise_title", "set_index", "weight_lbs", "reps") {
		return FormatHevyCSV
	}

	return ""
}

func firstLineOf(data []byte) string {
	for i, b := range data {
		if b == '\n' || b == '\r' {
			return string(data[:i])
		}
	}
	return string(data)
}

func containsAll(s string, substrings ...string) bool {
	for _, sub := range substrings {
		found := false
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
