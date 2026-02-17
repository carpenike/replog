package models

import (
	"database/sql"
	"testing"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

func TestCreateWebAuthnCredential(t *testing.T) {
	db := testDB(t)
	user, err := CreateUser(db, "passkeyuser", "password123", "", false, false, sql.NullInt64{})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	cred := &webauthn.Credential{
		ID:              []byte("test-credential-id-1234"),
		PublicKey:       []byte("test-public-key-bytes"),
		AttestationType: "none",
		Transport:       []protocol.AuthenticatorTransport{"internal"},
		Flags: webauthn.CredentialFlags{
			UserPresent:    true,
			UserVerified:   true,
			BackupEligible: false,
			BackupState:    false,
		},
		Authenticator: webauthn.Authenticator{
			AAGUID:    []byte("0123456789abcdef"),
			SignCount: 0,
		},
	}

	t.Run("basic create", func(t *testing.T) {
		wc, err := CreateWebAuthnCredential(db, user.ID, cred, "iPad")
		if err != nil {
			t.Fatalf("create webauthn credential: %v", err)
		}
		if wc.ID == 0 {
			t.Error("ID should not be zero")
		}
		if wc.UserID != user.ID {
			t.Errorf("UserID = %d, want %d", wc.UserID, user.ID)
		}
		if !wc.Label.Valid || wc.Label.String != "iPad" {
			t.Errorf("Label = %v, want iPad", wc.Label)
		}
		if !wc.UserPresent {
			t.Error("UserPresent should be true")
		}
		if !wc.UserVerified {
			t.Error("UserVerified should be true")
		}
		if wc.CreatedAt.IsZero() {
			t.Error("CreatedAt should be set")
		}
	})

	t.Run("no label", func(t *testing.T) {
		cred2 := &webauthn.Credential{
			ID:              []byte("credential-no-label"),
			PublicKey:       []byte("public-key-2"),
			AttestationType: "none",
			Authenticator: webauthn.Authenticator{
				AAGUID: []byte("0123456789abcdef"),
			},
		}
		wc, err := CreateWebAuthnCredential(db, user.ID, cred2, "")
		if err != nil {
			t.Fatalf("create webauthn credential without label: %v", err)
		}
		if wc.Label.Valid {
			t.Error("Label should be null when empty")
		}
	})

	t.Run("duplicate credential ID rejected", func(t *testing.T) {
		dupCred := &webauthn.Credential{
			ID:              []byte("test-credential-id-1234"), // same as first
			PublicKey:       []byte("different-key"),
			AttestationType: "none",
			Authenticator: webauthn.Authenticator{
				AAGUID: []byte("0123456789abcdef"),
			},
		}
		_, err := CreateWebAuthnCredential(db, user.ID, dupCred, "dup")
		if err == nil {
			t.Error("expected error for duplicate credential ID")
		}
	})
}

func TestListWebAuthnCredentialsByUser(t *testing.T) {
	db := testDB(t)
	user1, _ := CreateUser(db, "user1", "pass", "", false, false, sql.NullInt64{})
	user2, _ := CreateUser(db, "user2", "pass", "", false, false, sql.NullInt64{})

	// Create credentials for user1.
	for i, label := range []string{"iPad", "iPhone"} {
		cred := &webauthn.Credential{
			ID:              []byte("cred-" + label),
			PublicKey:       []byte("key-" + label),
			AttestationType: "none",
			Authenticator:   webauthn.Authenticator{AAGUID: []byte("aaguid-" + string(rune('0'+i)))},
		}
		if _, err := CreateWebAuthnCredential(db, user1.ID, cred, label); err != nil {
			t.Fatalf("create credential %s: %v", label, err)
		}
	}

	t.Run("returns user credentials", func(t *testing.T) {
		creds, err := ListWebAuthnCredentialsByUser(db, user1.ID)
		if err != nil {
			t.Fatalf("list credentials: %v", err)
		}
		if len(creds) != 2 {
			t.Fatalf("got %d credentials, want 2", len(creds))
		}
	})

	t.Run("empty for other user", func(t *testing.T) {
		creds, err := ListWebAuthnCredentialsByUser(db, user2.ID)
		if err != nil {
			t.Fatalf("list credentials: %v", err)
		}
		if len(creds) != 0 {
			t.Errorf("got %d credentials, want 0", len(creds))
		}
	})
}

func TestGetWebAuthnCredentialsByUser(t *testing.T) {
	db := testDB(t)
	user, _ := CreateUser(db, "libuser", "pass", "", false, false, sql.NullInt64{})

	cred := &webauthn.Credential{
		ID:              []byte("lib-cred-1"),
		PublicKey:       []byte("lib-key-1"),
		AttestationType: "none",
		Transport:       []protocol.AuthenticatorTransport{"internal", "hybrid"},
		Flags: webauthn.CredentialFlags{
			UserPresent:    true,
			UserVerified:   true,
			BackupEligible: true,
			BackupState:    false,
		},
		Authenticator: webauthn.Authenticator{
			AAGUID:    []byte("aaguid-lib"),
			SignCount: 5,
		},
	}
	if _, err := CreateWebAuthnCredential(db, user.ID, cred, "test device"); err != nil {
		t.Fatalf("create credential: %v", err)
	}

	libCreds, err := GetWebAuthnCredentialsByUser(db, user.ID)
	if err != nil {
		t.Fatalf("get library credentials: %v", err)
	}
	if len(libCreds) != 1 {
		t.Fatalf("got %d credentials, want 1", len(libCreds))
	}

	lc := libCreds[0]
	if string(lc.ID) != "lib-cred-1" {
		t.Errorf("credential ID = %s, want lib-cred-1", string(lc.ID))
	}
	if !lc.Flags.UserPresent {
		t.Error("UserPresent should be true")
	}
	if !lc.Flags.BackupEligible {
		t.Error("BackupEligible should be true")
	}
	if lc.Authenticator.SignCount != 5 {
		t.Errorf("SignCount = %d, want 5", lc.Authenticator.SignCount)
	}
	if len(lc.Transport) != 2 {
		t.Errorf("Transport length = %d, want 2", len(lc.Transport))
	}
}

func TestGetUserByCredentialID(t *testing.T) {
	db := testDB(t)
	user, _ := CreateUser(db, "credlookup", "pass", "", false, false, sql.NullInt64{})

	credID := []byte("lookup-cred-id")
	cred := &webauthn.Credential{
		ID:              credID,
		PublicKey:       []byte("lookup-key"),
		AttestationType: "none",
		Authenticator:   webauthn.Authenticator{AAGUID: []byte("aaguid-lookup")},
	}
	if _, err := CreateWebAuthnCredential(db, user.ID, cred, "lookup"); err != nil {
		t.Fatalf("create credential: %v", err)
	}

	t.Run("found", func(t *testing.T) {
		found, err := GetUserByCredentialID(db, credID)
		if err != nil {
			t.Fatalf("get user by credential ID: %v", err)
		}
		if found.ID != user.ID {
			t.Errorf("user ID = %d, want %d", found.ID, user.ID)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := GetUserByCredentialID(db, []byte("nonexistent"))
		if err == nil {
			t.Error("expected error for nonexistent credential")
		}
	})
}

func TestDeleteWebAuthnCredential(t *testing.T) {
	db := testDB(t)
	user, _ := CreateUser(db, "delcred", "pass", "", false, false, sql.NullInt64{})

	cred := &webauthn.Credential{
		ID:              []byte("del-cred-1"),
		PublicKey:       []byte("del-key-1"),
		AttestationType: "none",
		Authenticator:   webauthn.Authenticator{AAGUID: []byte("aaguid-del")},
	}
	wc, err := CreateWebAuthnCredential(db, user.ID, cred, "to delete")
	if err != nil {
		t.Fatalf("create credential: %v", err)
	}

	t.Run("delete existing", func(t *testing.T) {
		if err := DeleteWebAuthnCredential(db, wc.ID); err != nil {
			t.Fatalf("delete credential: %v", err)
		}
		creds, _ := ListWebAuthnCredentialsByUser(db, user.ID)
		if len(creds) != 0 {
			t.Errorf("got %d credentials, want 0", len(creds))
		}
	})

	t.Run("delete nonexistent", func(t *testing.T) {
		err := DeleteWebAuthnCredential(db, 99999)
		if err != ErrNotFound {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}

func TestUpdateWebAuthnCredentialSignCount(t *testing.T) {
	db := testDB(t)
	user, _ := CreateUser(db, "signcount", "pass", "", false, false, sql.NullInt64{})

	credID := []byte("signcount-cred")
	cred := &webauthn.Credential{
		ID:              credID,
		PublicKey:       []byte("signcount-key"),
		AttestationType: "none",
		Authenticator:   webauthn.Authenticator{AAGUID: []byte("aaguid-sc"), SignCount: 0},
	}
	if _, err := CreateWebAuthnCredential(db, user.ID, cred, "sign test"); err != nil {
		t.Fatalf("create credential: %v", err)
	}

	if err := UpdateWebAuthnCredentialSignCount(db, credID, 10, false); err != nil {
		t.Fatalf("update sign count: %v", err)
	}

	// Verify the update.
	libCreds, err := GetWebAuthnCredentialsByUser(db, user.ID)
	if err != nil {
		t.Fatalf("get credentials: %v", err)
	}
	if len(libCreds) != 1 {
		t.Fatalf("got %d credentials, want 1", len(libCreds))
	}
	if libCreds[0].Authenticator.SignCount != 10 {
		t.Errorf("SignCount = %d, want 10", libCreds[0].Authenticator.SignCount)
	}
}

func TestWebAuthnUser(t *testing.T) {
	db := testDB(t)
	user, _ := CreateUser(db, "wauser", "pass", "wa@test.com", false, false, sql.NullInt64{})

	cred := &webauthn.Credential{
		ID:              []byte("wa-user-cred"),
		PublicKey:       []byte("wa-user-key"),
		AttestationType: "none",
		Authenticator:   webauthn.Authenticator{AAGUID: []byte("aaguid-wa")},
	}
	if _, err := CreateWebAuthnCredential(db, user.ID, cred, "test"); err != nil {
		t.Fatalf("create credential: %v", err)
	}

	waUser := NewWebAuthnUser(user, db)

	t.Run("WebAuthnID round-trip", func(t *testing.T) {
		id := waUser.WebAuthnID()
		if len(id) != 8 {
			t.Fatalf("WebAuthnID length = %d, want 8", len(id))
		}
		recovered := UserIDFromWebAuthnID(id)
		if recovered != user.ID {
			t.Errorf("recovered ID = %d, want %d", recovered, user.ID)
		}
	})

	t.Run("WebAuthnName", func(t *testing.T) {
		if waUser.WebAuthnName() != "wauser" {
			t.Errorf("WebAuthnName = %q, want %q", waUser.WebAuthnName(), "wauser")
		}
	})

	t.Run("WebAuthnDisplayName", func(t *testing.T) {
		if waUser.WebAuthnDisplayName() != "wauser" {
			t.Errorf("WebAuthnDisplayName = %q, want %q", waUser.WebAuthnDisplayName(), "wauser")
		}
	})

	t.Run("WebAuthnCredentials", func(t *testing.T) {
		creds := waUser.WebAuthnCredentials()
		if len(creds) != 1 {
			t.Fatalf("got %d credentials, want 1", len(creds))
		}
		if string(creds[0].ID) != "wa-user-cred" {
			t.Errorf("credential ID = %s, want wa-user-cred", string(creds[0].ID))
		}
	})
}

func TestDeleteWebAuthnCredentialsByUser(t *testing.T) {
	db := testDB(t)
	user, _ := CreateUser(db, "delall", "pass", "", false, false, sql.NullInt64{})

	for i := 0; i < 3; i++ {
		cred := &webauthn.Credential{
			ID:              []byte("delall-cred-" + string(rune('a'+i))),
			PublicKey:       []byte("delall-key-" + string(rune('a'+i))),
			AttestationType: "none",
			Authenticator:   webauthn.Authenticator{AAGUID: []byte("aaguid-delall")},
		}
		if _, err := CreateWebAuthnCredential(db, user.ID, cred, ""); err != nil {
			t.Fatalf("create credential %d: %v", i, err)
		}
	}

	if err := DeleteWebAuthnCredentialsByUser(db, user.ID); err != nil {
		t.Fatalf("delete all credentials: %v", err)
	}

	creds, _ := ListWebAuthnCredentialsByUser(db, user.ID)
	if len(creds) != 0 {
		t.Errorf("got %d credentials, want 0", len(creds))
	}
}

func TestUserIDFromWebAuthnID(t *testing.T) {
	t.Run("short input", func(t *testing.T) {
		result := UserIDFromWebAuthnID([]byte{1, 2, 3})
		if result != 0 {
			t.Errorf("got %d, want 0 for short input", result)
		}
	})

	t.Run("nil input", func(t *testing.T) {
		result := UserIDFromWebAuthnID(nil)
		if result != 0 {
			t.Errorf("got %d, want 0 for nil input", result)
		}
	})
}
