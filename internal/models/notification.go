package models

import (
	"database/sql"
	"fmt"
	"time"
)

// Notification types — used as the `type` column in the notifications table.
const (
	NotifyReviewSubmitted = "review_submitted"
	NotifyProgramAssigned = "program_assigned"
	NotifyTMUpdated       = "tm_updated"
	NotifyNoteAdded       = "note_added"
	NotifyWorkoutLogged   = "workout_logged"
	NotifyMagicLinkSent   = "magic_link_sent"
)

// AllNotificationTypes lists all known notification types for preference UI.
var AllNotificationTypes = []NotificationType{
	{Type: NotifyReviewSubmitted, Label: "Workout Reviewed", Description: "When a coach reviews your workout"},
	{Type: NotifyProgramAssigned, Label: "Program Assigned", Description: "When a new program is assigned to you"},
	{Type: NotifyTMUpdated, Label: "Training Max Updated", Description: "When a training max is updated"},
	{Type: NotifyNoteAdded, Label: "Coach Note Added", Description: "When a coach adds a public note"},
	{Type: NotifyWorkoutLogged, Label: "Workout Logged", Description: "When an athlete logs a workout"},
	{Type: NotifyMagicLinkSent, Label: "Login Link Sent", Description: "When a login link is generated for you"},
}

// NotificationType describes a notification type for preference UI.
type NotificationType struct {
	Type        string
	Label       string
	Description string
}

// Notification represents an in-app notification row.
type Notification struct {
	ID        int64
	UserID    int64
	Type      string
	Title     string
	Message   sql.NullString
	Link      sql.NullString
	Read      bool
	AthleteID sql.NullInt64
	CreatedAt time.Time
}

// NotificationPreference represents a user's channel preference for a notification type.
type NotificationPreference struct {
	ID       int64
	UserID   int64
	Type     string
	InApp    bool
	External bool
}

// CreateNotification inserts a new notification. Returns the created notification.
func CreateNotification(db *sql.DB, userID int64, nType, title, message, link string, athleteID sql.NullInt64) (*Notification, error) {
	var msgVal, linkVal sql.NullString
	if message != "" {
		msgVal = sql.NullString{String: message, Valid: true}
	}
	if link != "" {
		linkVal = sql.NullString{String: link, Valid: true}
	}

	row := db.QueryRow(
		`INSERT INTO notifications (user_id, type, title, message, link, athlete_id)
		 VALUES (?, ?, ?, ?, ?, ?)
		 RETURNING id, user_id, type, title, message, link, read, athlete_id, created_at`,
		userID, nType, title, msgVal, linkVal, athleteID,
	)

	n := &Notification{}
	err := row.Scan(&n.ID, &n.UserID, &n.Type, &n.Title, &n.Message, &n.Link, &n.Read, &n.AthleteID, &n.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("models: create notification: %w", err)
	}
	return n, nil
}

// GetUnreadCount returns the number of unread notifications for a user.
func GetUnreadCount(db *sql.DB, userID int64) (int, error) {
	var count int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM notifications WHERE user_id = ? AND read = 0`,
		userID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("models: get unread count for user %d: %w", userID, err)
	}
	return count, nil
}

// GetUnreadNotifications returns up to `limit` unread notifications for a user,
// ordered newest first. Used for toast polling.
func GetUnreadNotifications(db *sql.DB, userID int64, limit int) ([]*Notification, error) {
	rows, err := db.Query(
		`SELECT id, user_id, type, title, message, link, read, athlete_id, created_at
		 FROM notifications
		 WHERE user_id = ? AND read = 0
		 ORDER BY created_at DESC
		 LIMIT ?`,
		userID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("models: get unread notifications for user %d: %w", userID, err)
	}
	defer rows.Close()

	return scanNotifications(rows)
}

// ListNotifications returns notifications for a user with pagination.
func ListNotifications(db *sql.DB, userID int64, limit, offset int) ([]*Notification, error) {
	rows, err := db.Query(
		`SELECT id, user_id, type, title, message, link, read, athlete_id, created_at
		 FROM notifications
		 WHERE user_id = ?
		 ORDER BY created_at DESC
		 LIMIT ? OFFSET ?`,
		userID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("models: list notifications for user %d: %w", userID, err)
	}
	defer rows.Close()

	return scanNotifications(rows)
}

// MarkAsRead marks a single notification as read.
func MarkAsRead(db *sql.DB, id, userID int64) error {
	result, err := db.Exec(
		`UPDATE notifications SET read = 1 WHERE id = ? AND user_id = ?`,
		id, userID,
	)
	if err != nil {
		return fmt.Errorf("models: mark notification %d as read: %w", id, err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("models: notification %d not found or not owned by user %d", id, userID)
	}
	return nil
}

// MarkAllAsRead marks all unread notifications as read for a user.
func MarkAllAsRead(db *sql.DB, userID int64) (int64, error) {
	result, err := db.Exec(
		`UPDATE notifications SET read = 1 WHERE user_id = ? AND read = 0`,
		userID,
	)
	if err != nil {
		return 0, fmt.Errorf("models: mark all read for user %d: %w", userID, err)
	}
	return result.RowsAffected()
}

// DeleteOldNotifications removes read notifications older than the given time.
func DeleteOldNotifications(db *sql.DB, olderThan time.Time) (int64, error) {
	result, err := db.Exec(
		`DELETE FROM notifications WHERE read = 1 AND created_at < ?`,
		olderThan,
	)
	if err != nil {
		return 0, fmt.Errorf("models: delete old notifications: %w", err)
	}
	return result.RowsAffected()
}

// GetNotificationsSince returns unread notifications created since the given time
// for a user. Used for toast polling — only show new notifications.
func GetNotificationsSince(db *sql.DB, userID int64, since time.Time) ([]*Notification, error) {
	rows, err := db.Query(
		`SELECT id, user_id, type, title, message, link, read, athlete_id, created_at
		 FROM notifications
		 WHERE user_id = ? AND read = 0 AND created_at > ?
		 ORDER BY created_at DESC
		 LIMIT 5`,
		userID, since,
	)
	if err != nil {
		return nil, fmt.Errorf("models: get notifications since for user %d: %w", userID, err)
	}
	defer rows.Close()

	return scanNotifications(rows)
}

// --- Notification Preferences ---

// GetNotificationPreference returns the preference for a user+type, or defaults.
func GetNotificationPreference(db *sql.DB, userID int64, nType string) NotificationPreference {
	pref := NotificationPreference{
		UserID:   userID,
		Type:     nType,
		InApp:    true,  // default
		External: false, // default
	}

	err := db.QueryRow(
		`SELECT id, in_app, external FROM notification_preferences WHERE user_id = ? AND type = ?`,
		userID, nType,
	).Scan(&pref.ID, &pref.InApp, &pref.External)
	if err != nil {
		// No row — return defaults.
		return pref
	}
	return pref
}

// ListNotificationPreferences returns all preferences for a user, filling in
// defaults for types without a stored preference.
func ListNotificationPreferences(db *sql.DB, userID int64) []NotificationPreference {
	// Load stored preferences.
	stored := make(map[string]NotificationPreference)
	rows, err := db.Query(
		`SELECT id, user_id, type, in_app, external FROM notification_preferences WHERE user_id = ?`,
		userID,
	)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var p NotificationPreference
			if err := rows.Scan(&p.ID, &p.UserID, &p.Type, &p.InApp, &p.External); err == nil {
				stored[p.Type] = p
			}
		}
	}

	// Build full list with defaults for missing types.
	var prefs []NotificationPreference
	for _, nt := range AllNotificationTypes {
		if p, ok := stored[nt.Type]; ok {
			prefs = append(prefs, p)
		} else {
			prefs = append(prefs, NotificationPreference{
				UserID:   userID,
				Type:     nt.Type,
				InApp:    true,
				External: false,
			})
		}
	}
	return prefs
}

// SetNotificationPreference upserts a preference for a user+type.
func SetNotificationPreference(db *sql.DB, userID int64, nType string, inApp, external bool) error {
	_, err := db.Exec(
		`INSERT INTO notification_preferences (user_id, type, in_app, external)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(user_id, type) DO UPDATE SET in_app = excluded.in_app, external = excluded.external`,
		userID, nType, inApp, external,
	)
	if err != nil {
		return fmt.Errorf("models: set notification preference for user %d type %q: %w", userID, nType, err)
	}
	return nil
}

// --- Helpers ---

func scanNotifications(rows *sql.Rows) ([]*Notification, error) {
	var notifications []*Notification
	for rows.Next() {
		n := &Notification{}
		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.Title, &n.Message, &n.Link, &n.Read, &n.AthleteID, &n.CreatedAt); err != nil {
			return nil, fmt.Errorf("models: scan notification: %w", err)
		}
		notifications = append(notifications, n)
	}
	return notifications, rows.Err()
}

// NotificationTypeLabel returns the human-readable label for a notification type.
func NotificationTypeLabel(nType string) string {
	for _, nt := range AllNotificationTypes {
		if nt.Type == nType {
			return nt.Label
		}
	}
	return nType
}
