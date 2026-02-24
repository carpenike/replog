package scheduler

import (
	"database/sql"
	"log"
	"time"

	"github.com/carpenike/replog/internal/models"
)

// Scheduler runs periodic maintenance tasks in the background.
type Scheduler struct {
	db   *sql.DB
	stop chan struct{}
	done chan struct{}
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
// then repeats every 24 hours. Call Stop to shut down gracefully.
func (s *Scheduler) Start() {
	go s.run()
	log.Println("Background scheduler started (daily maintenance)")
}

// Stop signals the scheduler to shut down and waits for it to finish.
func (s *Scheduler) Stop() {
	close(s.stop)
	<-s.done
}

func (s *Scheduler) run() {
	defer close(s.done)

	// Run immediately on startup, then daily.
	s.runMaintenance()

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.runMaintenance()
		case <-s.stop:
			return
		}
	}
}

// runMaintenance executes all periodic cleanup tasks.
func (s *Scheduler) runMaintenance() {
	log.Println("Running scheduled maintenance...")

	s.cleanExpiredTokens()
	s.pruneOldNotifications()

	log.Println("Scheduled maintenance complete")
}

// cleanExpiredTokens removes login tokens past their expiry date.
func (s *Scheduler) cleanExpiredTokens() {
	deleted, err := models.DeleteExpiredLoginTokens(s.db)
	if err != nil {
		log.Printf("Maintenance: clean expired tokens: %v", err)
		return
	}
	if deleted > 0 {
		log.Printf("Maintenance: deleted %d expired login token(s)", deleted)
	}
}

// pruneOldNotifications removes read notifications older than 90 days.
func (s *Scheduler) pruneOldNotifications() {
	cutoff := time.Now().Add(-90 * 24 * time.Hour)
	deleted, err := models.DeleteOldNotifications(s.db, cutoff)
	if err != nil {
		log.Printf("Maintenance: prune old notifications: %v", err)
		return
	}
	if deleted > 0 {
		log.Printf("Maintenance: pruned %d old notification(s)", deleted)
	}
}
