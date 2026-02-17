package models

import (
	"database/sql"
	"testing"
	"time"
)

func TestCreateLoginToken(t *testing.T) {
	db := testDB(t)
	user, err := CreateUser(db, "kid1", "password123", "", false, sql.NullInt64{})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	t.Run("basic create", func(t *testing.T) {
		lt, err := CreateLoginToken(db, user.ID, "iPad", nil)
		if err != nil {
			t.Fatalf("create login token: %v", err)
		}
		if lt.Token == "" {
			t.Error("token should not be empty")
		}
		if len(lt.Token) != 64 { // 32 bytes = 64 hex chars
			t.Errorf("token length = %d, want 64", len(lt.Token))
		}
		if lt.UserID != user.ID {
			t.Errorf("user_id = %d, want %d", lt.UserID, user.ID)
		}
		if !lt.Label.Valid || lt.Label.String != "iPad" {
			t.Errorf("label = %v, want iPad", lt.Label)
		}
		if lt.ExpiresAt.Valid {
			t.Error("expires_at should be null for no-expiry token")
		}
	})

	t.Run("with expiry", func(t *testing.T) {
		expires := time.Now().Add(24 * time.Hour)
		lt, err := CreateLoginToken(db, user.ID, "iPhone", &expires)
		if err != nil {
			t.Fatalf("create login token with expiry: %v", err)
		}
		if !lt.ExpiresAt.Valid {
			t.Error("expires_at should be set")
		}
	})

	t.Run("no label", func(t *testing.T) {
		lt, err := CreateLoginToken(db, user.ID, "", nil)
		if err != nil {
			t.Fatalf("create login token without label: %v", err)
		}
		if lt.Label.Valid {
			t.Error("label should be null when empty")
		}
	})

	t.Run("unique tokens", func(t *testing.T) {
		lt1, _ := CreateLoginToken(db, user.ID, "device1", nil)
		lt2, _ := CreateLoginToken(db, user.ID, "device2", nil)
		if lt1.Token == lt2.Token {
			t.Error("tokens should be unique")
		}
	})
}

func TestValidateLoginToken(t *testing.T) {
	db := testDB(t)
	user, _ := CreateUser(db, "kid2", "password123", "", false, sql.NullInt64{})

	t.Run("valid token", func(t *testing.T) {
		lt, _ := CreateLoginToken(db, user.ID, "iPad", nil)
		u, err := ValidateLoginToken(db, lt.Token)
		if err != nil {
			t.Fatalf("validate token: %v", err)
		}
		if u.ID != user.ID {
			t.Errorf("user id = %d, want %d", u.ID, user.ID)
		}
		if u.Username != "kid2" {
			t.Errorf("username = %q, want kid2", u.Username)
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		_, err := ValidateLoginToken(db, "nonexistent-token-value")
		if err != ErrNotFound {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})

	t.Run("expired token", func(t *testing.T) {
		past := time.Now().Add(-1 * time.Hour)
		lt, _ := CreateLoginToken(db, user.ID, "expired", &past)
		_, err := ValidateLoginToken(db, lt.Token)
		if err != ErrNotFound {
			t.Errorf("err = %v, want ErrNotFound for expired token", err)
		}
	})

	t.Run("future expiry is valid", func(t *testing.T) {
		future := time.Now().Add(24 * time.Hour)
		lt, _ := CreateLoginToken(db, user.ID, "future", &future)
		u, err := ValidateLoginToken(db, lt.Token)
		if err != nil {
			t.Fatalf("validate future token: %v", err)
		}
		if u.ID != user.ID {
			t.Errorf("user id = %d, want %d", u.ID, user.ID)
		}
	})
}

func TestListLoginTokensByUser(t *testing.T) {
	db := testDB(t)
	user, _ := CreateUser(db, "kid3", "password123", "", false, sql.NullInt64{})

	t.Run("empty list", func(t *testing.T) {
		tokens, err := ListLoginTokensByUser(db, user.ID)
		if err != nil {
			t.Fatalf("list tokens: %v", err)
		}
		if len(tokens) != 0 {
			t.Errorf("len = %d, want 0", len(tokens))
		}
	})

	t.Run("multiple tokens", func(t *testing.T) {
		CreateLoginToken(db, user.ID, "device1", nil)
		CreateLoginToken(db, user.ID, "device2", nil)
		CreateLoginToken(db, user.ID, "device3", nil)

		tokens, err := ListLoginTokensByUser(db, user.ID)
		if err != nil {
			t.Fatalf("list tokens: %v", err)
		}
		if len(tokens) != 3 {
			t.Errorf("len = %d, want 3", len(tokens))
		}
	})
}

func TestDeleteLoginToken(t *testing.T) {
	db := testDB(t)
	user, _ := CreateUser(db, "kid4", "password123", "", false, sql.NullInt64{})

	t.Run("delete existing", func(t *testing.T) {
		lt, _ := CreateLoginToken(db, user.ID, "to-delete", nil)
		err := DeleteLoginToken(db, lt.ID)
		if err != nil {
			t.Fatalf("delete token: %v", err)
		}

		// Verify it's gone.
		_, err = ValidateLoginToken(db, lt.Token)
		if err != ErrNotFound {
			t.Errorf("err = %v, want ErrNotFound after delete", err)
		}
	})

	t.Run("delete non-existent", func(t *testing.T) {
		err := DeleteLoginToken(db, 99999)
		if err != ErrNotFound {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}

func TestDeleteLoginTokensByUser(t *testing.T) {
	db := testDB(t)
	user, _ := CreateUser(db, "kid5", "password123", "", false, sql.NullInt64{})

	CreateLoginToken(db, user.ID, "d1", nil)
	CreateLoginToken(db, user.ID, "d2", nil)

	err := DeleteLoginTokensByUser(db, user.ID)
	if err != nil {
		t.Fatalf("delete all tokens: %v", err)
	}

	tokens, _ := ListLoginTokensByUser(db, user.ID)
	if len(tokens) != 0 {
		t.Errorf("len = %d, want 0 after delete all", len(tokens))
	}
}

func TestIsExpired(t *testing.T) {
	t.Run("no expiry", func(t *testing.T) {
		lt := &LoginToken{}
		if lt.IsExpired() {
			t.Error("token without expiry should not be expired")
		}
	})

	t.Run("past expiry", func(t *testing.T) {
		lt := &LoginToken{
			ExpiresAt: sql.NullTime{Time: time.Now().Add(-1 * time.Hour), Valid: true},
		}
		if !lt.IsExpired() {
			t.Error("token with past expiry should be expired")
		}
	})

	t.Run("future expiry", func(t *testing.T) {
		lt := &LoginToken{
			ExpiresAt: sql.NullTime{Time: time.Now().Add(1 * time.Hour), Valid: true},
		}
		if lt.IsExpired() {
			t.Error("token with future expiry should not be expired")
		}
	})
}

func TestLoginTokenCascadeDelete(t *testing.T) {
	db := testDB(t)
	user, _ := CreateUser(db, "kid6", "password123", "", false, sql.NullInt64{})
	CreateLoginToken(db, user.ID, "device", nil)

	// Delete the user — tokens should cascade.
	err := DeleteUser(db, user.ID)
	if err != nil {
		t.Fatalf("delete user: %v", err)
	}

	tokens, _ := ListLoginTokensByUser(db, user.ID)
	if len(tokens) != 0 {
		t.Errorf("len = %d, want 0 after user delete cascade", len(tokens))
	}
}
