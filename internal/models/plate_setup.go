package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"time"
)

// Plate setup units. Each part of the setup carries its own unit, separate
// from the logging unit in user_preferences, so an athlete can log in lbs
// while loading kg competition plates.
const (
	PlateUnitKg  = "kg"
	PlateUnitLbs = "lbs"
)

// MaxPlatePairs caps one plate size in an inventory.
const MaxPlatePairs = 50

// MaxPlateKinds caps how many plate sizes one inventory lists.
const MaxPlateKinds = 30

// PlatePair is one plate size in an inventory. Pairs is how many matched
// pairs the gym has (one plate per sleeve per pair).
type PlatePair struct {
	Weight float64
	Unit   string
	Pairs  int
}

// PlateSetup is the barbell loading setup for one athlete's gym: the bar,
// optional collars (Collar is the weight of one collar), and the plate
// inventory. IsDefault is true when the athlete has not saved a setup and
// the default kg competition setup is returned instead.
type PlateSetup struct {
	AthleteID    int64
	BarWeight    float64
	BarUnit      string
	CollarWeight float64
	CollarUnit   string
	UseCollars   bool
	Plates       []PlatePair
	IsDefault    bool
	UpdatedAt    *time.Time
}

// PlateSetupPreset is a named starting setup a coach can pick.
type PlateSetupPreset struct {
	Key   string
	Name  string
	Setup PlateSetup
}

// DefaultPlateSetup is the home setup offered when none is saved: a 20 kg
// bar, optional 2.5 kg competition collars (off), and competition kg plates.
func DefaultPlateSetup(athleteID int64) *PlateSetup {
	return &PlateSetup{
		AthleteID:    athleteID,
		BarWeight:    20,
		BarUnit:      PlateUnitKg,
		CollarWeight: 2.5,
		CollarUnit:   PlateUnitKg,
		UseCollars:   false,
		Plates: []PlatePair{
			{Weight: 25, Unit: PlateUnitKg, Pairs: 1},
			{Weight: 20, Unit: PlateUnitKg, Pairs: 2},
			{Weight: 15, Unit: PlateUnitKg, Pairs: 1},
			{Weight: 10, Unit: PlateUnitKg, Pairs: 1},
			{Weight: 5, Unit: PlateUnitKg, Pairs: 1},
			{Weight: 2.5, Unit: PlateUnitKg, Pairs: 1},
			{Weight: 2, Unit: PlateUnitKg, Pairs: 1},
			{Weight: 1.5, Unit: PlateUnitKg, Pairs: 1},
			{Weight: 1, Unit: PlateUnitKg, Pairs: 1},
			{Weight: 0.5, Unit: PlateUnitKg, Pairs: 1},
			{Weight: 0.25, Unit: PlateUnitKg, Pairs: 1},
			{Weight: 0.125, Unit: PlateUnitKg, Pairs: 1},
		},
		IsDefault: true,
	}
}

// StandardLbPlateSetup is a standard lb gym: a 45 lb bar and 45/35/25/10/5/2.5 lb plates.
func StandardLbPlateSetup(athleteID int64) *PlateSetup {
	return &PlateSetup{
		AthleteID:    athleteID,
		BarWeight:    45,
		BarUnit:      PlateUnitLbs,
		CollarWeight: 0,
		CollarUnit:   PlateUnitLbs,
		UseCollars:   false,
		Plates: []PlatePair{
			{Weight: 45, Unit: PlateUnitLbs, Pairs: 4},
			{Weight: 35, Unit: PlateUnitLbs, Pairs: 1},
			{Weight: 25, Unit: PlateUnitLbs, Pairs: 1},
			{Weight: 10, Unit: PlateUnitLbs, Pairs: 2},
			{Weight: 5, Unit: PlateUnitLbs, Pairs: 1},
			{Weight: 2.5, Unit: PlateUnitLbs, Pairs: 1},
		},
	}
}

// PlateSetupPresets lists the starting setups: kg competition (the default)
// and a standard lb gym.
func PlateSetupPresets() []PlateSetupPreset {
	kg := DefaultPlateSetup(0)
	kg.IsDefault = false
	return []PlateSetupPreset{
		{Key: "kg_competition", Name: "Competition kg plates (20 kg bar)", Setup: *kg},
		{Key: "lb_standard", Name: "Standard lb plates (45 lb bar)", Setup: *StandardLbPlateSetup(0)},
	}
}

// ValidPlateUnit reports whether u is kg or lbs.
func ValidPlateUnit(u string) bool {
	return u == PlateUnitKg || u == PlateUnitLbs
}

// ValidatePlateSetup checks a setup before it is saved. It returns the
// offending field and a message, or "" when the setup is valid.
func ValidatePlateSetup(s *PlateSetup) (string, string) {
	if !validWeight(s.BarWeight) || s.BarWeight > 100 {
		return "bar_weight", "must be between 0 and 100"
	}
	if !ValidPlateUnit(s.BarUnit) {
		return "bar_unit", "must be kg or lbs"
	}
	if !validWeight(s.CollarWeight) || s.CollarWeight > 25 {
		return "collar_weight", "must be between 0 and 25"
	}
	if !ValidPlateUnit(s.CollarUnit) {
		return "collar_unit", "must be kg or lbs"
	}
	if len(s.Plates) > MaxPlateKinds {
		return "plates", fmt.Sprintf("must list at most %d plate sizes", MaxPlateKinds)
	}
	seen := map[PlatePair]bool{}
	for _, p := range s.Plates {
		if !validWeight(p.Weight) || p.Weight == 0 || p.Weight > 100 {
			return "plates.weight", "must be greater than 0 and at most 100"
		}
		if !ValidPlateUnit(p.Unit) {
			return "plates.unit", "must be kg or lbs"
		}
		if p.Pairs < 0 || p.Pairs > MaxPlatePairs {
			return "plates.pairs", fmt.Sprintf("must be between 0 and %d", MaxPlatePairs)
		}
		key := PlatePair{Weight: p.Weight, Unit: p.Unit}
		if seen[key] {
			return "plates", fmt.Sprintf("%g %s is listed twice", p.Weight, p.Unit)
		}
		seen[key] = true
	}
	return "", ""
}

func validWeight(v float64) bool {
	return v >= 0 && !math.IsNaN(v) && !math.IsInf(v, 0)
}

// GetPlateSetup returns the athlete's saved setup, or DefaultPlateSetup
// (IsDefault true) when none is saved. Plates are heaviest first.
func GetPlateSetup(ctx context.Context, db *sql.DB, athleteID int64) (*PlateSetup, error) {
	s := &PlateSetup{AthleteID: athleteID}
	var updated time.Time
	err := db.QueryRowContext(ctx,
		`SELECT bar_weight, bar_unit, collar_weight, collar_unit, use_collars, updated_at
		   FROM plate_setups WHERE athlete_id = ?`, athleteID,
	).Scan(&s.BarWeight, &s.BarUnit, &s.CollarWeight, &s.CollarUnit, &s.UseCollars, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return DefaultPlateSetup(athleteID), nil
	}
	if err != nil {
		return nil, fmt.Errorf("models: get plate setup for athlete %d: %w", athleteID, err)
	}
	s.UpdatedAt = &updated

	rows, err := db.QueryContext(ctx,
		`SELECT weight, unit, pairs FROM plate_setup_plates
		  WHERE athlete_id = ?
		  ORDER BY CASE unit WHEN 'kg' THEN weight * 2.20462 ELSE weight END DESC, unit`, athleteID)
	if err != nil {
		return nil, fmt.Errorf("models: list plates for athlete %d: %w", athleteID, err)
	}
	defer rows.Close()
	s.Plates = []PlatePair{}
	for rows.Next() {
		var p PlatePair
		if err := rows.Scan(&p.Weight, &p.Unit, &p.Pairs); err != nil {
			return nil, fmt.Errorf("models: scan plate: %w", err)
		}
		s.Plates = append(s.Plates, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("models: iterate plates: %w", err)
	}
	return s, nil
}

// SavePlateSetup stores the athlete's setup, replacing the whole plate
// inventory in one transaction. The setup is validated first; an invalid
// one returns an ErrInvalidInput error.
func SavePlateSetup(ctx context.Context, db *sql.DB, s *PlateSetup) (*PlateSetup, error) {
	if field, msg := ValidatePlateSetup(s); field != "" {
		return nil, fmt.Errorf("%w: %s %s", ErrInvalidInput, field, msg)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("models: save plate setup begin: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO plate_setups (athlete_id, bar_weight, bar_unit, collar_weight, collar_unit, use_collars)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT (athlete_id) DO UPDATE SET
		     bar_weight = excluded.bar_weight,
		     bar_unit = excluded.bar_unit,
		     collar_weight = excluded.collar_weight,
		     collar_unit = excluded.collar_unit,
		     use_collars = excluded.use_collars,
		     updated_at = CURRENT_TIMESTAMP`,
		s.AthleteID, s.BarWeight, s.BarUnit, s.CollarWeight, s.CollarUnit, boolToInt(s.UseCollars),
	); err != nil {
		return nil, fmt.Errorf("models: save plate setup for athlete %d: %w", s.AthleteID, err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM plate_setup_plates WHERE athlete_id = ?`, s.AthleteID); err != nil {
		return nil, fmt.Errorf("models: clear plates for athlete %d: %w", s.AthleteID, err)
	}
	for _, p := range s.Plates {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO plate_setup_plates (athlete_id, weight, unit, pairs) VALUES (?, ?, ?, ?)`,
			s.AthleteID, p.Weight, p.Unit, p.Pairs,
		); err != nil {
			return nil, fmt.Errorf("models: save plate %g %s for athlete %d: %w", p.Weight, p.Unit, s.AthleteID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("models: save plate setup commit: %w", err)
	}
	return GetPlateSetup(ctx, db, s.AthleteID)
}

// DeletePlateSetup removes the athlete's saved setup so reads return the
// default again. Deleting a setup that does not exist is not an error.
func DeletePlateSetup(ctx context.Context, db *sql.DB, athleteID int64) error {
	if _, err := db.ExecContext(ctx, `DELETE FROM plate_setups WHERE athlete_id = ?`, athleteID); err != nil {
		return fmt.Errorf("models: delete plate setup for athlete %d: %w", athleteID, err)
	}
	return nil
}
