package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
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
	// MCPEnabled gates whether the bearer middleware accepts JWTs that
	// resolve to this user. Default 0 (rejected with 403 mcp-not-enabled)
	// per HOF-004. The webui's scs session-cookie auth IGNORES this flag
	// — it only affects the /api-mcp/* path.
	MCPEnabled bool
	// PocketIDSub is the stable PocketID subject (`sub` claim), bound on
	// first OIDC login (ADR 019 Phase 1). Authoritative identity key for
	// returning webui logins. NULL for accounts not yet bound (e.g. kids
	// who still log in via magic link). Only GetUserByPocketIDSub populates
	// this field; other getters leave it zero-valued.
	PocketIDSub sql.NullString
	AvatarPath  sql.NullString
	CreatedAt   time.Time
	UpdatedAt   time.Time
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
func CreateUser(ctx context.Context, db *sql.DB, username, name, password, email string, isCoach bool, isAdmin bool, athleteID sql.NullInt64) (*User, error) {
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
	err := db.QueryRowContext(ctx,
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

	return GetUserByID(ctx, db, id)
}

// GetUserByID retrieves a user by primary key.
func GetUserByID(ctx context.Context, db *sql.DB, id int64) (*User, error) {
	u := &User{}
	err := db.QueryRowContext(ctx,
		`SELECT id, username, name, email, COALESCE(password_hash, ''), athlete_id, is_coach, is_admin, mcp_enabled, avatar_path, created_at, updated_at
		 FROM users WHERE id = ?`, id,
	).Scan(&u.ID, &u.Username, &u.Name, &u.Email, &u.PasswordHash, &u.AthleteID, &u.IsCoach, &u.IsAdmin, &u.MCPEnabled, &u.AvatarPath, &u.CreatedAt, &u.UpdatedAt)
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
func GetUserByAthleteID(ctx context.Context, db *sql.DB, athleteID int64) (*User, error) {
	u := &User{}
	err := db.QueryRowContext(ctx,
		`SELECT id, username, name, email, COALESCE(password_hash, ''), athlete_id, is_coach, is_admin, mcp_enabled, avatar_path, created_at, updated_at
		 FROM users WHERE athlete_id = ?`, athleteID,
	).Scan(&u.ID, &u.Username, &u.Name, &u.Email, &u.PasswordHash, &u.AthleteID, &u.IsCoach, &u.IsAdmin, &u.MCPEnabled, &u.AvatarPath, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("models: get user by athlete %d: %w", athleteID, err)
	}
	return u, nil
}

// GetUserByUsername retrieves a user by username (case-insensitive).
func GetUserByUsername(ctx context.Context, db *sql.DB, username string) (*User, error) {
	u := &User{}
	err := db.QueryRowContext(ctx,
		`SELECT id, username, name, email, COALESCE(password_hash, ''), athlete_id, is_coach, is_admin, mcp_enabled, avatar_path, created_at, updated_at
		 FROM users WHERE username = ?`, username,
	).Scan(&u.ID, &u.Username, &u.Name, &u.Email, &u.PasswordHash, &u.AthleteID, &u.IsCoach, &u.IsAdmin, &u.MCPEnabled, &u.AvatarPath, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("models: get user by username %q: %w", username, err)
	}
	return u, nil
}

// GetUserByEmail retrieves a user by email address (case-insensitive).
//
// Used by the MCP bearer middleware (HOF-004) to resolve a JWT `email`
// claim to a *User. `users.email` is `UNIQUE COLLATE NOCASE` and may be
// NULL (kids typically have no email); UNIQUE permits multiple NULLs in
// SQLite, so the caller MUST refuse an empty/absent claim BEFORE calling
// this function — otherwise an empty lookup could resolve to an unintended
// NULL row. This function is intentionally a thin SELECT and does NOT
// guard against empty input itself; the bearer middleware enforces the
// rule at the request boundary.
func GetUserByEmail(ctx context.Context, db *sql.DB, email string) (*User, error) {
	u := &User{}
	err := db.QueryRowContext(ctx,
		`SELECT id, username, name, email, COALESCE(password_hash, ''), athlete_id, is_coach, is_admin, mcp_enabled, avatar_path, created_at, updated_at
		 FROM users WHERE email = ?`, email,
	).Scan(&u.ID, &u.Username, &u.Name, &u.Email, &u.PasswordHash, &u.AthleteID, &u.IsCoach, &u.IsAdmin, &u.MCPEnabled, &u.AvatarPath, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("models: get user by email %q: %w", email, err)
	}
	return u, nil
}

// GetUserByPocketIDSub retrieves a user by their bound PocketID subject
// (ADR 019 Phase 1). This is the steady-state lookup for returning webui
// logins: once a user's `sub` is bound, every subsequent OIDC login resolves
// here. Returns ErrNotFound when no row carries the sub.
//
// The caller MUST refuse an empty `sub` before calling this — `pocketid_sub`
// is UNIQUE but permits multiple NULLs in SQLite, so an empty lookup must
// never be allowed to resolve to an unbound row. UpsertUserFromOIDC enforces
// that guard; this function is a thin SELECT and does not.
func GetUserByPocketIDSub(ctx context.Context, db *sql.DB, sub string) (*User, error) {
	u := &User{}
	err := db.QueryRowContext(ctx,
		`SELECT id, username, name, email, COALESCE(password_hash, ''), athlete_id, is_coach, is_admin, mcp_enabled, COALESCE(pocketid_sub, ''), avatar_path, created_at, updated_at
		 FROM users WHERE pocketid_sub = ?`, sub,
	).Scan(&u.ID, &u.Username, &u.Name, &u.Email, &u.PasswordHash, &u.AthleteID, &u.IsCoach, &u.IsAdmin, &u.MCPEnabled, &u.PocketIDSub, &u.AvatarPath, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("models: get user by pocketid_sub: %w", err)
	}
	return u, nil
}

// bindPocketIDSub binds a PocketID subject to an existing user row. Used both
// for the one-time verified-email cutover of a pre-existing account and right
// after a JIT-create. Returns ErrDuplicateUsername-style mapping is not needed
// here — a sub collision means a concurrent bind, surfaced as a wrapped error.
func bindPocketIDSub(ctx context.Context, db *sql.DB, userID int64, sub string) error {
	result, err := db.ExecContext(ctx, `UPDATE users SET pocketid_sub = ? WHERE id = ?`, sub, userID)
	if err != nil {
		return fmt.Errorf("models: bind pocketid_sub to user %d: %w", userID, err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// UpsertUserFromOIDC resolves a PocketID OIDC login to a local user, creating
// or binding as needed (ADR 019 Phase 1 — HOF-012). The linking rule, in order:
//
//  1. Match by `sub` — the steady-state path for a returning, already-bound user.
//  2. Verified-email fallback — only when emailVerified is true, match an
//     existing account by email and bind its `sub`. This is the one-time cutover
//     for the pre-existing (single) account. Requiring email_verified closes the
//     account-takeover window where an attacker registers an unverified address
//     matching a local user's email to hijack the bind.
//  3. JIT-create — a new passwordless user with a derived, unique username, then
//     bind the `sub`. Email is stored on the new row only when verified, so an
//     unverified address never lands as a future match target.
//
// An empty `sub` is rejected (ErrInvalidInput) — defense in depth; the OIDC
// callback handler also rejects it at the request boundary, mirroring the
// bearer middleware's empty-claim 401.
func UpsertUserFromOIDC(ctx context.Context, db *sql.DB, sub, email, name string, emailVerified bool) (*User, error) {
	if sub == "" {
		return nil, ErrInvalidInput
	}

	// 1. Steady state: already bound.
	u, err := GetUserByPocketIDSub(ctx, db, sub)
	if err == nil {
		return u, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	// 2. Verified-email cutover: bind sub to the existing account.
	if email != "" && emailVerified {
		existing, err := GetUserByEmail(ctx, db, email)
		switch {
		case err == nil:
			if err := bindPocketIDSub(ctx, db, existing.ID, sub); err != nil {
				return nil, err
			}
			return GetUserByPocketIDSub(ctx, db, sub)
		case errors.Is(err, ErrNotFound):
			// fall through to JIT-create
		default:
			return nil, err
		}
	}

	// 3. JIT-create a passwordless user with a derived unique username.
	username, err := deriveUniqueUsername(ctx, db, email, sub)
	if err != nil {
		return nil, err
	}
	createEmail := ""
	if emailVerified {
		createEmail = email
	}
	created, err := CreateUser(ctx, db, username, name, "", createEmail, false, false, sql.NullInt64{})
	if err != nil {
		return nil, err
	}
	if err := bindPocketIDSub(ctx, db, created.ID, sub); err != nil {
		return nil, err
	}
	return GetUserByPocketIDSub(ctx, db, sub)
}

// deriveUniqueUsername builds a username for a JIT-created OIDC user. It seeds
// from the email local-part (falling back to a sub-derived stub), then resolves
// collisions by appending an incrementing suffix. usernames are UNIQUE COLLATE
// NOCASE so the lookup is case-insensitive.
func deriveUniqueUsername(ctx context.Context, db *sql.DB, email, sub string) (string, error) {
	base := "user"
	if at := strings.IndexByte(email, '@'); at > 0 {
		base = email[:at]
	} else if len(sub) >= 8 {
		base = "pocketid_" + sub[:8]
	} else if sub != "" {
		base = "pocketid_" + sub
	}

	candidate := base
	for i := 2; i < 1000; i++ {
		_, err := GetUserByUsername(ctx, db, candidate)
		if errors.Is(err, ErrNotFound) {
			return candidate, nil
		}
		if err != nil {
			return "", err
		}
		candidate = fmt.Sprintf("%s_%d", base, i)
	}
	return "", fmt.Errorf("models: could not derive a unique username from %q", base)
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
func Authenticate(ctx context.Context, db *sql.DB, username, password string) (*User, error) {
	u, err := GetUserByUsername(ctx, db, username)
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
	if remaining, err := checkAndExtendLockout(ctx, db, u.ID); err != nil {
		return nil, err
	} else if remaining > 0 {
		return nil, &LockoutError{Remaining: remaining}
	}

	if !CheckPassword(u.PasswordHash, password) {
		// Increment the failure counter; trip the lock if we just hit
		// the threshold. Best-effort: if the UPDATE fails we still
		// return the auth failure (better to refuse login than to leak
		// success on an unrelated DB error).
		_ = recordFailedLogin(ctx, db, u.ID)
		return nil, ErrNotFound
	}

	// Success — clear any leftover failure state.
	_ = clearFailedLogin(ctx, db, u.ID)
	return u, nil
}

// checkAndExtendLockout returns the remaining lockout duration when the
// account is currently locked; in that case it ALSO extends locked_until
// by LockoutDuration so the attacker cannot wait out the window in the
// background by spreading attempts across accounts. Returns 0 when not
// locked.
func checkAndExtendLockout(ctx context.Context, db *sql.DB, userID int64) (time.Duration, error) {
	var lockedUntil sql.NullTime
	err := db.QueryRowContext(ctx, `SELECT locked_until FROM users WHERE id = ?`, userID).Scan(&lockedUntil)
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
	_, _ = db.ExecContext(ctx, `UPDATE users SET locked_until = ? WHERE id = ?`, newUntil, userID)
	return LockoutDuration, nil
}

// recordFailedLogin increments failed_login_count and, if it crosses
// LockoutThreshold, sets locked_until = now + LockoutDuration.
func recordFailedLogin(ctx context.Context, db *sql.DB, userID int64) error {
	// Single statement — atomic under SQLite's single-writer model.
	// CASE expression decides whether to set locked_until in the same
	// UPDATE based on the post-increment count.
	_, err := db.ExecContext(ctx, `
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
func clearFailedLogin(ctx context.Context, db *sql.DB, userID int64) error {
	_, err := db.ExecContext(ctx,
		`UPDATE users SET failed_login_count = 0, locked_until = NULL WHERE id = ?`,
		userID,
	)
	return err
}

// CountUsers returns the total number of users in the database.
func CountUsers(ctx context.Context, db *sql.DB) (int, error) {
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("models: count users: %w", err)
	}
	return count, nil
}

// ListUsers returns all users with linked athlete names, ordered by username.
func ListUsers(ctx context.Context, db *sql.DB) ([]*UserWithAthlete, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT u.id, u.username, u.name, u.email, COALESCE(u.password_hash, ''), u.athlete_id, u.is_coach, u.is_admin, u.mcp_enabled, u.avatar_path, u.created_at, u.updated_at,
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
		if err := rows.Scan(&u.ID, &u.Username, &u.Name, &u.Email, &u.PasswordHash, &u.AthleteID, &u.IsCoach, &u.IsAdmin, &u.MCPEnabled, &u.AvatarPath, &u.CreatedAt, &u.UpdatedAt, &u.AthleteName); err != nil {
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
func UpdateUser(ctx context.Context, db *sql.DB, id int64, username, name, email string, athleteID sql.NullInt64, isCoach bool, isAdmin bool) (*User, error) {
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

	result, err := db.ExecContext(ctx,
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

	return GetUserByID(ctx, db, id)
}

// UpdatePassword changes a user's password hash. Also clears any lockout
// state (failed_login_count + locked_until) — a successful password change
// is the canonical recovery path from a forgotten-password lockout.
func UpdatePassword(ctx context.Context, db *sql.DB, id int64, newPassword string) error {
	hash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	result, err := db.ExecContext(ctx,
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
func UpdateAvatarPath(ctx context.Context, db *sql.DB, id int64, avatarPath sql.NullString) error {
	result, err := db.ExecContext(ctx, `UPDATE users SET avatar_path = ? WHERE id = ?`, avatarPath, id)
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
func DeleteUser(ctx context.Context, db *sql.DB, id int64) error {
	result, err := db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("models: delete user %d: %w", id, err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetUserMCPEnabled flips the MCP-access gate on a user (HOF-004).
//
// Toggled by an admin via PUT /api/users/{userID}/mcp; consumed by the
// bearer middleware on every /api-mcp/* request. Returns ErrNotFound if
// no row exists for the given id.
func SetUserMCPEnabled(ctx context.Context, db *sql.DB, id int64, enabled bool) error {
	flag := 0
	if enabled {
		flag = 1
	}
	result, err := db.ExecContext(ctx, `UPDATE users SET mcp_enabled = ? WHERE id = ?`, flag, id)
	if err != nil {
		return fmt.Errorf("models: set mcp_enabled for user %d: %w", id, err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
