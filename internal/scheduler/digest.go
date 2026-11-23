package scheduler

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/carpenike/replog/internal/models"
	"github.com/carpenike/replog/internal/notify"
)

// digestSendHour is the coach's local hour on Monday after which last
// week's digest goes out. The digest loop checks hourly, so it lands
// between 07:00 and 08:00, or at the next check after a restart.
const digestSendHour = 7

// digestCheckInterval is how often the digest loop looks for due digests.
const digestCheckInterval = time.Hour

// digestSender delivers one digest. Tests swap it to capture the request.
var digestSender = notify.Send

// digestLoop checks hourly for weekly digests that are due.
func (s *Scheduler) digestLoop() {
	defer s.wg.Done()
	ticker := time.NewTicker(digestCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.RunWeeklyDigests(time.Now())
		case <-s.stop:
			return
		}
	}
}

// RunWeeklyDigests sends last week's digest to every coach for whom it is
// due at now: it is Monday 07:00 or later in the coach's timezone, the
// digest for the previous Monday–Sunday has not gone out, and the coach has
// not switched the Weekly Coach Digest notification off. It returns how
// many digests were sent. The digest only reports; it never changes a plan.
func (s *Scheduler) RunWeeklyDigests(now time.Time) int {
	coaches, err := models.ListDigestCoaches(s.ctx, s.db)
	if err != nil {
		log.Printf("Digest: list coaches: %v", err)
		return 0
	}
	sent := 0
	for _, coach := range coaches {
		ok, err := s.sendDigest(coach, now)
		if err != nil {
			log.Printf("Digest: coach %d: %v", coach.ID, err)
			continue
		}
		if ok {
			sent++
		}
	}
	if sent > 0 {
		log.Printf("Digest: sent %d weekly digest(s)", sent)
	}
	return sent
}

func (s *Scheduler) sendDigest(coach *models.User, now time.Time) (bool, error) {
	pref := models.GetNotificationPreference(s.ctx, s.db, coach.ID, models.NotifyWeeklyDigest)
	if !pref.InApp && !pref.External {
		return false, nil
	}
	loc := coachLocation(s.ctx, s.db, coach.ID)
	local := now.In(loc)
	thisMonday := mondayOf(local)
	if local.Before(thisMonday.Add(digestSendHour * time.Hour)) {
		return false, nil
	}
	weekStart := thisMonday.AddDate(0, 0, -7)
	weekStartStr := weekStart.Format("2006-01-02")
	if done, err := models.WeeklyDigestSent(s.ctx, s.db, coach.ID, weekStartStr); err != nil || done {
		return false, err
	}

	athletes, err := models.ListAthletes(s.ctx, s.db, sql.NullInt64{Int64: coach.ID, Valid: true})
	if err != nil {
		return false, err
	}
	if len(athletes) == 0 {
		return false, nil
	}
	today := local.Format("2006-01-02")
	digests := make([]*models.DigestAthlete, 0, len(athletes))
	for _, a := range athletes {
		d, err := models.BuildDigestAthlete(s.ctx, s.db, a, weekStartStr, today, loc)
		if err != nil {
			return false, fmt.Errorf("athlete %d: %w", a.ID, err)
		}
		digests = append(digests, d)
	}

	claimed, err := models.ClaimWeeklyDigest(s.ctx, s.db, coach.ID, weekStartStr)
	if err != nil || !claimed {
		return false, err
	}
	title, message := FormatDigest(weekStart, digests)
	digestSender(s.ctx, s.db, notify.Request{
		UserID:  coach.ID,
		Type:    models.NotifyWeeklyDigest,
		Title:   title,
		Message: message,
		Link:    "/coach/compliance?week=" + weekStartStr,
	})
	return true, nil
}

func coachLocation(ctx context.Context, db *sql.DB, userID int64) *time.Location {
	tz := models.DefaultTimezone
	if prefs, err := models.GetUserPreferences(ctx, db, userID); err == nil && prefs != nil && prefs.Timezone != "" {
		tz = prefs.Timezone
	}
	if loc, err := time.LoadLocation(tz); err == nil {
		return loc
	}
	return time.UTC
}

// mondayOf returns midnight on the Monday of t's week, in t's location.
func mondayOf(t time.Time) time.Time {
	offset := (int(t.Weekday()) + 6) % 7
	return time.Date(t.Year(), t.Month(), t.Day()-offset, 0, 0, 0, 0, t.Location())
}

// FormatDigest renders the digest as a title and a plain-text message, one
// block per athlete. Copy follows docs/COACH_VOICE.md: it reports what
// happened and leaves every decision to the coach.
func FormatDigest(weekStart time.Time, athletes []*models.DigestAthlete) (title, message string) {
	weekEnd := weekStart.AddDate(0, 0, 6)
	span := weekStart.Format("Jan 2") + "–" + weekEnd.Format("Jan 2")
	if weekStart.Month() == weekEnd.Month() {
		span = weekStart.Format("Jan 2") + "–" + weekEnd.Format("2")
	}
	title = "Weekly digest: " + span

	var b strings.Builder
	for i, a := range athletes {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(a.Name + ": " + planLine(a.Plan) + "\n")
		if len(a.NewPRs) > 0 {
			parts := make([]string, len(a.NewPRs))
			for j, pr := range a.NewPRs {
				parts[j] = prText(pr)
			}
			b.WriteString("  New PRs: " + strings.Join(parts, "; ") + "\n")
		}
		if len(a.LowReadinessDates) > 0 {
			days := make([]string, len(a.LowReadinessDates))
			for j, d := range a.LowReadinessDates {
				days[j] = shortDay(d)
			}
			b.WriteString("  Low readiness check-in: " + strings.Join(days, ", ") + "\n")
		}
		if a.PitchSmart != nil {
			b.WriteString("  Pitch Smart: " + a.PitchSmart.Advisory + "\n")
		}
	}
	b.WriteString("\nFor your review. RepLog does not change any plan on its own.")
	return title, b.String()
}

func planLine(p *models.WeekPlan) string {
	if p == nil || p.Planned == 0 {
		return "no sessions were scheduled."
	}
	line := fmt.Sprintf("%d of %d planned sessions done", p.Done, p.Planned)
	var extra []string
	if p.Partial > 0 {
		extra = append(extra, fmt.Sprintf("%d partial", p.Partial))
	}
	if p.Missed > 0 {
		extra = append(extra, fmt.Sprintf("%d skipped", p.Missed))
	}
	if p.Upcoming > 0 {
		extra = append(extra, fmt.Sprintf("%d not yet due", p.Upcoming))
	}
	if len(extra) > 0 {
		line += " (" + strings.Join(extra, ", ") + ")"
	}
	return line + "."
}

func prText(pr models.DigestPR) string {
	var s string
	if pr.Weight > 0 {
		s = fmt.Sprintf("%s %s lb × %d", pr.Exercise, trimFloat(pr.Weight), pr.Reps)
	} else {
		s = fmt.Sprintf("%s %d reps", pr.Exercise, pr.Reps)
	}
	if pr.E1RM > 0 {
		s += fmt.Sprintf(" (e1RM %d lb)", int(math.Round(pr.E1RM)))
	}
	return s
}

func trimFloat(f float64) string {
	return strings.TrimSuffix(strings.TrimRight(fmt.Sprintf("%.1f", f), "0"), ".")
}

func shortDay(date string) string {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date
	}
	return t.Format("Mon Jan 2")
}
