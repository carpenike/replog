package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// PushSubscription is one browser Web Push subscription for a user.
type PushSubscription struct {
	ID            int64
	UserID        int64
	Endpoint      string
	P256dh        string
	Auth          string
	UserAgent     sql.NullString
	CreatedAt     time.Time
	LastSuccessAt sql.NullTime
}

// UpsertPushSubscription stores a subscription for userID. The endpoint is
// unique per browser, so re-subscribing refreshes the keys, and a device
// that is now signed in as a different user moves to that user.
func UpsertPushSubscription(ctx context.Context, db *sql.DB, userID int64, endpoint, p256dh, auth, userAgent string) (*PushSubscription, error) {
	var ua sql.NullString
	if userAgent != "" {
		ua = sql.NullString{String: userAgent, Valid: true}
	}
	sub := &PushSubscription{}
	err := db.QueryRowContext(ctx,
		`INSERT INTO push_subscriptions (user_id, endpoint, p256dh, auth, user_agent)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(endpoint) DO UPDATE SET
		     user_id = excluded.user_id, p256dh = excluded.p256dh,
		     auth = excluded.auth, user_agent = excluded.user_agent
		 RETURNING id, user_id, endpoint, p256dh, auth, user_agent, created_at, last_success_at`,
		userID, endpoint, p256dh, auth, ua,
	).Scan(&sub.ID, &sub.UserID, &sub.Endpoint, &sub.P256dh, &sub.Auth, &sub.UserAgent, &sub.CreatedAt, &sub.LastSuccessAt)
	if err != nil {
		return nil, fmt.Errorf("models: upsert push subscription for user %d: %w", userID, err)
	}
	return sub, nil
}

// ListPushSubscriptions returns userID's subscriptions, oldest first.
func ListPushSubscriptions(ctx context.Context, db *sql.DB, userID int64) ([]*PushSubscription, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, user_id, endpoint, p256dh, auth, user_agent, created_at, last_success_at
		   FROM push_subscriptions WHERE user_id = ? ORDER BY id`, userID)
	if err != nil {
		return nil, fmt.Errorf("models: list push subscriptions for user %d: %w", userID, err)
	}
	defer rows.Close()
	var out []*PushSubscription
	for rows.Next() {
		s := &PushSubscription{}
		if err := rows.Scan(&s.ID, &s.UserID, &s.Endpoint, &s.P256dh, &s.Auth, &s.UserAgent, &s.CreatedAt, &s.LastSuccessAt); err != nil {
			return nil, fmt.Errorf("models: scan push subscription: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// DeletePushSubscription removes userID's subscription for endpoint.
// Scoped to the user so nobody can unsubscribe another user's device.
// Returns ErrNotFound when there was no such row.
func DeletePushSubscription(ctx context.Context, db *sql.DB, userID int64, endpoint string) error {
	res, err := db.ExecContext(ctx,
		`DELETE FROM push_subscriptions WHERE user_id = ? AND endpoint = ?`, userID, endpoint)
	if err != nil {
		return fmt.Errorf("models: delete push subscription: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeletePushSubscriptionByID removes a subscription the push service
// reported as gone (404/410).
func DeletePushSubscriptionByID(ctx context.Context, db *sql.DB, id int64) error {
	if _, err := db.ExecContext(ctx, `DELETE FROM push_subscriptions WHERE id = ?`, id); err != nil {
		return fmt.Errorf("models: delete push subscription %d: %w", id, err)
	}
	return nil
}

// MarkPushSubscriptionSuccess records a delivered push.
func MarkPushSubscriptionSuccess(ctx context.Context, db *sql.DB, id int64) error {
	if _, err := db.ExecContext(ctx,
		`UPDATE push_subscriptions SET last_success_at = CURRENT_TIMESTAMP WHERE id = ?`, id); err != nil {
		return fmt.Errorf("models: mark push subscription %d: %w", id, err)
	}
	return nil
}

// ErrVAPIDKeyLocked means the stored VAPID private key is encrypted but
// REPLOG_SECRET_KEY is missing or wrong, so it cannot be read.
var ErrVAPIDKeyLocked = errors.New("models: stored VAPID private key is encrypted and REPLOG_SECRET_KEY cannot decrypt it")

// LoadOrCreateVAPIDKeys returns the persisted VAPID key pair, creating it
// with generate on first use. The private key is encrypted at rest when
// REPLOG_SECRET_KEY is set (the same scheme as sensitive app settings).
func LoadOrCreateVAPIDKeys(ctx context.Context, db *sql.DB, generate func() (public, private string, err error)) (public, private string, created bool, err error) {
	var storedPriv string
	err = db.QueryRowContext(ctx, `SELECT public_key, private_key FROM vapid_keys WHERE id = 1`).Scan(&public, &storedPriv)
	switch {
	case err == nil:
		private, err = openVAPIDPrivate(storedPriv)
		return public, private, false, err
	case !errors.Is(err, sql.ErrNoRows):
		return "", "", false, fmt.Errorf("models: read vapid keys: %w", err)
	}

	public, private, err = generate()
	if err != nil {
		return "", "", false, err
	}
	store := private
	if secretKey() != nil {
		enc, err := encryptValue(private)
		if err != nil {
			return "", "", false, fmt.Errorf("models: encrypt vapid key: %w", err)
		}
		store = "enc:" + enc
	}
	// INSERT OR IGNORE plus a re-read keeps two racing starts on one pair.
	if _, err := db.ExecContext(ctx,
		`INSERT OR IGNORE INTO vapid_keys (id, public_key, private_key) VALUES (1, ?, ?)`, public, store); err != nil {
		return "", "", false, fmt.Errorf("models: store vapid keys: %w", err)
	}
	var gotPub, gotPriv string
	if err := db.QueryRowContext(ctx, `SELECT public_key, private_key FROM vapid_keys WHERE id = 1`).Scan(&gotPub, &gotPriv); err != nil {
		return "", "", false, fmt.Errorf("models: re-read vapid keys: %w", err)
	}
	if gotPub != public {
		private, err = openVAPIDPrivate(gotPriv)
		return gotPub, private, false, err
	}
	return public, private, true, nil
}

func openVAPIDPrivate(stored string) (string, error) {
	if !strings.HasPrefix(stored, "enc:") {
		return stored, nil
	}
	plain, err := decryptValue(stored[4:])
	if err != nil {
		return "", ErrVAPIDKeyLocked
	}
	return plain, nil
}
