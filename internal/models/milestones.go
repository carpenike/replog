package models

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"
)

// Milestone kinds. Milestones only ever compare an athlete with their own
// history. They are never ranked against other athletes.
const (
	MilestoneKindTier        = "tier"
	MilestoneKindFirst       = "first"
	MilestoneKindConsistency = "consistency"
)

// Thresholds for the consistency milestones.
var (
	MilestoneSessionTargets = []int{5, 10, 25, 50, 75, 100, 150, 200, 300, 400, 500, 750, 1000}
	MilestoneStreakTargets  = []int{2, 4, 8, 12, 26, 52}
	MilestonePRTargets      = []int{5, 10, 25, 50, 100, 200}
)

// milestoneFullWeek is the session count that makes a "full week".
const milestoneFullWeek = 3

// Milestone is one thing an athlete achieved, dated the day they did it.
type Milestone struct {
	Key        string
	Kind       string
	Title      string
	Detail     string
	AchievedOn string // YYYY-MM-DD
}

// MilestoneGoal is the next consistency milestone and how close it is.
type MilestoneGoal struct {
	Key     string
	Title   string
	Current int
	Target  int
}

// AthleteMilestones is an athlete's milestone summary.
type AthleteMilestones struct {
	Achieved      []Milestone // newest first
	Next          []MilestoneGoal
	TotalSessions int
	CurrentStreak int // consecutive Monday-based weeks, with last-week grace
	BestStreak    int
	PRCount       int
}

// TierLabel is the display name of an athlete tier.
func TierLabel(tier string) string {
	switch tier {
	case "foundational":
		return "Foundational"
	case "intermediate":
		return "Intermediate"
	case "sport_performance":
		return "Sport Performance"
	}
	return tier
}

type milestoneSession struct {
	id   int64
	date string
}

// AthleteMilestonesFor computes an athlete's milestones from their own
// logged history: tier badges from tier_history, firsts (first session,
// first PR, first full week, first coach-approved session), and
// consistency milestones (session counts, week streaks, PR counts).
// A session is a resistance workout with at least one counted (non
// warm-up) set. today is YYYY-MM-DD in the viewer's time zone and only
// anchors the current streak. Nothing here changes a program or a tier.
func AthleteMilestonesFor(ctx context.Context, db *sql.DB, athleteID int64, today string) (*AthleteMilestones, error) {
	athlete, err := GetAthleteByID(ctx, db, athleteID)
	if err != nil {
		return nil, err
	}
	out := &AthleteMilestones{Achieved: []Milestone{}, Next: []MilestoneGoal{}}

	if err := addTierMilestones(ctx, db, athlete, out); err != nil {
		return nil, err
	}

	sessions, err := milestoneSessions(ctx, db, athleteID)
	if err != nil {
		return nil, err
	}
	out.TotalSessions = len(sessions)
	if len(sessions) > 0 {
		out.Achieved = append(out.Achieved, Milestone{
			Key: "first_session", Kind: MilestoneKindFirst,
			Title: "First session", Detail: "Logged the first workout.", AchievedOn: sessions[0].date,
		})
	}
	for _, target := range MilestoneSessionTargets {
		if len(sessions) < target {
			out.Next = append(out.Next, MilestoneGoal{
				Key: fmt.Sprintf("sessions_%d", target), Title: fmt.Sprintf("%d sessions", target),
				Current: len(sessions), Target: target,
			})
			break
		}
		out.Achieved = append(out.Achieved, Milestone{
			Key: fmt.Sprintf("sessions_%d", target), Kind: MilestoneKindConsistency,
			Title:      fmt.Sprintf("%d sessions", target),
			Detail:     fmt.Sprintf("Logged %d training sessions.", target),
			AchievedOn: sessions[target-1].date,
		})
	}

	addWeekMilestones(sessions, today, out)

	if err := addPRMilestones(ctx, db, athleteID, out); err != nil {
		return nil, err
	}

	var approved sql.NullString
	if err := db.QueryRowContext(ctx, `
		SELECT MIN(date(w.date))
		  FROM workout_reviews r
		  JOIN workouts w ON w.id = r.workout_id
		 WHERE w.athlete_id = ? AND r.status = 'approved'`, athleteID,
	).Scan(&approved); err != nil {
		return nil, fmt.Errorf("models: first approved review for athlete %d: %w", athleteID, err)
	}
	if approved.Valid && approved.String != "" {
		out.Achieved = append(out.Achieved, Milestone{
			Key: "first_approved_review", Kind: MilestoneKindFirst,
			Title: "First coach sign-off", Detail: "A coach approved a logged session.", AchievedOn: isoDate(approved.String),
		})
	}

	sortMilestones(out.Achieved)
	return out, nil
}

func addTierMilestones(ctx context.Context, db *sql.DB, athlete *Athlete, out *AthleteMilestones) error {
	rows, err := db.QueryContext(ctx, `
		SELECT tier, previous_tier, effective_date
		  FROM tier_history
		 WHERE athlete_id = ?
		 ORDER BY date(effective_date), created_at, id`, athlete.ID)
	if err != nil {
		return fmt.Errorf("models: tier milestones for athlete %d: %w", athlete.ID, err)
	}
	defer rows.Close()
	seen := map[string]bool{}
	for rows.Next() {
		var tier, date string
		var prev sql.NullString
		if err := rows.Scan(&tier, &prev, &date); err != nil {
			return fmt.Errorf("models: scan tier milestone: %w", err)
		}
		if seen[tier] {
			continue
		}
		seen[tier] = true
		detail := "Starting tier, set by the coach."
		if prev.Valid && prev.String != "" {
			detail = fmt.Sprintf("Moved up from %s.", TierLabel(prev.String))
		}
		out.Achieved = append(out.Achieved, Milestone{
			Key: "tier_" + tier, Kind: MilestoneKindTier,
			Title: "Reached " + TierLabel(tier), Detail: detail, AchievedOn: isoDate(date),
		})
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if athlete.Tier.Valid && athlete.Tier.String != "" && !seen[athlete.Tier.String] {
		out.Achieved = append(out.Achieved, Milestone{
			Key: "tier_" + athlete.Tier.String, Kind: MilestoneKindTier,
			Title: "Reached " + TierLabel(athlete.Tier.String), Detail: "Current tier, set by the coach.",
			AchievedOn: athlete.CreatedAt.UTC().Format("2006-01-02"),
		})
	}
	return nil
}

func milestoneSessions(ctx context.Context, db *sql.DB, athleteID int64) ([]milestoneSession, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT w.id, w.date
		  FROM workouts w
		 WHERE w.athlete_id = ?
		   AND w.discipline = 'resistance'
		   AND EXISTS (SELECT 1 FROM workout_sets ws WHERE ws.workout_id = w.id AND `+countedSet+`)
		 ORDER BY date(w.date), w.id`, athleteID)
	if err != nil {
		return nil, fmt.Errorf("models: milestone sessions for athlete %d: %w", athleteID, err)
	}
	defer rows.Close()
	var out []milestoneSession
	for rows.Next() {
		var s milestoneSession
		if err := rows.Scan(&s.id, &s.date); err != nil {
			return nil, fmt.Errorf("models: scan milestone session: %w", err)
		}
		s.date = isoDate(s.date)
		out = append(out, s)
	}
	return out, rows.Err()
}

// mondayIndex returns the Monday-based week number of a YYYY-MM-DD date
// (days since the Unix epoch Monday, divided by seven), or false.
func mondayIndex(date string) (int, bool) {
	t, err := time.ParseInLocation("2006-01-02", isoDate(date), time.UTC)
	if err != nil {
		return 0, false
	}
	// 1970-01-05 was a Monday.
	days := int(t.Sub(time.Date(1970, 1, 5, 0, 0, 0, 0, time.UTC)).Hours() / 24)
	if days < 0 {
		return (days - 6) / 7, true
	}
	return days / 7, true
}

// addWeekMilestones adds the streak and full-week milestones and the
// current and best week streaks. Sessions are in date order.
func addWeekMilestones(sessions []milestoneSession, today string, out *AthleteMilestones) {
	weekCount := map[int]int{}
	var weeks []int // distinct, ascending
	firstDate := map[int]string{}
	fullWeekDone := false
	for _, s := range sessions {
		wk, ok := mondayIndex(s.date)
		if !ok {
			continue
		}
		if _, seen := weekCount[wk]; !seen {
			weeks = append(weeks, wk)
			firstDate[wk] = s.date
		}
		weekCount[wk]++
		if !fullWeekDone && weekCount[wk] == milestoneFullWeek {
			fullWeekDone = true
			out.Achieved = append(out.Achieved, Milestone{
				Key: "first_full_week", Kind: MilestoneKindFirst,
				Title:      fmt.Sprintf("First %d-session week", milestoneFullWeek),
				Detail:     fmt.Sprintf("Trained %d times in one week.", milestoneFullWeek),
				AchievedOn: s.date,
			})
		}
	}

	// Best streak, and the day each streak threshold was first reached.
	reached := map[int]string{}
	run := 0
	for i, wk := range weeks {
		if i > 0 && wk == weeks[i-1]+1 {
			run++
		} else {
			run = 1
		}
		if run > out.BestStreak {
			out.BestStreak = run
		}
		for _, target := range MilestoneStreakTargets {
			if run == target {
				if _, ok := reached[target]; !ok {
					reached[target] = firstDate[wk]
				}
			}
		}
	}

	// Current streak: the current week extends it but never breaks it.
	if cur, ok := mondayIndex(today); ok {
		start := cur
		if weekCount[cur] == 0 {
			start = cur - 1
		}
		for wk := start; weekCount[wk] > 0; wk-- {
			out.CurrentStreak++
		}
	}

	for _, target := range MilestoneStreakTargets {
		date, ok := reached[target]
		if !ok {
			out.Next = append(out.Next, MilestoneGoal{
				Key: fmt.Sprintf("streak_%d", target), Title: fmt.Sprintf("%d-week streak", target),
				Current: out.CurrentStreak, Target: target,
			})
			break
		}
		out.Achieved = append(out.Achieved, Milestone{
			Key: fmt.Sprintf("streak_%d", target), Kind: MilestoneKindConsistency,
			Title:      fmt.Sprintf("%d-week streak", target),
			Detail:     fmt.Sprintf("Trained at least once a week for %d weeks in a row.", target),
			AchievedOn: date,
		})
	}
}

// addPRMilestones counts sessions whose estimated 1RM beat every earlier
// session of the same exercise. An exercise's first session sets the
// baseline and is not a PR.
func addPRMilestones(ctx context.Context, db *sql.DB, athleteID int64, out *AthleteMilestones) error {
	progress, err := ListCoachProgress(ctx, db, athleteID, 0, "", "")
	if err != nil {
		return err
	}
	var dates []string
	for _, ex := range progress {
		if len(ex.PRs) > 1 {
			for _, pr := range ex.PRs[1:] {
				dates = append(dates, pr.Date)
			}
		}
	}
	sort.Strings(dates)
	out.PRCount = len(dates)
	if len(dates) > 0 {
		out.Achieved = append(out.Achieved, Milestone{
			Key: "first_pr", Kind: MilestoneKindFirst,
			Title: "First PR", Detail: "Beat a previous best on a lift.", AchievedOn: dates[0],
		})
	}
	for _, target := range MilestonePRTargets {
		if len(dates) < target {
			out.Next = append(out.Next, MilestoneGoal{
				Key: fmt.Sprintf("prs_%d", target), Title: fmt.Sprintf("%d PRs", target),
				Current: len(dates), Target: target,
			})
			break
		}
		out.Achieved = append(out.Achieved, Milestone{
			Key: fmt.Sprintf("prs_%d", target), Kind: MilestoneKindConsistency,
			Title:      fmt.Sprintf("%d PRs", target),
			Detail:     fmt.Sprintf("Beat a previous best %d times.", target),
			AchievedOn: dates[target-1],
		})
	}
	return nil
}

var milestoneKindOrder = map[string]int{MilestoneKindTier: 0, MilestoneKindConsistency: 1, MilestoneKindFirst: 2}

// sortMilestones orders newest first; same-day ties put tier badges first.
func sortMilestones(list []Milestone) {
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].AchievedOn != list[j].AchievedOn {
			return list[i].AchievedOn > list[j].AchievedOn
		}
		return milestoneKindOrder[list[i].Kind] < milestoneKindOrder[list[j].Kind]
	})
}
