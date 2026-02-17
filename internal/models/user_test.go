package models

import (
	"database/sql"
	"testing"
)

func TestCreateUser(t *testing.T) {
	db := testDB(t)

	t.Run("basic create", func(t *testing.T) {
		u, err := CreateUser(db, "admin", "password123", "admin@test.com", true, false, sql.NullInt64{})
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
		_, err := CreateUser(db, "admin", "other", "", false, false, sql.NullInt64{})
		if err != ErrDuplicateUsername {
			t.Errorf("err = %v, want ErrDuplicateUsername", err)
		}
	})

	t.Run("case insensitive duplicate", func(t *testing.T) {
		_, err := CreateUser(db, "ADMIN", "other", "", false, false, sql.NullInt64{})
		if err != ErrDuplicateUsername {
			t.Errorf("err = %v, want ErrDuplicateUsername", err)
		}
	})

	t.Run("duplicate athlete link on create", func(t *testing.T) {
		a, _ := CreateAthlete(db, "Solo", "", "", sql.NullInt64{})
		CreateUser(db, "first_link", "password123", "", false, false, sql.NullInt64{Int64: a.ID, Valid: true})
		_, err := CreateUser(db, "second_link", "password123", "", false, false, sql.NullInt64{Int64: a.ID, Valid: true})
		if err != ErrAthleteAlreadyLinked {
			t.Errorf("err = %v, want ErrAthleteAlreadyLinked", err)
		}
	})

	t.Run("passwordless create", func(t *testing.T) {
		u, err := CreateUser(db, "kiduser", "", "", false, false, sql.NullInt64{})
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

	CreateUser(db, "testuser", "correct-password", "", false, false, sql.NullInt64{})

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
		CreateUser(db, "kidonly", "", "", false, false, sql.NullInt64{})
		_, err := Authenticate(db, "kidonly", "anything")
		if err != ErrNoPassword {
			t.Errorf("err = %v, want ErrNoPassword", err)
		}
	})
}

func TestHasPassword(t *testing.T) {
	db := testDB(t)

	t.Run("user with password", func(t *testing.T) {
		u, _ := CreateUser(db, "withpw", "secret123", "", false, false, sql.NullInt64{})
		if !u.HasPassword() {
			t.Error("HasPassword() should be true for user with password")
		}
	})

	t.Run("user without password", func(t *testing.T) {
		u, _ := CreateUser(db, "nopw", "", "", false, false, sql.NullInt64{})
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

	CreateUser(db, "u1", "pass", "", false, false, sql.NullInt64{})
	CreateUser(db, "u2", "pass", "", true, false, sql.NullInt64{})

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

	u, _ := CreateUser(db, "pwuser", "old-password", "", false, false, sql.NullInt64{})

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

	u, _ := CreateUser(db, "delme", "pass", "", false, false, sql.NullInt64{})

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

	a, _ := CreateAthlete(db, "Linked Athlete", "", "", sql.NullInt64{})
	CreateUser(db, "alice", "pass", "alice@test.com", true, false, sql.NullInt64{})
	CreateUser(db, "bob", "pass", "", false, false, sql.NullInt64{Int64: a.ID, Valid: true})

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

	u, _ := CreateUser(db, "original", "pass", "orig@test.com", false, false, sql.NullInt64{})

	t.Run("basic update", func(t *testing.T) {
		updated, err := UpdateUser(db, u.ID, "renamed", "new@test.com", sql.NullInt64{}, true, false)
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
		CreateUser(db, "taken", "pass", "", false, false, sql.NullInt64{})
		_, err := UpdateUser(db, u.ID, "taken", "", sql.NullInt64{}, false, false)
		if err != ErrDuplicateUsername {
			t.Errorf("err = %v, want ErrDuplicateUsername", err)
		}
	})

	t.Run("link athlete", func(t *testing.T) {
		a, _ := CreateAthlete(db, "Kid", "", "", sql.NullInt64{})
		updated, err := UpdateUser(db, u.ID, "renamed", "", sql.NullInt64{Int64: a.ID, Valid: true}, false, false)
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
		other, _ := CreateUser(db, "other", "pass", "", false, false, sql.NullInt64{})
		_, err := UpdateUser(db, other.ID, "other", "", sql.NullInt64{Int64: a.AthleteID.Int64, Valid: true}, false, false)
		if err != ErrAthleteAlreadyLinked {
			t.Errorf("err = %v, want ErrAthleteAlreadyLinked", err)
		}
	})
}
