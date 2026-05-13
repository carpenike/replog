package models

import (
	"database/sql"
	"errors"
	"testing"
	"time"
)

func TestCreateUser(t *testing.T) {
	db := testDB(t)

	t.Run("basic create", func(t *testing.T) {
		u, err := CreateUser(db, "admin", "", "password123", "admin@test.com", true, false, sql.NullInt64{})
		if err != nil {
			t.Fatalf("create user: %v", err)
		}
		if u.Username != "admin" {
			t.Errorf("username = %q, want admin", u.Username)
		}
		if !u.IsCoach {
			t.Error("is_coach should be true")
		}
		if !u.Email.Valid || u.Email.String != "admin@test.com" {
			t.Errorf("email = %v, want admin@test.com", u.Email)
		}
	})

	t.Run("duplicate username", func(t *testing.T) {
		_, err := CreateUser(db, "admin", "", "other", "", false, false, sql.NullInt64{})
		if err != ErrDuplicateUsername {
			t.Errorf("err = %v, want ErrDuplicateUsername", err)
		}
	})

	t.Run("case insensitive duplicate", func(t *testing.T) {
		_, err := CreateUser(db, "ADMIN", "", "other", "", false, false, sql.NullInt64{})
		if err != ErrDuplicateUsername {
			t.Errorf("err = %v, want ErrDuplicateUsername", err)
		}
	})

	t.Run("duplicate athlete link on create", func(t *testing.T) {
		a, _ := CreateAthlete(db, "Solo", "", "", "", "", "", "", sql.NullInt64{}, true)
		CreateUser(db, "first_link", "", "password123", "", false, false, sql.NullInt64{Int64: a.ID, Valid: true})
		_, err := CreateUser(db, "second_link", "", "password123", "", false, false, sql.NullInt64{Int64: a.ID, Valid: true})
		if err != ErrAthleteAlreadyLinked {
			t.Errorf("err = %v, want ErrAthleteAlreadyLinked", err)
		}
	})

	t.Run("passwordless create", func(t *testing.T) {
		u, err := CreateUser(db, "kiduser", "", "", "", false, false, sql.NullInt64{})
		if err != nil {
			t.Fatalf("create passwordless user: %v", err)
		}
		if u.HasPassword() {
			t.Error("expected passwordless user, but HasPassword() is true")
		}
		if u.Username != "kiduser" {
			t.Errorf("username = %q, want kiduser", u.Username)
		}
	})
}

func TestAuthenticate(t *testing.T) {
	db := testDB(t)

	CreateUser(db, "testuser", "", "correct-password", "", false, false, sql.NullInt64{})

	t.Run("valid credentials", func(t *testing.T) {
		u, err := Authenticate(db, "testuser", "correct-password")
		if err != nil {
			t.Fatalf("authenticate: %v", err)
		}
		if u.Username != "testuser" {
			t.Errorf("username = %q, want testuser", u.Username)
		}
	})

	t.Run("wrong password", func(t *testing.T) {
		_, err := Authenticate(db, "testuser", "wrong-password")
		if err != ErrNotFound {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})

	t.Run("non-existent user", func(t *testing.T) {
		_, err := Authenticate(db, "nobody", "anything")
		if err != ErrNotFound {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})

	t.Run("passwordless user", func(t *testing.T) {
		CreateUser(db, "kidonly", "", "", "", false, false, sql.NullInt64{})
		_, err := Authenticate(db, "kidonly", "anything")
		if err != ErrNoPassword {
			t.Errorf("err = %v, want ErrNoPassword", err)
		}
	})

	t.Run("constant-time on user-not-found (no enumeration)", func(t *testing.T) {
		// Authenticate must run a bcrypt compare even when the username is
		// unknown so an attacker cannot distinguish "no such user" from
		// "wrong password" by response time. Without the dummy compare,
		// unknown-user returns in microseconds while wrong-password takes
		// ~80ms — trivial to enumerate accounts.
		//
		// We assert the unknown-user path is at least roughly the same
		// order of magnitude as the wrong-password path. Generous bound
		// (5x) to keep this stable on noisy CI.
		const samples = 3

		var unknown, wrong time.Duration
		for i := 0; i < samples; i++ {
			t0 := time.Now()
			_, _ = Authenticate(db, "no-such-user", "anything")
			unknown += time.Since(t0)

			t0 = time.Now()
			_, _ = Authenticate(db, "testuser", "wrong-password")
			wrong += time.Since(t0)
		}
		unknown /= samples
		wrong /= samples

		// Sanity: bcrypt should keep wrong-password well above 1ms.
		if wrong < time.Millisecond {
			t.Skipf("bcrypt finished too fast (%v) — env is probably mocked, can't measure", wrong)
		}
		if unknown*5 < wrong {
			t.Errorf("unknown-user (%v) is more than 5x faster than wrong-password (%v) — timing oracle for user enumeration",
				unknown, wrong)
		}
	})
}

// --- Per-account login lockout (ADR 014) ---

func TestAuthenticate_LocksAfterThreshold(t *testing.T) {
	db := testDB(t)
	CreateUser(db, "victim", "", "correct", "", false, false, sql.NullInt64{})

	// LockoutThreshold-1 wrong attempts should NOT lock.
	for i := 0; i < LockoutThreshold-1; i++ {
		_, err := Authenticate(db, "victim", "nope")
		if err != ErrNotFound {
			t.Fatalf("attempt %d: err = %v, want ErrNotFound", i+1, err)
		}
	}
	// The right password still works at threshold-1.
	if _, err := Authenticate(db, "victim", "correct"); err != nil {
		t.Fatalf("right password at threshold-1: err = %v", err)
	}
	// Successful login resets the counter, so we should be able to fail
	// LockoutThreshold-1 more times without locking.
	for i := 0; i < LockoutThreshold-1; i++ {
		if _, err := Authenticate(db, "victim", "nope"); err != ErrNotFound {
			t.Fatalf("post-reset attempt %d: err = %v, want ErrNotFound", i+1, err)
		}
	}

	// The Nth (== threshold) wrong attempt trips the lock. The error from
	// that attempt is still ErrNotFound (the password WAS wrong), but the
	// next attempt — even with the right password — must return ErrLocked.
	if _, err := Authenticate(db, "victim", "nope"); err != ErrNotFound {
		t.Fatalf("threshold attempt: err = %v, want ErrNotFound", err)
	}

	_, err := Authenticate(db, "victim", "correct")
	if !errors.Is(err, ErrLocked) {
		t.Fatalf("post-threshold with correct password: err = %v, want ErrLocked", err)
	}
	var le *LockoutError
	if !errors.As(err, &le) {
		t.Fatalf("expected *LockoutError, got %T", err)
	}
	if le.Remaining <= 0 || le.Remaining > LockoutDuration {
		t.Errorf("Remaining = %v, want in (0, %v]", le.Remaining, LockoutDuration)
	}
	if le.RetryAfter() < 1 {
		t.Errorf("RetryAfter() = %d, want >= 1", le.RetryAfter())
	}
}

func TestAuthenticate_LockoutWindowSlides(t *testing.T) {
	db := testDB(t)
	CreateUser(db, "victim", "", "correct", "", false, false, sql.NullInt64{})

	// Trip the lock.
	for i := 0; i < LockoutThreshold; i++ {
		_, _ = Authenticate(db, "victim", "nope")
	}

	// Read locked_until twice with a wrong-password attempt in between
	// and confirm it slid forward (the second value must be strictly
	// later than the first).
	var firstUntil, secondUntil sql.NullTime
	if err := db.QueryRow(`SELECT locked_until FROM users WHERE username = 'victim'`).Scan(&firstUntil); err != nil {
		t.Fatalf("read first locked_until: %v", err)
	}
	time.Sleep(20 * time.Millisecond) // ensure clock moves
	_, _ = Authenticate(db, "victim", "still wrong")
	if err := db.QueryRow(`SELECT locked_until FROM users WHERE username = 'victim'`).Scan(&secondUntil); err != nil {
		t.Fatalf("read second locked_until: %v", err)
	}

	if !firstUntil.Valid || !secondUntil.Valid {
		t.Fatalf("expected both timestamps valid, got %v / %v", firstUntil, secondUntil)
	}
	if !secondUntil.Time.After(firstUntil.Time) {
		t.Errorf("locked_until did not slide: first=%v, second=%v", firstUntil.Time, secondUntil.Time)
	}
}

func TestAuthenticate_LockoutDoesNotApplyToUnknownUser(t *testing.T) {
	db := testDB(t)
	// No user named "ghost" exists. Hammering ErrNotFound on an unknown
	// username must NOT create / lock any per-account state — otherwise
	// an attacker who knows real usernames can DoS them by submitting
	// usernames that don't exist (cheap) and still tripping a lock.
	for i := 0; i < LockoutThreshold*2; i++ {
		_, err := Authenticate(db, "ghost", "anything")
		if err != ErrNotFound {
			t.Fatalf("attempt %d: err = %v, want ErrNotFound", i+1, err)
		}
	}
	// Sanity: the row count is unchanged.
	var n int
	_ = db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
	if n != 0 {
		t.Errorf("user count = %d, want 0", n)
	}
}

func TestAuthenticate_PassthroughWhenLockExpired(t *testing.T) {
	db := testDB(t)
	user, _ := CreateUser(db, "victim", "", "correct", "", false, false, sql.NullInt64{})

	// Manually expire a lockout to simulate "15 minutes have passed".
	past := time.Now().Add(-1 * time.Second)
	if _, err := db.Exec(
		`UPDATE users SET failed_login_count = ?, locked_until = ? WHERE id = ?`,
		LockoutThreshold, past, user.ID,
	); err != nil {
		t.Fatalf("manually expire lock: %v", err)
	}

	// A correct-password attempt must succeed and reset state.
	if _, err := Authenticate(db, "victim", "correct"); err != nil {
		t.Fatalf("post-expiry login: err = %v", err)
	}

	var count int
	var lockedUntil sql.NullTime
	_ = db.QueryRow(`SELECT failed_login_count, locked_until FROM users WHERE id = ?`, user.ID).
		Scan(&count, &lockedUntil)
	if count != 0 || lockedUntil.Valid {
		t.Errorf("post-success state: count=%d, locked_until=%v, want 0/NULL", count, lockedUntil)
	}
}

func TestUpdatePassword_ClearsLockout(t *testing.T) {
	db := testDB(t)
	user, _ := CreateUser(db, "victim", "", "old-password", "", false, false, sql.NullInt64{})

	// Trip the lock.
	for i := 0; i < LockoutThreshold; i++ {
		_, _ = Authenticate(db, "victim", "nope")
	}
	// Confirm it's locked.
	if _, err := Authenticate(db, "victim", "old-password"); !errors.Is(err, ErrLocked) {
		t.Fatalf("pre-reset: err = %v, want ErrLocked", err)
	}

	// Admin resets the password — that must also clear the lockout.
	if err := UpdatePassword(db, user.ID, "new-password"); err != nil {
		t.Fatalf("update password: %v", err)
	}

	// New password works immediately — no lockout in the way.
	if _, err := Authenticate(db, "victim", "new-password"); err != nil {
		t.Errorf("post-reset login: err = %v, want nil", err)
	}
}

func TestLockoutError_RetryAfter(t *testing.T) {
	cases := []struct {
		remaining time.Duration
		want      int
	}{
		{0, 1},                   // never < 1
		{-5 * time.Second, 1},    // expired but still surfaced as 1
		{500 * time.Millisecond, 1},
		{1 * time.Second, 1},
		{1500 * time.Millisecond, 2}, // round up
		{30 * time.Second, 30},
		{15 * time.Minute, 900},
	}
	for _, tc := range cases {
		got := (&LockoutError{Remaining: tc.remaining}).RetryAfter()
		if got != tc.want {
			t.Errorf("RetryAfter(%v) = %d, want %d", tc.remaining, got, tc.want)
		}
	}
}

func TestHasPassword(t *testing.T) {
	db := testDB(t)

	t.Run("user with password", func(t *testing.T) {
		u, _ := CreateUser(db, "withpw", "", "secret123", "", false, false, sql.NullInt64{})
		if !u.HasPassword() {
			t.Error("HasPassword() should be true for user with password")
		}
	})

	t.Run("user without password", func(t *testing.T) {
		u, _ := CreateUser(db, "nopw", "", "", "", false, false, sql.NullInt64{})
		if u.HasPassword() {
			t.Error("HasPassword() should be false for passwordless user")
		}
	})
}

func TestCountUsers(t *testing.T) {
	db := testDB(t)

	count, err := CountUsers(db)
	if err != nil {
		t.Fatalf("count users: %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}

	CreateUser(db, "u1", "", "pass", "", false, false, sql.NullInt64{})
	CreateUser(db, "u2", "", "pass", "", true, false, sql.NullInt64{})

	count, err = CountUsers(db)
	if err != nil {
		t.Fatalf("count users: %v", err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
}

func TestUpdatePassword(t *testing.T) {
	db := testDB(t)

	u, _ := CreateUser(db, "pwuser", "", "old-password", "", false, false, sql.NullInt64{})

	if err := UpdatePassword(db, u.ID, "new-password"); err != nil {
		t.Fatalf("update password: %v", err)
	}

	// Old password should fail.
	_, err := Authenticate(db, "pwuser", "old-password")
	if err != ErrNotFound {
		t.Errorf("old password should fail, got %v", err)
	}

	// New password should work.
	_, err = Authenticate(db, "pwuser", "new-password")
	if err != nil {
		t.Errorf("new password should work, got %v", err)
	}
}

func TestDeleteUser(t *testing.T) {
	db := testDB(t)

	u, _ := CreateUser(db, "delme", "", "pass", "", false, false, sql.NullInt64{})

	if err := DeleteUser(db, u.ID); err != nil {
		t.Fatalf("delete user: %v", err)
	}

	_, err := GetUserByID(db, u.ID)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestListUsers(t *testing.T) {
	db := testDB(t)

	a, _ := CreateAthlete(db, "Linked Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	CreateUser(db, "alice", "", "pass", "alice@test.com", true, false, sql.NullInt64{})
	CreateUser(db, "bob", "", "pass", "", false, false, sql.NullInt64{Int64: a.ID, Valid: true})

	users, err := ListUsers(db)
	if err != nil {
		t.Fatalf("list users: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("count = %d, want 2", len(users))
	}
	// Ordered by username.
	if users[0].Username != "alice" {
		t.Errorf("first user = %q, want alice", users[0].Username)
	}
	if users[1].Username != "bob" {
		t.Errorf("second user = %q, want bob", users[1].Username)
	}
	// Bob should have athlete name.
	if !users[1].AthleteName.Valid || users[1].AthleteName.String != "Linked Athlete" {
		t.Errorf("bob athlete name = %v, want Linked Athlete", users[1].AthleteName)
	}
}

func TestUpdateUser(t *testing.T) {
	db := testDB(t)

	u, _ := CreateUser(db, "original", "", "pass", "orig@test.com", false, false, sql.NullInt64{})

	t.Run("basic update", func(t *testing.T) {
		updated, err := UpdateUser(db, u.ID, "renamed", "", "new@test.com", sql.NullInt64{}, true, false)
		if err != nil {
			t.Fatalf("update user: %v", err)
		}
		if updated.Username != "renamed" {
			t.Errorf("username = %q, want renamed", updated.Username)
		}
		if !updated.IsCoach {
			t.Error("is_coach should be true")
		}
	})

	t.Run("duplicate username", func(t *testing.T) {
		CreateUser(db, "taken", "", "pass", "", false, false, sql.NullInt64{})
		_, err := UpdateUser(db, u.ID, "taken", "", "", sql.NullInt64{}, false, false)
		if err != ErrDuplicateUsername {
			t.Errorf("err = %v, want ErrDuplicateUsername", err)
		}
	})

	t.Run("link athlete", func(t *testing.T) {
		a, _ := CreateAthlete(db, "Kid", "", "", "", "", "", "", sql.NullInt64{}, true)
		updated, err := UpdateUser(db, u.ID, "renamed", "", "", sql.NullInt64{Int64: a.ID, Valid: true}, false, false)
		if err != nil {
			t.Fatalf("update user: %v", err)
		}
		if !updated.AthleteID.Valid || updated.AthleteID.Int64 != a.ID {
			t.Errorf("athlete_id = %v, want %d", updated.AthleteID, a.ID)
		}
	})

	t.Run("duplicate athlete link", func(t *testing.T) {
		// "renamed" user is linked to "Kid" from the previous subtest.
		// Try linking a new user to the same athlete.
		a, _ := GetUserByUsername(db, "renamed")
		other, _ := CreateUser(db, "other", "", "pass", "", false, false, sql.NullInt64{})
		_, err := UpdateUser(db, other.ID, "other", "", "", sql.NullInt64{Int64: a.AthleteID.Int64, Valid: true}, false, false)
		if err != ErrAthleteAlreadyLinked {
			t.Errorf("err = %v, want ErrAthleteAlreadyLinked", err)
		}
	})
}

func TestUpdateAvatarPath(t *testing.T) {
	db := testDB(t)

	u, _ := CreateUser(db, "avatartest", "", "pass", "", false, false, sql.NullInt64{})

	// Initially no avatar.
	if u.HasAvatar() {
		t.Error("new user should not have avatar")
	}
	if u.AvatarURL() != "" {
		t.Errorf("AvatarURL = %q, want empty", u.AvatarURL())
	}

	t.Run("set avatar", func(t *testing.T) {
		err := UpdateAvatarPath(db, u.ID, sql.NullString{String: "1_abc123.png", Valid: true})
		if err != nil {
			t.Fatalf("update avatar: %v", err)
		}

		updated, _ := GetUserByID(db, u.ID)
		if !updated.HasAvatar() {
			t.Error("expected HasAvatar() to be true")
		}
		if updated.AvatarURL() != "/avatars/1_abc123.png" {
			t.Errorf("AvatarURL = %q, want /avatars/1_abc123.png", updated.AvatarURL())
		}
	})

	t.Run("clear avatar", func(t *testing.T) {
		err := UpdateAvatarPath(db, u.ID, sql.NullString{})
		if err != nil {
			t.Fatalf("clear avatar: %v", err)
		}

		updated, _ := GetUserByID(db, u.ID)
		if updated.HasAvatar() {
			t.Error("expected HasAvatar() to be false after clear")
		}
	})

	t.Run("not found", func(t *testing.T) {
		err := UpdateAvatarPath(db, 99999, sql.NullString{String: "x.png", Valid: true})
		if err != ErrNotFound {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}
