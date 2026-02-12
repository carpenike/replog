package models

import (
	"testing"
)

func TestCreateUser(t *testing.T) {
	db := testDB(t)

	t.Run("basic create", func(t *testing.T) {
		u, err := CreateUser(db, "admin", "password123", "admin@test.com", true)
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
		_, err := CreateUser(db, "admin", "other", "", false)
		if err != ErrDuplicateUsername {
			t.Errorf("err = %v, want ErrDuplicateUsername", err)
		}
	})

	t.Run("case insensitive duplicate", func(t *testing.T) {
		_, err := CreateUser(db, "ADMIN", "other", "", false)
		if err != ErrDuplicateUsername {
			t.Errorf("err = %v, want ErrDuplicateUsername", err)
		}
	})
}

func TestAuthenticate(t *testing.T) {
	db := testDB(t)

	CreateUser(db, "testuser", "correct-password", "", false)

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

	CreateUser(db, "u1", "pass", "", false)
	CreateUser(db, "u2", "pass", "", true)

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

	u, _ := CreateUser(db, "pwuser", "old-password", "", false)

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

	u, _ := CreateUser(db, "delme", "pass", "", false)

	if err := DeleteUser(db, u.ID); err != nil {
		t.Fatalf("delete user: %v", err)
	}

	_, err := GetUserByID(db, u.ID)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}
