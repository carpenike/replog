package models

import (
	"database/sql"
	"errors"
	"testing"
)

// TestUpsertUserFromOIDC exercises the PocketID relying-party JIT/binding
// logic (ADR 019 Phase 1 — HOF-012). The security-sensitive paths are:
//   - an empty subject is rejected (mirrors the bearer empty-claim 401);
//   - a returning user is matched by sub (steady state);
//   - an existing account is bound by email ONLY when email_verified is true;
//   - an unverified email never binds to an existing account — it JIT-creates;
//   - JIT-created usernames are unique even when local-parts collide.
func TestUpsertUserFromOIDC(t *testing.T) {
	t.Run("empty sub is rejected", func(t *testing.T) {
		db := testDB(t)
		if _, err := UpsertUserFromOIDC(db, "", "a@example.com", "A", true); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("empty sub: err = %v, want ErrInvalidInput", err)
		}
	})

	t.Run("steady state: match by sub", func(t *testing.T) {
		db := testDB(t)
		first, err := UpsertUserFromOIDC(db, "sub-1", "kid@example.com", "Kid", true)
		if err != nil {
			t.Fatalf("first upsert: %v", err)
		}
		again, err := UpsertUserFromOIDC(db, "sub-1", "kid@example.com", "Kid", true)
		if err != nil {
			t.Fatalf("second upsert: %v", err)
		}
		if again.ID != first.ID {
			t.Errorf("matched a different user: got %d, want %d", again.ID, first.ID)
		}
	})

	t.Run("verified email binds to existing account", func(t *testing.T) {
		db := testDB(t)
		existing, err := CreateUser(db, "coach", "Coach", "pw123456", "coach@example.com", true, true, sql.NullInt64{})
		if err != nil {
			t.Fatalf("seed user: %v", err)
		}
		got, err := UpsertUserFromOIDC(db, "sub-coach", "coach@example.com", "Coach", true)
		if err != nil {
			t.Fatalf("upsert: %v", err)
		}
		if got.ID != existing.ID {
			t.Errorf("did not bind to existing account: got %d, want %d", got.ID, existing.ID)
		}
		bound, err := GetUserByPocketIDSub(db, "sub-coach")
		if err != nil {
			t.Fatalf("lookup by sub: %v", err)
		}
		if bound.ID != existing.ID {
			t.Errorf("sub bound to wrong user: got %d, want %d", bound.ID, existing.ID)
		}
	})

	t.Run("unverified email does NOT bind, JIT-creates instead", func(t *testing.T) {
		db := testDB(t)
		existing, err := CreateUser(db, "victim", "Victim", "pw123456", "victim@example.com", true, true, sql.NullInt64{})
		if err != nil {
			t.Fatalf("seed user: %v", err)
		}
		got, err := UpsertUserFromOIDC(db, "sub-attacker", "victim@example.com", "Attacker", false)
		if err != nil {
			t.Fatalf("upsert: %v", err)
		}
		if got.ID == existing.ID {
			t.Fatal("unverified email hijacked the existing account — security regression")
		}
		// The JIT user must not have inherited the (unverified) email.
		if got.Email.Valid && got.Email.String == "victim@example.com" {
			t.Error("JIT user adopted an unverified email")
		}
	})

	t.Run("colliding local-parts get unique usernames", func(t *testing.T) {
		db := testDB(t)
		a, err := UpsertUserFromOIDC(db, "sub-a", "sam@a.example.com", "Sam A", true)
		if err != nil {
			t.Fatalf("upsert a: %v", err)
		}
		b, err := UpsertUserFromOIDC(db, "sub-b", "sam@b.example.com", "Sam B", true)
		if err != nil {
			t.Fatalf("upsert b: %v", err)
		}
		if a.Username == b.Username {
			t.Errorf("usernames collided: both %q", a.Username)
		}
	})
}
