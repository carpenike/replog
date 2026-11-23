package models

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"
)

// Share-link limits. A link lives for one of the lifetimes in
// VideoShareLifetimes; anything longer than MaxVideoShareLifetime is refused.
const (
	// VideoShareTokenBytes is the number of random bytes in a share token
	// (256 bits). The token is base64url without padding: 43 characters.
	VideoShareTokenBytes = 32
	// MaxVideoShareLabelLen caps the optional label on a link.
	MaxVideoShareLabelLen = 100
	// DefaultVideoShareLifetime is the expiry when the caller picks none.
	DefaultVideoShareLifetime = 7 * 24 * time.Hour
	// MaxVideoShareLifetime is the longest a link may live.
	MaxVideoShareLifetime = 30 * 24 * time.Hour
	// maxShareTokenLen bounds how much of a presented token is hashed.
	maxShareTokenLen = 128
)

// VideoShareLifetimes are the expiry choices a link can be created with.
var VideoShareLifetimes = []time.Duration{24 * time.Hour, 7 * 24 * time.Hour, 30 * 24 * time.Hour}

// ValidVideoShareLifetime reports whether d is one of VideoShareLifetimes.
func ValidVideoShareLifetime(d time.Duration) bool {
	if d <= 0 || d > MaxVideoShareLifetime {
		return false
	}
	for _, l := range VideoShareLifetimes {
		if d == l {
			return true
		}
	}
	return false
}

// VideoShare is a no-login link to one form-check clip. TokenHash is the
// SHA-256 of the token; the token itself is never stored.
type VideoShare struct {
	ID             int64
	VideoID        int64
	TokenHash      string
	Label          sql.NullString
	CreatedBy      sql.NullInt64
	CreatorName    sql.NullString
	CreatedAt      time.Time
	ExpiresAt      time.Time
	RevokedAt      sql.NullTime
	RevokedBy      sql.NullInt64
	LastAccessedAt sql.NullTime
	AccessCount    int64
}

// Share-link states, as reported by VideoShare.Status.
const (
	VideoShareActive  = "active"
	VideoShareExpired = "expired"
	VideoShareRevoked = "revoked"
)

// Status is "revoked", "expired" or "active" at now.
func (s *VideoShare) Status(now time.Time) string {
	switch {
	case s.RevokedAt.Valid:
		return VideoShareRevoked
	case !now.Before(s.ExpiresAt):
		return VideoShareExpired
	default:
		return VideoShareActive
	}
}

// SharedVideo is what a valid share token resolves to: the link, the clip
// file to stream, and the only metadata a viewer sees (exercise and date).
type SharedVideo struct {
	ShareID      int64
	VideoID      int64
	ExpiresAt    time.Time
	FileName     string
	ContentType  string
	ExerciseName string
	Date         string
}

// HashVideoShareToken returns the lowercase-hex SHA-256 of a share token.
func HashVideoShareToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// newVideoShareToken returns a fresh random base64url token.
func newVideoShareToken() (string, error) {
	b := make([]byte, VideoShareTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("models: generate share token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// wellFormedShareToken reports whether token looks like one this package
// issues: 32 bytes of base64url without padding.
func wellFormedShareToken(token string) bool {
	if len(token) != base64.RawURLEncoding.EncodedLen(VideoShareTokenBytes) {
		return false
	}
	b, err := base64.RawURLEncoding.DecodeString(token)
	return err == nil && len(b) == VideoShareTokenBytes
}

const videoShareSelect = `SELECT s.id, s.video_id, s.token_hash, s.label, s.created_by, COALESCE(u.name, u.username),
       s.created_at, s.expires_at, s.revoked_at, s.revoked_by, s.last_accessed_at, s.access_count
  FROM set_video_shares s
  LEFT JOIN users u ON u.id = s.created_by`

func scanVideoShare(sc interface{ Scan(...any) error }) (*VideoShare, error) {
	s := &VideoShare{}
	err := sc.Scan(&s.ID, &s.VideoID, &s.TokenHash, &s.Label, &s.CreatedBy, &s.CreatorName,
		&s.CreatedAt, &s.ExpiresAt, &s.RevokedAt, &s.RevokedBy, &s.LastAccessedAt, &s.AccessCount)
	return s, err
}

// CreateVideoShare mints a link to clip videoID that expires lifetime after
// now. It returns the stored row and the plaintext token, which is the only
// copy: the row holds its hash. The caller checks access to the clip first.
func CreateVideoShare(ctx context.Context, db *sql.DB, videoID, createdBy int64, label string, lifetime time.Duration, now time.Time) (*VideoShare, string, error) {
	if !ValidVideoShareLifetime(lifetime) {
		return nil, "", fmt.Errorf("%w: share lifetime must be 24 hours, 7 days or 30 days", ErrInvalidInput)
	}
	if utf8.RuneCountInString(label) > MaxVideoShareLabelLen {
		return nil, "", fmt.Errorf("%w: label must be at most %d characters", ErrInvalidInput, MaxVideoShareLabelLen)
	}
	token, err := newVideoShareToken()
	if err != nil {
		return nil, "", err
	}
	var labelVal sql.NullString
	if label != "" {
		labelVal = sql.NullString{String: label, Valid: true}
	}
	var creator sql.NullInt64
	if createdBy > 0 {
		creator = sql.NullInt64{Int64: createdBy, Valid: true}
	}
	now = now.UTC().Truncate(time.Second)
	var id int64
	err = db.QueryRowContext(ctx,
		`INSERT INTO set_video_shares (video_id, token_hash, label, created_by, created_at, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?) RETURNING id`,
		videoID, HashVideoShareToken(token), labelVal, creator, now, now.Add(lifetime)).Scan(&id)
	if err != nil {
		return nil, "", fmt.Errorf("models: create share for video %d: %w", videoID, err)
	}
	s, err := scanVideoShare(db.QueryRowContext(ctx, videoShareSelect+` WHERE s.id = ?`, id))
	if err != nil {
		return nil, "", fmt.Errorf("models: reload share %d: %w", id, err)
	}
	return s, token, nil
}

// ListVideoShares returns the links to clip videoID, newest first. With
// includeInactive false, revoked and expired links are left out.
func ListVideoShares(ctx context.Context, db *sql.DB, videoID int64, includeInactive bool, now time.Time) ([]*VideoShare, error) {
	rows, err := db.QueryContext(ctx, videoShareSelect+` WHERE s.video_id = ? ORDER BY s.created_at DESC, s.id DESC`, videoID)
	if err != nil {
		return nil, fmt.Errorf("models: list shares for video %d: %w", videoID, err)
	}
	defer rows.Close()
	out := []*VideoShare{}
	for rows.Next() {
		s, err := scanVideoShare(rows)
		if err != nil {
			return nil, fmt.Errorf("models: scan share: %w", err)
		}
		if !includeInactive && s.Status(now) != VideoShareActive {
			continue
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// RevokeVideoShare revokes link shareID, which must belong to clip videoID
// (ErrNotFound otherwise). Revoking an already revoked link keeps the first
// revocation. It takes effect at once: the token stops resolving.
func RevokeVideoShare(ctx context.Context, db *sql.DB, shareID, videoID, revokedBy int64, now time.Time) (*VideoShare, error) {
	var by sql.NullInt64
	if revokedBy > 0 {
		by = sql.NullInt64{Int64: revokedBy, Valid: true}
	}
	res, err := db.ExecContext(ctx,
		`UPDATE set_video_shares SET revoked_at = COALESCE(revoked_at, ?), revoked_by = COALESCE(revoked_by, ?)
		  WHERE id = ? AND video_id = ?`,
		now.UTC().Truncate(time.Second), by, shareID, videoID)
	if err != nil {
		return nil, fmt.Errorf("models: revoke share %d: %w", shareID, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	s, err := scanVideoShare(db.QueryRowContext(ctx, videoShareSelect+` WHERE s.id = ?`, shareID))
	if err != nil {
		return nil, fmt.Errorf("models: reload share %d: %w", shareID, err)
	}
	return s, nil
}

// ResolveVideoShare looks up a presented token and returns the clip it
// opens. Unknown, malformed, expired and revoked tokens all return
// ErrNotFound after the same work (hash, one indexed lookup), so neither the
// result nor its timing tells them apart. The file is resolved only through
// the stored clip row, never from the token.
func ResolveVideoShare(ctx context.Context, db *sql.DB, token string, now time.Time) (*SharedVideo, error) {
	wellFormed := wellFormedShareToken(token)
	if len(token) > maxShareTokenLen {
		token = token[:maxShareTokenLen]
	}
	hash := HashVideoShareToken(token)

	var (
		sv        SharedVideo
		storedSum string
		revokedAt sql.NullTime
	)
	err := db.QueryRowContext(ctx,
		`SELECT s.id, s.video_id, s.token_hash, s.expires_at, s.revoked_at,
		        v.file_name, v.content_type, e.name, w.date
		   FROM set_video_shares s
		   JOIN workout_set_videos v ON v.id = s.video_id
		   JOIN workout_sets ws ON ws.id = v.set_id
		   JOIN workouts w ON w.id = ws.workout_id
		   JOIN exercises e ON e.id = ws.exercise_id
		  WHERE s.token_hash = ?`, hash).
		Scan(&sv.ShareID, &sv.VideoID, &storedSum, &sv.ExpiresAt, &revokedAt,
			&sv.FileName, &sv.ContentType, &sv.ExerciseName, &sv.Date)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("models: resolve share: %w", err)
	}
	match := subtle.ConstantTimeCompare([]byte(storedSum), []byte(hash)) == 1
	if !wellFormed || !match || revokedAt.Valid || !now.Before(sv.ExpiresAt) {
		return nil, ErrNotFound
	}
	sv.Date = normalizeDate(sv.Date)
	return &sv, nil
}

// RecordVideoShareAccess stamps last_accessed_at on a link and, when
// countView is set, adds one to its access_count.
func RecordVideoShareAccess(ctx context.Context, db *sql.DB, shareID int64, countView bool, now time.Time) error {
	inc := 0
	if countView {
		inc = 1
	}
	_, err := db.ExecContext(ctx,
		`UPDATE set_video_shares SET last_accessed_at = ?, access_count = access_count + ? WHERE id = ?`,
		now.UTC().Truncate(time.Second), inc, shareID)
	if err != nil {
		return fmt.Errorf("models: record share %d access: %w", shareID, err)
	}
	return nil
}
