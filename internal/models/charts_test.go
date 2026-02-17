package models

import (
	"database/sql"
	"testing"
)

func TestComputeChartPoints_Empty(t *testing.T) {
	result := computeChartPoints(nil, nil, "lbs")
	if result.HasData {
		t.Error("expected HasData=false for empty input")
	}
}

func TestComputeChartPoints_SinglePoint(t *testing.T) {
	result := computeChartPoints([]string{"2025-01-01"}, []float64{185.0}, "lbs")
	if !result.HasData {
		t.Fatal("expected HasData=true")
	}
	if len(result.Points) != 1 {
		t.Fatalf("expected 1 point, got %d", len(result.Points))
	}
	if result.Points[0].Value != 185.0 {
		t.Errorf("expected value 185.0, got %f", result.Points[0].Value)
	}
	if result.PolyLine == "" {
		t.Error("expected non-empty polyline")
	}
}

func TestComputeChartPoints_MultiplePoints(t *testing.T) {
	dates := []string{"2025-01-01", "2025-01-08", "2025-01-15", "2025-01-22"}
	values := []float64{180.0, 185.0, 190.0, 195.0}

	result := computeChartPoints(dates, values, "kg")
	if !result.HasData {
		t.Fatal("expected HasData=true")
	}
	if len(result.Points) != 4 {
		t.Fatalf("expected 4 points, got %d", len(result.Points))
	}

	// First point should be leftmost, last should be rightmost.
	if result.Points[0].X >= result.Points[3].X {
		t.Error("points should go left to right")
	}

	// Higher values should have lower Y (SVG y increases downward).
	if result.Points[0].Y <= result.Points[3].Y {
		t.Error("higher values should have lower Y coordinates")
	}

	if result.MinLabel != "2025-01-01" {
		t.Errorf("expected MinLabel=2025-01-01, got %s", result.MinLabel)
	}
	if result.MaxLabel != "2025-01-22" {
		t.Errorf("expected MaxLabel=2025-01-22, got %s", result.MaxLabel)
	}
	if result.ValueUnit != "kg" {
		t.Errorf("expected unit=kg, got %s", result.ValueUnit)
	}
}

func TestComputeChartPoints_ConstantValues(t *testing.T) {
	dates := []string{"2025-01-01", "2025-01-02", "2025-01-03"}
	values := []float64{100.0, 100.0, 100.0}

	result := computeChartPoints(dates, values, "lbs")
	if !result.HasData {
		t.Fatal("expected HasData=true for constant values")
	}

	// All Y values should be the same (middle of chart).
	for i := 1; i < len(result.Points); i++ {
		if result.Points[i].Y != result.Points[0].Y {
			t.Errorf("expected same Y for constant values, got %f vs %f", result.Points[0].Y, result.Points[i].Y)
		}
	}
}

func TestNiceYLabels(t *testing.T) {
	labels := niceYLabels(175.0, 200.0, 4)
	if len(labels) == 0 {
		t.Error("expected at least one Y label")
	}
	for _, l := range labels {
		if l.Label == "" {
			t.Error("expected non-empty label")
		}
		if l.Y < chartPadTop || l.Y > chartHeight-chartPadBot {
			t.Errorf("Y label outside chart bounds: %f", l.Y)
		}
	}
}

func TestFormatChartValue(t *testing.T) {
	tests := []struct {
		input float64
		want  string
	}{
		{100.0, "100"},
		{185.5, "185.5"},
		{200.0, "200"},
		{0.0, "0"},
	}
	for _, tt := range tests {
		got := formatChartValue(tt.input)
		if got != tt.want {
			t.Errorf("formatChartValue(%f) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestVolumeToLevel(t *testing.T) {
	tests := []struct {
		vol, max float64
		want     int
	}{
		{0, 100, 0},
		{10, 100, 1},
		{30, 100, 2},
		{55, 100, 3},
		{80, 100, 4},
		{100, 100, 4},
		{0, 0, 0},
	}
	for _, tt := range tests {
		got := volumeToLevel(tt.vol, tt.max)
		if got != tt.want {
			t.Errorf("volumeToLevel(%f, %f) = %d, want %d", tt.vol, tt.max, got, tt.want)
		}
	}
}

func TestBodyWeightChartData(t *testing.T) {
	db := testDB(t)

	// Create athlete and seed body weight entries.
	athlete, err := CreateAthlete(db, "Chart Athlete", "", "", "", sql.NullInt64{})
	if err != nil {
		t.Fatalf("create athlete: %v", err)
	}

	dates := []string{"2025-01-01", "2025-01-02", "2025-01-03", "2025-01-04", "2025-01-05"}
	weights := []float64{185.0, 184.5, 184.0, 183.5, 183.0}
	for i := range dates {
		_, err := CreateBodyWeight(db, athlete.ID, dates[i], weights[i], "")
		if err != nil {
			t.Fatalf("create body weight: %v", err)
		}
	}

	chart, err := BodyWeightChartData(db, athlete.ID, 30, "lbs")
	if err != nil {
		t.Fatalf("BodyWeightChartData: %v", err)
	}
	if !chart.HasData {
		t.Fatal("expected HasData=true")
	}
	if len(chart.Points) != 5 {
		t.Errorf("expected 5 points, got %d", len(chart.Points))
	}

	// Verify chronological order.
	if chart.Points[0].Label != "2025-01-01" {
		t.Errorf("first point should be oldest date, got %s", chart.Points[0].Label)
	}
}

func TestBodyWeightChartData_Empty(t *testing.T) {
	db := testDB(t)

	chart, err := BodyWeightChartData(db, 999, 30, "lbs")
	if err != nil {
		t.Fatalf("BodyWeightChartData: %v", err)
	}
	if chart.HasData {
		t.Error("expected HasData=false for nonexistent athlete")
	}
}

func TestTrainingMaxChartData(t *testing.T) {
	db := testDB(t)

	athlete, err := CreateAthlete(db, "TM Chart Athlete", "", "", "", sql.NullInt64{})
	if err != nil {
		t.Fatalf("create athlete: %v", err)
	}

	exercise, err := CreateExercise(db, "Squat", "foundational", "", "", 0)
	if err != nil {
		t.Fatalf("create exercise: %v", err)
	}

	tms := []struct {
		date   string
		weight float64
	}{
		{"2025-01-01", 275.0},
		{"2025-02-01", 285.0},
		{"2025-03-01", 295.0},
	}
	for _, tm := range tms {
		_, err := SetTrainingMax(db, athlete.ID, exercise.ID, tm.weight, tm.date, "")
		if err != nil {
			t.Fatalf("set training max: %v", err)
		}
	}

	chart, err := TrainingMaxChartData(db, athlete.ID, exercise.ID, "lbs")
	if err != nil {
		t.Fatalf("TrainingMaxChartData: %v", err)
	}
	if !chart.HasData {
		t.Fatal("expected HasData=true")
	}
	if len(chart.Points) != 3 {
		t.Errorf("expected 3 points, got %d", len(chart.Points))
	}
}

func TestExerciseVolumeChart(t *testing.T) {
	db := testDB(t)

	athlete, err := CreateAthlete(db, "Vol Athlete", "", "", "", sql.NullInt64{})
	if err != nil {
		t.Fatalf("create athlete: %v", err)
	}

	exercise, err := CreateExercise(db, "Bench Press", "", "", "", 0)
	if err != nil {
		t.Fatalf("create exercise: %v", err)
	}

	// Create workouts with sets.
	dates := []string{"2025-01-01", "2025-01-03"}
	for _, date := range dates {
		w, err := CreateWorkout(db, athlete.ID, date, "")
		if err != nil {
			t.Fatalf("create workout: %v", err)
		}
		_, err = AddSet(db, w.ID, exercise.ID, 5, 100.0, 0, "", "")
		if err != nil {
			t.Fatalf("add set: %v", err)
		}
	}

	chart, err := ExerciseVolumeChart(db, athlete.ID, exercise.ID, 20)
	if err != nil {
		t.Fatalf("ExerciseVolumeChart: %v", err)
	}
	if !chart.HasData {
		t.Fatal("expected HasData=true")
	}
	if len(chart.Bars) != 2 {
		t.Errorf("expected 2 bars, got %d", len(chart.Bars))
	}
	// Volume should be 5 * 100 = 500 for each session.
	for _, bar := range chart.Bars {
		if bar.Volume != 500.0 {
			t.Errorf("expected volume=500, got %f", bar.Volume)
		}
	}
}

func TestExerciseVolumeChart_Empty(t *testing.T) {
	db := testDB(t)

	chart, err := ExerciseVolumeChart(db, 999, 999, 20)
	if err != nil {
		t.Fatalf("ExerciseVolumeChart: %v", err)
	}
	if chart.HasData {
		t.Error("expected HasData=false for no data")
	}
}

func TestWorkoutHeatmap(t *testing.T) {
	db := testDB(t)

	athlete, err := CreateAthlete(db, "Heatmap Athlete", "", "", "", sql.NullInt64{})
	if err != nil {
		t.Fatalf("create athlete: %v", err)
	}

	// Create a workout today so heatmap has data.
	exercise, err := CreateExercise(db, "Deadlift", "", "", "", 0)
	if err != nil {
		t.Fatalf("create exercise: %v", err)
	}
	w, err := CreateWorkout(db, athlete.ID, "2025-06-01", "")
	if err != nil {
		t.Fatalf("create workout: %v", err)
	}
	_, err = AddSet(db, w.ID, exercise.ID, 3, 200.0, 0, "", "")
	if err != nil {
		t.Fatalf("add set: %v", err)
	}

	heatmap, err := WorkoutHeatmap(db, athlete.ID)
	if err != nil {
		t.Fatalf("WorkoutHeatmap: %v", err)
	}
	if !heatmap.HasData {
		t.Error("expected HasData=true")
	}
	if len(heatmap.Cells) == 0 {
		t.Error("expected cells")
	}
	if len(heatmap.MonthLabels) == 0 {
		t.Error("expected month labels")
	}
}

func TestWorkoutHeatmap_Empty(t *testing.T) {
	db := testDB(t)

	athlete, err := CreateAthlete(db, "Empty Heatmap", "", "", "", sql.NullInt64{})
	if err != nil {
		t.Fatalf("create athlete: %v", err)
	}

	heatmap, err := WorkoutHeatmap(db, athlete.ID)
	if err != nil {
		t.Fatalf("WorkoutHeatmap: %v", err)
	}
	if heatmap.HasData {
		t.Error("expected HasData=false for no workouts")
	}
	// Should still have cells (365 days worth).
	if len(heatmap.Cells) == 0 {
		t.Error("expected cells even with no data")
	}
}

func TestGetDashboardStats(t *testing.T) {
	db := testDB(t)

	stats, err := GetDashboardStats(db)
	if err != nil {
		t.Fatalf("GetDashboardStats: %v", err)
	}

	if stats.TotalAthletes != 0 {
		t.Errorf("expected 0 athletes, got %d", stats.TotalAthletes)
	}
	if stats.WeekSessions != 0 {
		t.Errorf("expected 0 sessions, got %d", stats.WeekSessions)
	}
	if stats.ConsecutiveWeeks != 0 {
		t.Errorf("expected 0 week streak, got %d", stats.ConsecutiveWeeks)
	}
}
