package importers

import (
	"encoding/json"
	"fmt"
	"io"
)

// replogJSON mirrors the RepLog Native JSON schema for deserialization.
type replogJSON struct {
	Version    string `json:"version"`
	WeightUnit string `json:"weight_unit"`

	Athlete          *ParsedAthlete     `json:"athlete"`
	Equipment        []ParsedEquipment  `json:"equipment"`
	AthleteEquipment []string           `json:"athlete_equipment"`
	Exercises        []ParsedExercise   `json:"exercises"`
	Assignments      []ParsedAssignment `json:"assignments"`
	TrainingMaxes    []ParsedTrainingMax `json:"training_maxes"`
	BodyWeights      []ParsedBodyWeight `json:"body_weights"`
	Workouts         []replogWorkoutJSON `json:"workouts"`
	Programs         []ParsedProgram    `json:"programs"`
}

// replogWorkoutJSON matches the JSON workout structure (review is inline).
type replogWorkoutJSON struct {
	Date   string             `json:"date"`
	Notes  *string            `json:"notes"`
	Review *ParsedReview      `json:"review"`
	Sets   []ParsedWorkoutSet `json:"sets"`
}

// ParseRepLogJSON parses a RepLog Native JSON export.
func ParseRepLogJSON(r io.Reader) (*ParsedFile, error) {
	var rj replogJSON
	if err := json.NewDecoder(r).Decode(&rj); err != nil {
		return nil, fmt.Errorf("importers: decode replog json: %w", err)
	}

	if rj.Version == "" {
		return nil, fmt.Errorf("importers: replog json missing version field")
	}

	pf := &ParsedFile{
		Format:           FormatRepLogJSON,
		WeightUnit:       rj.WeightUnit,
		Athlete:          rj.Athlete,
		Equipment:        rj.Equipment,
		AthleteEquipment: rj.AthleteEquipment,
		Exercises:        rj.Exercises,
		Assignments:      rj.Assignments,
		TrainingMaxes:    rj.TrainingMaxes,
		BodyWeights:      rj.BodyWeights,
		Programs:         rj.Programs,
	}

	// Convert workouts.
	for _, wj := range rj.Workouts {
		pw := ParsedWorkout{
			Date:   wj.Date,
			Notes:  wj.Notes,
			Review: wj.Review,
			Sets:   wj.Sets,
		}
		// Default rep_type if not specified.
		for i := range pw.Sets {
			if pw.Sets[i].RepType == "" {
				pw.Sets[i].RepType = "reps"
			}
		}
		pf.Workouts = append(pf.Workouts, pw)
	}

	if pf.WeightUnit == "" {
		pf.WeightUnit = "lbs"
	}

	return pf, nil
}
