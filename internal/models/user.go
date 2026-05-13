package models

import (
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// ErrNotFound is returned when a query finds no matching row.
var ErrNotFound = errors.New("not found")
var ErrInvalidInput = errors.New("invalid input")

// ErrDuplicateUsername is returned when a username already exists.
var ErrDuplicateUsername = errors.New("duplicate username")

// ErrAthleteAlreadyLinked is returned when an athlete is already linked to another user.
var ErrAthleteAlreadyLinked = errors.New("athlete already linked to another user")

// ErrNoPassword is returned when authenticating a user that has no password set.
var ErrNoPassword = errors.New("account has no password")

// User represents a login account in the system.
type User struct {
	ID           int64
	Username     string
	Name         sql.NullString
	Email        sql.NullString
	PasswordHash string
	AthleteID    sql.NullInt64
	IsCoach      bool
	IsAdmin      bool
	AvatarPath   sql.NullString
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// HasAvatar reports whether the user has an avatar image set.
func (u *User) HasAvatar() bool {
	return u.AvatarPath.Valid && u.AvatarPath.String != ""
}

// AvatarURL returns the URL path for the user's avatar image.
// Returns empty string if no avatar is set.
func (u *User) AvatarURL() string {
	if !u.HasAvatar() {
		return ""
	}
	return "/avatars/" + u.AvatarPath.String
}

// HasPassword reports whether the user has a password set.
// Passwordless users authenticate via magic links or passkeys.
func (u *User) HasPassword() bool {
	return u.PasswordHash != ""
}

// UserWithAthlete extends User with the linked athlete's name.
type UserWithAthlete struct {
	User
	AthleteName sql.NullString
}

// HashPassword generates a bcrypt hash of the given plaintext password.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("models: hash password: %w", err)
	}
	return string(hash), nil
}

// CheckPassword compares a plaintext password against a bcrypt hash.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// CreateUser inserts a new user. Returns ErrDuplicateUsername if the username
// is already taken. When athleteID is valid the user is linked atomically.
// If password is empty the user is created without a password (passwordless).
func CreateUser(db *sql.DB, username, name, password, email string, isCoach bool, isAdmin bool, athleteID sql.NullInt64) (*User, error) {
	var hashVal sql.NullString
	if password != "" {
		hash, err := HashPassword(password)
		if err != nil {
			return nil, err
		}
		hashVal = sql.NullString{String: hash, Valid: true}
	}

	var emailVal sql.NullString
	if email != "" {
		emailVal = sql.NullString{String: email, Valid: true}
	}

	var nameVal sql.NullString
	if name != "" {
		nameVal = sql.NullString{String: name, Valid: true}
	}

	coachInt := 0
	if isCoach {
		coachInt = 1
	}

	adminInt := 0
	if isAdmin {
		adminInt = 1
	}

	var id int64
	err := db.QueryRow(
		`INSERT INTO users (username, name, email, password_hash, is_coach, is_admin, athlete_id) VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		username, nameVal, emailVal, hashVal, coachInt, adminInt, athleteID,
	).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			if errContains(err, "athlete_id") {
				return nil, ErrAthleteAlreadyLinked
			}
			return nil, ErrDuplicateUsername
		}
		return nil, fmt.Errorf("models: create user %q: %w", username, err)
	}

	return GetUserByID(db, id)
}

// GetUserByID retrieves a user by primary key.
func GetUserByID(db *sql.DB, id int64) (*User, error) {
	u := &User{}
	err := db.QueryRow(
		`SELECT id, username, name, email, COALESCE(password_hash, ''), athlete_id, is_coach, is_admin, avatar_path, created_at, updated_at
		 FROM users WHERE id = ?`, id,
	).Scan(&u.ID, &u.Username, &u.Name, &u.Email, &u.PasswordHash, &u.AthleteID, &u.IsCoach, &u.IsAdmin, &u.AvatarPath, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("models: get user %d: %w", id, err)
	}
	return u, nil
}

// GetUserByAthleteID returns the user linked to the given athlete (if any).
// Returns ErrNotFound when no user is linked — that's expected for athletes
// who don't have their own login (e.g. young kids whose parent logs sets on
// their behalf). Callers in the notification path should treat ErrNotFound
// as "skip the notify"; it is not an error condition.
//
// users.athlete_id has a partial unique index (`WHERE athlete_id IS NOT NULL`),
// so at most one user per athlete.
func GetUserByAthleteID(db *sql.DB, athleteID int64) (*User, error) {
	u := &User{}
	err := db.QueryRow(
		`SELECT id, username, name, email, COALESCE(password_hash, ''), athlete_id, is_coach, is_admin, avatar_path, created_at, updated_at
		 FROM users WHERE athlete_id = ?`, athleteID,
	).Scan(&u.ID, &u.Username, &u.Name, &u.Email, &u.PasswordHash, &u.AthleteID, &u.IsCoach, &u.IsAdmin, &u.AvatarPath, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("models: get user by athlete %d: %w", athleteID, err)
	}
	return u, nil
}

// GetUserByUsername retrieves a user by username (case-insensitive).
func GetUserByUsername(db *sql.DB, username string) (*User, error) {
	u := &User{}
	err := db.QueryRow(
		`SELECT id, username, name, email, COALESCE(password_hash, ''), athlete_id, is_coach, is_admin, avatar_path, created_at, updated_at
		 FROM users WHERE username = ?`, username,
	).Scan(&u.ID, &u.Username, &u.Name, &u.Email, &u.PasswordHash, &u.AthleteID, &u.IsCoach, &u.IsAdmin, &u.AvatarPath, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("models: get user by username %q: %w", username, err)
	}
	return u, nil
}

// ErrLocked is returned by Authenticate when the account is currently
// locked due to too many recent wrong-password attempts. Use
// errors.As to extract the *LockoutError and call its RetryAfter()
// for the HTTP Retry-After hint.
var ErrLocked = &LockoutError{}

// LockoutError carries the remaining lockout duration so callers can
// surface it in the response (e.g. HTTP Retry-After). Implements error
// and unwraps to ErrLocked so errors.Is(err, ErrLocked) works.
type LockoutError struct {
	Remaining time.Duration
}

func (e *LockoutError) Error() string { return "account temporarily locked" }

// Is reports whether target is the sentinel ErrLocked. This lets
// errors.Is(err, ErrLocked) match any *LockoutError instance regardless
// of the embedded Remaining duration.
func (e *LockoutError) Is(target error) bool {
	_, ok := target.(*LockoutError)
	return ok
}

// RetryAfter returns the lockout window remaining, rounded up to the
// next whole second so HTTP Retry-After is never 0 while still locked.
func (e *LockoutError) RetryAfter() int {
	if e.Remaining <= 0 {
		return 1
	}
	secs := int(e.Remaining / time.Second)
	if e.Remaining%time.Second != 0 {
		secs++
	}
	if secs < 1 {
		secs = 1
	}
	return secs
}

// LockoutThreshold is the number of consecutive wrong-password attempts
// against a known account that triggers a lockout. See ADR 014.
const LockoutThreshold = 5

// LockoutDuration is how long an account stays locked after exceeding
// LockoutThreshold. The window slides — every additional wrong attempt
// while locked extends locked_until by another LockoutDuration. See ADR 014.
const LockoutDuration = 15 * time.Minute

// dummyBcryptHash is a bcrypt hash of an internal value that no real account
// will ever hold. Generated lazily on first use so the cost stays in sync
// with bcrypt.DefaultCost without hard-coding a literal hash. We compare
// against it on the user-not-found and passwordless-account paths so that
// Authenticate's response time does not reveal whether a username exists
// (defense against user enumeration via timing).
var (
	dummyBcryptHashOnce sync.Once
	dummyBcryptHash     []byte
)

func dummyHash() []byte {
	dummyBcryptHashOnce.Do(func() {
		// Errors here mean bcrypt itself is broken; let the panic surface.
		h, err := bcrypt.GenerateFromPassword([]byte("replog-dummy-credential"), bcrypt.DefaultCost)
		if err != nil {
			panic(fmt.Sprintf("models: generate dummy bcrypt hash: %v", err))
		}
		dummyBcryptHash = h
	})
	return dummyBcryptHash
}

// Authenticate verifies a username/password combination and returns the user
// if valid.
//
// Possible errors:
//   - ErrNotFound: unknown username, or known username with wrong password
//   - ErrNoPassword: account exists but has no password (passwordless-only)
//   - ErrLocked: account is temporarily locked due to too many recent
//     wrong-password attempts (see ADR 014). Use LockoutRemaining for the
//     Retry-After hint.
//
// Timing defenses: on user-not-found and passwordless-account paths we run
// a bcrypt compare against a fixed dummy hash so that the response time
// does not reveal whether the username exists or has a password.
//
// Lockout defenses: after LockoutThreshold consecutive wrong-password
// attempts against a known account, the account is locked for
// LockoutDuration. The window slides — every additional wrong attempt
// while locked extends locked_until. Successful login resets the counter.
// Unknown-username attempts do NOT consume any per-account budget so
// attackers cannot DoS arbitrary accounts knowing only the username.
func Authenticate(db *sql.DB, username, password string) (*User, error) {
	u, err := GetUserByUsername(db, username)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			// Burn the same time a real bcrypt compare would. Result discarded.
			_ = bcrypt.CompareHashAndPassword(dummyHash(), []byte(password))
		}
		return nil, err
	}
	if !u.HasPassword() {
		_ = bcrypt.CompareHashAndPassword(dummyHash(), []byte(password))
		return nil, ErrNoPassword
	}

	// Account-level lockout check. If currently locked, refuse without
	// running bcrypt — that keeps the locked response cheap and removes
	// any "still locked, but right password" timing oracle.
	if remaining, err := checkAndExtendLockout(db, u.ID); err != nil {
		return nil, err
	} else if remaining > 0 {
		return nil, &LockoutError{Remaining: remaining}
	}

	if !CheckPassword(u.PasswordHash, password) {
		// Increment the failure counter; trip the lock if we just hit
		// the threshold. Best-effort: if the UPDATE fails we still
		// return the auth failure (better to refuse login than to leak
		// success on an unrelated DB error).
		_ = recordFailedLogin(db, u.ID)
		return nil, ErrNotFound
	}

	// Success — clear any leftover failure state.
	_ = clearFailedLogin(db, u.ID)
	return u, nil
}

// checkAndExtendLockout returns the remaining lockout duration when the
// account is currently locked; in that case it ALSO extends locked_until
// by LockoutDuration so the attacker cannot wait out the window in the
// background by spreading attempts across accounts. Returns 0 when not
// locked.
func checkAndExtendLockout(db *sql.DB, userID int64) (time.Duration, error) {
	var lockedUntil sql.NullTime
	err := db.QueryRow(`SELECT locked_until FROM users WHERE id = ?`, userID).Scan(&lockedUntil)
	if err != nil {
		return 0, fmt.Errorf("models: read lockout state for user %d: %w", userID, err)
	}
	if !lockedUntil.Valid || !lockedUntil.Time.After(time.Now()) {
		return 0, nil
	}
	// Slide the window: another attempt during a lockout pushes the
	// unlock further out. We do NOT increment the counter here; the
	// counter exists to trip the lock, not to track attempts during.
	newUntil := time.Now().Add(LockoutDuration)
	_, _ = db.Exec(`UPDATE users SET locked_until = ? WHERE id = ?`, newUntil, userID)
	return LockoutDuration, nil
}

// recordFailedLogin increments failed_login_count and, if it crosses
// LockoutThreshold, sets locked_until = now + LockoutDuration.
func recordFailedLogin(db *sql.DB, userID int64) error {
	// Single statement — atomic under SQLite's single-writer model.
	// CASE expression decides whether to set locked_until in the same
	// UPDATE based on the post-increment count.
	_, err := db.Exec(`
		UPDATE users
		   SET failed_login_count = failed_login_count + 1,
		       locked_until = CASE
		           WHEN failed_login_count + 1 >= ? THEN ?
		           ELSE locked_until
		       END
		 WHERE id = ?`,
		LockoutThreshold,
		time.Now().Add(LockoutDuration),
		userID,
	)
	return err
}

// clearFailedLogin resets failure counter and lockout on successful login.
func clearFailedLogin(db *sql.DB, userID int64) error {
	_, err := db.Exec(
		`UPDATE users SET failed_login_count = 0, locked_until = NULL WHERE id = ?`,
		userID,
	)
	return err
}

// CountUsers returns the total number of users in the database.
func CountUsers(db *sql.DB) (int, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("models: count users: %w", err)
	}
	return count, nil
}

// ListUsers returns all users with linked athlete names, ordered by username.
func ListUsers(db *sql.DB) ([]*UserWithAthlete, error) {
	rows, err := db.Query(`
		SELECT u.id, u.username, u.name, u.email, COALESCE(u.password_hash, ''), u.athlete_id, u.is_coach, u.is_admin, u.avatar_path, u.created_at, u.updated_at,
		       a.name
		FROM users u
		LEFT JOIN athletes a ON u.athlete_id = a.id
		ORDER BY u.username COLLATE NOCASE
		LIMIT 100
	`)
	if err != nil {
		return nil, fmt.Errorf("models: list users: %w", err)
	}
	defer rows.Close()

	var users []*UserWithAthlete
	for rows.Next() {
		u := &UserWithAthlete{}
		if err := rows.Scan(&u.ID, &u.Username, &u.Name, &u.Email, &u.PasswordHash, &u.AthleteID, &u.IsCoach, &u.IsAdmin, &u.AvatarPath, &u.CreatedAt, &u.UpdatedAt, &u.AthleteName); err != nil {
			return nil, fmt.Errorf("models: list users scan: %w", err)
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("models: iterate users: %w", err)
	}
	return users, nil
}

// UpdateUser updates a user's profile fields (not password).
// Returns ErrDuplicateUsername if the new username conflicts.
func UpdateUser(db *sql.DB, id int64, username, name, email string, athleteID sql.NullInt64, isCoach bool, isAdmin bool) (*User, error) {
	var emailVal sql.NullString
	if email != "" {
		emailVal = sql.NullString{String: email, Valid: true}
	}

	var nameVal sql.NullString
	if name != "" {
		nameVal = sql.NullString{String: name, Valid: true}
	}

	coachInt := 0
	if isCoach {
		coachInt = 1
	}

	adminInt := 0
	if isAdmin {
		adminInt = 1
	}

	result, err := db.Exec(
		`UPDATE users SET username = ?, name = ?, email = ?, athlete_id = ?, is_coach = ?, is_admin = ? WHERE id = ?`,
		username, nameVal, emailVal, athleteID, coachInt, adminInt, id,
	)
	if err != nil {
		if isUniqueViolation(err) {
			if errContains(err, "athlete_id") {
				return nil, ErrAthleteAlreadyLinked
			}
			return nil, ErrDuplicateUsername
		}
		return nil, fmt.Errorf("models: update user %d: %w", id, err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("models: update user %d rows affected: %w", id, err)
	}
	if n == 0 {
		return nil, ErrNotFound
	}

	return GetUserByID(db, id)
}

// UpdatePassword changes a user's password hash. Also clears any lockout
// state (failed_login_count + locked_until) — a successful password change
// is the canonical recovery path from a forgotten-password lockout.
func UpdatePassword(db *sql.DB, id int64, newPassword string) error {
	hash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	result, err := db.Exec(
		`UPDATE users
		    SET password_hash = ?,
		        failed_login_count = 0,
		        locked_until = NULL
		  WHERE id = ?`,
		hash, id,
	)
	if err != nil {
		return fmt.Errorf("models: update password for user %d: %w", id, err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateAvatarPath sets the avatar_path for a user.
func UpdateAvatarPath(db *sql.DB, id int64, avatarPath sql.NullString) error {
	result, err := db.Exec(`UPDATE users SET avatar_path = ? WHERE id = ?`, avatarPath, id)
	if err != nil {
		return fmt.Errorf("models: update avatar for user %d: %w", id, err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteUser removes a user by ID.
func DeleteUser(db *sql.DB, id int64) error {
	result, err := db.Exec(`DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("models: delete user %d: %w", id, err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
