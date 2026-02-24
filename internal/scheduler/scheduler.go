package scheduler

import (
	"database/sql"
	"log"
	"sync"
	"time"

	"github.com/carpenike/replog/internal/models"
)

// Status holds the result of the last maintenance run.
type Status struct {
	LastRun           time.Time
	NextRun           time.Time
	TokensDeleted     int64
	NotificationsPruned int64
	IntervalHours     int
	RetentionDays     int
}

// Scheduler runs periodic maintenance tasks in the background.
type Scheduler struct {
	db   *sql.DB
	stop chan struct{}
	done chan struct{}

	mu     sync.RWMutex
	status Status
}

// New creates a new Scheduler for the given database.
func New(db *sql.DB) *Scheduler {
	return &Scheduler{
		db:   db,
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
}

// Start begins running maintenance tasks. It runs an initial pass immediately,
// then repeats at the configured interval. Call Stop to shut down gracefully.
func (s *Scheduler) Start() {
	go s.run()
	log.Println("Background scheduler started")
}

// Stop signals the scheduler to shut down and waits for it to finish.
func (s *Scheduler) Stop() {
	close(s.stop)
	<-s.done
}

// Status returns the result of the last maintenance run.
func (s *Scheduler) Status() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

func (s *Scheduler) run() {
	defer close(s.done)

	// Run immediately on startup, then at the configured interval.
	s.runMaintenance()

	for {
		interval := s.getInterval()
		ticker := time.NewTicker(interval)

		select {
		case <-ticker.C:
			ticker.Stop()
			s.runMaintenance()
		case <-s.stop:
			ticker.Stop()
			return
		}
	}
}

// getInterval reads the configured interval from app settings.
func (s *Scheduler) getInterval() time.Duration {
	hours := models.GetMaintenanceIntervalHours(s.db)
	return time.Duration(hours) * time.Hour
}

// getRetention reads the configured retention period from app settings.
func (s *Scheduler) getRetention() time.Duration {
	days := models.GetMaintenanceRetentionDays(s.db)
	return time.Duration(days) * 24 * time.Hour
}

// runMaintenance executes all periodic cleanup tasks.
func (s *Scheduler) runMaintenance() {
	log.Println("Running scheduled maintenance...")

	tokensDeleted := s.cleanExpiredTokens()
	notifsPruned := s.pruneOldNotifications()

	now := time.Now()
	interval := s.getInterval()

	s.mu.Lock()
	s.status = Status{
		LastRun:             now,
		NextRun:             now.Add(interval),
		TokensDeleted:       tokensDeleted,
		NotificationsPruned: notifsPruned,
		IntervalHours:       models.GetMaintenanceIntervalHours(s.db),
		RetentionDays:       models.GetMaintenanceRetentionDays(s.db),
	}
	s.mu.Unlock()

	log.Println("Scheduled maintenance complete")
}

// cleanExpiredTokens removes login tokens past their expiry date.
func (s *Scheduler) cleanExpiredTokens() int64 {
	deleted, err := models.DeleteExpiredLoginTokens(s.db)
	if err != nil {
		log.Printf("Maintenance: clean expired tokens: %v", err)
		return 0
	}
	if deleted > 0 {
		log.Printf("Maintenance: deleted %d expired login token(s)", deleted)
	}
	return deleted
}

// pruneOldNotifications removes read notifications older than the configured retention period.
func (s *Scheduler) pruneOldNotifications() int64 {
	cutoff := time.Now().Add(-s.getRetention())
	deleted, err := models.DeleteOldNotifications(s.db, cutoff)
	if err != nil {
		log.Printf("Maintenance: prune old notifications: %v", err)
		return 0
	}
	if deleted > 0 {
		log.Printf("Maintenance: pruned %d old notification(s)", deleted)
	}
	return deleted
}
