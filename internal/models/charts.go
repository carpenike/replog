package models

import (
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"
)

// ChartPoint represents a single data point for SVG line/area charts.
type ChartPoint struct {
	X     float64 // SVG x coordinate
	Y     float64 // SVG y coordinate
	Value float64 // Original data value
	Label string  // Date or label for tooltip
}

// ChartData holds pre-computed SVG chart data for rendering in templates.
type ChartData struct {
	Points    []ChartPoint
	PolyLine  string // Pre-computed SVG polyline points string
	AreaPath  string // Pre-computed SVG area path (for filled charts)
	MinValue  float64
	MaxValue  float64
	MinLabel  string
	MaxLabel  string
	YLabels   []ChartYLabel // Y-axis grid labels
	HasData   bool
	ValueUnit string // e.g. "kg", "lbs"
}

// ChartYLabel is a horizontal grid line label.
type ChartYLabel struct {
	Y     float64
	Label string
}

// chartDimensions defines the SVG viewBox dimensions and padding.
const (
	chartWidth    = 600.0
	chartHeight   = 200.0
	chartPadLeft  = 50.0
	chartPadRight = 10.0
	chartPadTop   = 15.0
	chartPadBot   = 25.0
)

// computeChartPoints normalizes a series of (date, value) pairs into SVG
// coordinates within the chart dimensions. Points are ordered chronologically.
func computeChartPoints(dates []string, values []float64, unit string) *ChartData {
	if len(dates) == 0 || len(dates) != len(values) {
		return &ChartData{HasData: false}
	}

	// Find min/max values with 5% padding.
	minVal, maxVal := values[0], values[0]
	for _, v := range values {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}

	// Add padding so points don't sit on the edge.
	valRange := maxVal - minVal
	if valRange == 0 {
		valRange = maxVal * 0.1
		if valRange == 0 {
			valRange = 10
		}
		minVal -= valRange / 2
		maxVal += valRange / 2
	} else {
		minVal -= valRange * 0.05
		maxVal += valRange * 0.05
	}

	plotW := chartWidth - chartPadLeft - chartPadRight
	plotH := chartHeight - chartPadTop - chartPadBot

	points := make([]ChartPoint, len(dates))
	for i := range dates {
		var xFrac float64
		if len(dates) == 1 {
			xFrac = 0.5
		} else {
			xFrac = float64(i) / float64(len(dates)-1)
		}
		yFrac := 1.0 - (values[i]-minVal)/(maxVal-minVal)

		points[i] = ChartPoint{
			X:     chartPadLeft + xFrac*plotW,
			Y:     chartPadTop + yFrac*plotH,
			Value: values[i],
			Label: dates[i],
		}
	}

	// Build polyline string.
	var polyParts []string
	for _, p := range points {
		polyParts = append(polyParts, fmt.Sprintf("%.1f,%.1f", p.X, p.Y))
	}
	polyLine := strings.Join(polyParts, " ")

	// Build area path (same line, with bottom closed).
	bottomY := chartPadTop + plotH
	areaPath := fmt.Sprintf("M%.1f,%.1f ", points[0].X, bottomY)
	for _, p := range points {
		areaPath += fmt.Sprintf("L%.1f,%.1f ", p.X, p.Y)
	}
	areaPath += fmt.Sprintf("L%.1f,%.1f Z", points[len(points)-1].X, bottomY)

	// Generate Y-axis labels (4-5 nice round numbers).
	yLabels := niceYLabels(minVal, maxVal, 4)

	return &ChartData{
		Points:    points,
		PolyLine:  polyLine,
		AreaPath:  areaPath,
		MinValue:  minVal,
		MaxValue:  maxVal,
		MinLabel:  dates[0],
		MaxLabel:  dates[len(dates)-1],
		YLabels:   yLabels,
		HasData:   true,
		ValueUnit: unit,
	}
}

// niceYLabels generates evenly-spaced y-axis labels with nice round numbers.
func niceYLabels(minVal, maxVal float64, count int) []ChartYLabel {
	if count <= 0 {
		count = 4
	}
	valRange := maxVal - minVal
	rawStep := valRange / float64(count)

	// Round step to a nice number.
	magnitude := math.Pow(10, math.Floor(math.Log10(rawStep)))
	normalized := rawStep / magnitude
	var niceStep float64
	switch {
	case normalized <= 1.5:
		niceStep = magnitude
	case normalized <= 3.5:
		niceStep = 2.5 * magnitude
	case normalized <= 7.5:
		niceStep = 5 * magnitude
	default:
		niceStep = 10 * magnitude
	}

	plotH := chartHeight - chartPadTop - chartPadBot
	var labels []ChartYLabel

	// Start from the first nice number above minVal.
	start := math.Ceil(minVal/niceStep) * niceStep
	for v := start; v <= maxVal; v += niceStep {
		yFrac := 1.0 - (v-minVal)/(maxVal-minVal)
		y := chartPadTop + yFrac*plotH
		labels = append(labels, ChartYLabel{
			Y:     y,
			Label: formatChartValue(v),
		})
	}

	return labels
}

// formatChartValue formats a number for y-axis display.
func formatChartValue(v float64) string {
	if v == math.Trunc(v) {
		return fmt.Sprintf("%.0f", v)
	}
	return fmt.Sprintf("%.1f", v)
}

// BodyWeightChartData returns chart data for an athlete's body weight history.
// Returns the last `limit` entries in chronological order.
func BodyWeightChartData(db *sql.DB, athleteID int64, limit int, unit string) (*ChartData, error) {
	if limit <= 0 {
		limit = 30
	}

	rows, err := db.Query(`
		SELECT date, weight FROM body_weights
		WHERE athlete_id = ?
		ORDER BY date DESC
		LIMIT ?`, athleteID, limit)
	if err != nil {
		return nil, fmt.Errorf("models: body weight chart data for athlete %d: %w", athleteID, err)
	}
	defer rows.Close()

	var dates []string
	var values []float64
	for rows.Next() {
		var d string
		var w float64
		if err := rows.Scan(&d, &w); err != nil {
			return nil, fmt.Errorf("models: scan body weight chart: %w", err)
		}
		dates = append(dates, normalizeDate(d))
		values = append(values, w)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Reverse to chronological order (query returns DESC).
	for i, j := 0, len(dates)-1; i < j; i, j = i+1, j-1 {
		dates[i], dates[j] = dates[j], dates[i]
		values[i], values[j] = values[j], values[i]
	}

	return computeChartPoints(dates, values, unit), nil
}

// TrainingMaxChartData returns chart data for a training max progression.
// Returns all TM records in chronological order.
func TrainingMaxChartData(db *sql.DB, athleteID, exerciseID int64, unit string) (*ChartData, error) {
	rows, err := db.Query(`
		SELECT effective_date, weight FROM training_maxes
		WHERE athlete_id = ? AND exercise_id = ?
		ORDER BY effective_date ASC
		LIMIT 100`, athleteID, exerciseID)
	if err != nil {
		return nil, fmt.Errorf("models: TM chart data: %w", err)
	}
	defer rows.Close()

	var dates []string
	var values []float64
	for rows.Next() {
		var d string
		var w float64
		if err := rows.Scan(&d, &w); err != nil {
			return nil, fmt.Errorf("models: scan TM chart: %w", err)
		}
		dates = append(dates, normalizeDate(d))
		values = append(values, w)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return computeChartPoints(dates, values, unit), nil
}

// ExerciseVolumeBar represents one session's volume for a bar chart.
type ExerciseVolumeBar struct {
	X      float64 // SVG x position
	Y      float64 // SVG y position (top of bar)
	Width  float64 // Bar width
	Height float64 // Bar height
	Volume float64 // Total volume (sets * reps * weight)
	Date   string  // Workout date for tooltip
}

// ExerciseVolumeChartData holds bar chart data for per-session exercise volume.
type ExerciseVolumeChartData struct {
	Bars    []ExerciseVolumeBar
	HasData bool
	MaxVol  float64
}

// ExerciseVolumeChart returns bar chart data for an exercise's per-session volume.
func ExerciseVolumeChart(db *sql.DB, athleteID, exerciseID int64, limit int) (*ExerciseVolumeChartData, error) {
	if limit <= 0 {
		limit = 20
	}

	rows, err := db.Query(`
		SELECT w.date, SUM(ws.reps * COALESCE(ws.weight, 0)) as volume
		FROM workout_sets ws
		JOIN workouts w ON w.id = ws.workout_id
		WHERE w.athlete_id = ? AND ws.exercise_id = ?
		GROUP BY w.date
		ORDER BY w.date DESC
		LIMIT ?`, athleteID, exerciseID, limit)
	if err != nil {
		return nil, fmt.Errorf("models: exercise volume chart: %w", err)
	}
	defer rows.Close()

	type session struct {
		date   string
		volume float64
	}
	var sessions []session
	for rows.Next() {
		var s session
		if err := rows.Scan(&s.date, &s.volume); err != nil {
			return nil, fmt.Errorf("models: scan exercise volume: %w", err)
		}
		s.date = normalizeDate(s.date)
		sessions = append(sessions, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(sessions) == 0 {
		return &ExerciseVolumeChartData{HasData: false}, nil
	}

	// Reverse to chronological order.
	for i, j := 0, len(sessions)-1; i < j; i, j = i+1, j-1 {
		sessions[i], sessions[j] = sessions[j], sessions[i]
	}

	maxVol := 0.0
	for _, s := range sessions {
		if s.volume > maxVol {
			maxVol = s.volume
		}
	}
	if maxVol == 0 {
		maxVol = 1
	}

	plotW := chartWidth - chartPadLeft - chartPadRight
	plotH := chartHeight - chartPadTop - chartPadBot
	barGap := 2.0
	barW := (plotW - barGap*float64(len(sessions)-1)) / float64(len(sessions))
	if barW > 40 {
		barW = 40
	}

	bars := make([]ExerciseVolumeBar, len(sessions))
	for i, s := range sessions {
		hFrac := s.volume / maxVol
		barH := hFrac * plotH
		bars[i] = ExerciseVolumeBar{
			X:      chartPadLeft + float64(i)*(barW+barGap),
			Y:      chartPadTop + plotH - barH,
			Width:  barW,
			Height: barH,
			Volume: s.volume,
			Date:   s.date,
		}
	}

	return &ExerciseVolumeChartData{
		Bars:    bars,
		HasData: true,
		MaxVol:  maxVol,
	}, nil
}

// HeatmapCell represents one day in a workout frequency heatmap.
type HeatmapCell struct {
	X      float64
	Y      float64
	Date   string
	Volume float64
	Level  int // 0 = no workout, 1-4 = intensity quartiles
}

// HeatmapData holds a full year of workout heatmap data.
type HeatmapData struct {
	Cells       []HeatmapCell
	HasData     bool
	MonthLabels []HeatmapMonthLabel
}

// HeatmapMonthLabel positions a month name along the top of the heatmap.
type HeatmapMonthLabel struct {
	X     float64
	Label string
}

// WorkoutHeatmap returns a GitHub-style heatmap of workout volume over the
// last 52 weeks for an athlete. Each cell is one day.
func WorkoutHeatmap(db *sql.DB, athleteID int64) (*HeatmapData, error) {
	// Calculate date range: 52 weeks back from today, aligned to Sunday starts.
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	// Find the start: go back 52 weeks, then back to the nearest Sunday.
	startDate := today.AddDate(0, 0, -52*7)
	for startDate.Weekday() != time.Sunday {
		startDate = startDate.AddDate(0, 0, -1)
	}

	endDate := today

	// Query workout volumes per day.
	rows, err := db.Query(`
		SELECT w.date, SUM(ws.reps * COALESCE(ws.weight, 0)) as volume
		FROM workouts w
		LEFT JOIN workout_sets ws ON ws.workout_id = w.id
		WHERE w.athlete_id = ?
		  AND date(w.date) >= date(?)
		  AND date(w.date) <= date(?)
		GROUP BY w.date`, athleteID, startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))
	if err != nil {
		return nil, fmt.Errorf("models: workout heatmap: %w", err)
	}
	defer rows.Close()

	volumeByDate := make(map[string]float64)
	maxVolume := 0.0
	for rows.Next() {
		var date string
		var vol float64
		if err := rows.Scan(&date, &vol); err != nil {
			return nil, fmt.Errorf("models: scan heatmap: %w", err)
		}
		volumeByDate[normalizeDate(date)] = vol
		if vol > maxVolume {
			maxVolume = vol
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Build cells grid: columns = weeks, rows = days of week (Sun=0 to Sat=6).
	cellSize := 13.0
	cellGap := 2.0
	cellStep := cellSize + cellGap

	var cells []HeatmapCell
	var monthLabels []HeatmapMonthLabel
	lastMonth := -1

	d := startDate
	for !d.After(endDate) {
		weekIdx := int(d.Sub(startDate).Hours()/24) / 7
		dayIdx := int(d.Weekday()) // 0=Sun, 6=Sat

		x := float64(weekIdx) * cellStep
		y := float64(dayIdx) * cellStep

		dateStr := d.Format("2006-01-02")
		vol := volumeByDate[dateStr]
		level := volumeToLevel(vol, maxVolume)

		cells = append(cells, HeatmapCell{
			X:      x,
			Y:      y,
			Date:   dateStr,
			Volume: vol,
			Level:  level,
		})

		// Add month labels on the first occurrence of each month.
		if d.Day() <= 7 && int(d.Month()) != lastMonth {
			monthLabels = append(monthLabels, HeatmapMonthLabel{
				X:     x,
				Label: d.Format("Jan"),
			})
			lastMonth = int(d.Month())
		}

		d = d.AddDate(0, 0, 1)
	}

	return &HeatmapData{
		Cells:       cells,
		HasData:     len(volumeByDate) > 0,
		MonthLabels: monthLabels,
	}, nil
}

// volumeToLevel converts a volume value to a 0-4 intensity level.
func volumeToLevel(vol, maxVol float64) int {
	if vol == 0 || maxVol == 0 {
		return 0
	}
	ratio := vol / maxVol
	switch {
	case ratio >= 0.75:
		return 4
	case ratio >= 0.50:
		return 3
	case ratio >= 0.25:
		return 2
	default:
		return 1
	}
}

// DashboardStats holds aggregate stats for the coach dashboard.
type DashboardStats struct {
	WeekSessions     int
	WeekVolume       float64
	TotalAthletes    int
	TrainedThisWeek  int
	ConsecutiveWeeks int // Streak of weeks where at least one workout was logged
}

// GetDashboardStats computes summary statistics for the coach dashboard.
func GetDashboardStats(db *sql.DB) (*DashboardStats, error) {
	stats := &DashboardStats{}

	now := time.Now()
	// Monday of this week.
	weekday := now.Weekday()
	if weekday == time.Sunday {
		weekday = 7
	}
	monday := now.AddDate(0, 0, -int(weekday-time.Monday))
	mondayStr := time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, time.UTC).Format("2006-01-02")

	// Total sessions this week.
	err := db.QueryRow(`
		SELECT COUNT(*) FROM workouts
		WHERE date(date) >= date(?)`, mondayStr).Scan(&stats.WeekSessions)
	if err != nil {
		return nil, fmt.Errorf("models: dashboard week sessions: %w", err)
	}

	// Total volume this week.
	err = db.QueryRow(`
		SELECT COALESCE(SUM(ws.reps * COALESCE(ws.weight, 0)), 0)
		FROM workout_sets ws
		JOIN workouts w ON w.id = ws.workout_id
		WHERE date(w.date) >= date(?)`, mondayStr).Scan(&stats.WeekVolume)
	if err != nil {
		return nil, fmt.Errorf("models: dashboard week volume: %w", err)
	}

	// Total athletes.
	err = db.QueryRow(`SELECT COUNT(*) FROM athletes`).Scan(&stats.TotalAthletes)
	if err != nil {
		return nil, fmt.Errorf("models: dashboard total athletes: %w", err)
	}

	// Distinct athletes who trained this week.
	err = db.QueryRow(`
		SELECT COUNT(DISTINCT athlete_id) FROM workouts
		WHERE date(date) >= date(?)`, mondayStr).Scan(&stats.TrainedThisWeek)
	if err != nil {
		return nil, fmt.Errorf("models: dashboard athletes trained: %w", err)
	}

	// Consecutive weeks streak (weeks with at least one workout, going backward).
	stats.ConsecutiveWeeks = computeWeekStreak(db, monday)

	return stats, nil
}

// computeWeekStreak counts consecutive past weeks (including current) with workouts.
func computeWeekStreak(db *sql.DB, currentMonday time.Time) int {
	streak := 0
	for i := 0; i < 52; i++ {
		weekStart := currentMonday.AddDate(0, 0, -i*7)
		weekEnd := weekStart.AddDate(0, 0, 6)

		var count int
		err := db.QueryRow(`
			SELECT COUNT(*) FROM workouts
			WHERE date(date) >= date(?) AND date(date) <= date(?)`,
			weekStart.Format("2006-01-02"), weekEnd.Format("2006-01-02")).Scan(&count)
		if err != nil || count == 0 {
			break
		}
		streak++
	}
	return streak
}
