package api

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/carpenike/replog/internal/models"
)

// fixtureCredential returns a synthetic webauthn.Credential suitable for
// inserting via models.CreateWebAuthnCredential. We never run the actual
// WebAuthn ceremony in API tests \u2014 we just need a row in the table.
func fixtureCredential(idSuffix string) *webauthn.Credential {
	return &webauthn.Credential{
		ID:              []byte("test-credential-" + idSuffix),
		PublicKey:       []byte("test-public-key-" + idSuffix),
		AttestationType: "none",
		Transport:       []protocol.AuthenticatorTransport{"internal"},
		Flags: webauthn.CredentialFlags{
			UserPresent:  true,
			UserVerified: true,
		},
		Authenticator: webauthn.Authenticator{
			AAGUID:    []byte("0123456789abcdef"),
			SignCount: 0,
		},
	}
}

func TestListPasskeys_Empty(t *testing.T) {
	env := setupTest(t)
	user := env.createUser(t, "alice", false, false)
	cookies := env.loginAs(t, user)

	rr := env.do(t, "GET", "/api/passkeys", nil, cookies)
	requireStatus(t, rr, http.StatusOK)

	var got []PasskeyResponse
	decodeJSON(t, rr, &got)
	if len(got) != 0 {
		t.Errorf("expected 0 passkeys, got %d", len(got))
	}
}

func TestListPasskeys_ReturnsOwnCredentials(t *testing.T) {
	env := setupTest(t)
	alice := env.createUser(t, "alice", false, false)
	bob := env.createUser(t, "bob", false, false)

	// Insert one credential each.
	if _, err := models.CreateWebAuthnCredential(env.DB, alice.ID, fixtureCredential("alice"), "Alice's iPad"); err != nil {
		t.Fatalf("create alice's credential: %v", err)
	}
	if _, err := models.CreateWebAuthnCredential(env.DB, bob.ID, fixtureCredential("bob"), "Bob's phone"); err != nil {
		t.Fatalf("create bob's credential: %v", err)
	}

	cookies := env.loginAs(t, alice)
	rr := env.do(t, "GET", "/api/passkeys", nil, cookies)
	requireStatus(t, rr, http.StatusOK)

	var got []PasskeyResponse
	decodeJSON(t, rr, &got)
	if len(got) != 1 {
		t.Fatalf("alice should see exactly 1 passkey, got %d", len(got))
	}
	if got[0].Label == nil || *got[0].Label != "Alice's iPad" {
		t.Errorf("got label %v, want \"Alice's iPad\"", got[0].Label)
	}
}

func TestListPasskeys_Unauthenticated(t *testing.T) {
	env := setupTest(t)
	rr := env.do(t, "GET", "/api/passkeys", nil, nil)
	requireStatus(t, rr, http.StatusUnauthorized)
}

func TestDeletePasskey_OwnCredential(t *testing.T) {
	env := setupTest(t)
	user := env.createUser(t, "alice", false, false)
	wc, err := models.CreateWebAuthnCredential(env.DB, user.ID, fixtureCredential("alice"), "iPad")
	if err != nil {
		t.Fatalf("create credential: %v", err)
	}
	cookies := env.loginAs(t, user)

	rr := env.do(t, "DELETE", fmt.Sprintf("/api/passkeys/%d", wc.ID), nil, cookies)
	requireStatus(t, rr, http.StatusOK)

	// Listing should now be empty.
	rr = env.do(t, "GET", "/api/passkeys", nil, cookies)
	requireStatus(t, rr, http.StatusOK)
	var got []PasskeyResponse
	decodeJSON(t, rr, &got)
	if len(got) != 0 {
		t.Errorf("expected passkey to be gone, got %d", len(got))
	}
}

func TestDeletePasskey_OtherUserCannotDelete(t *testing.T) {
	// DeleteWebAuthnCredential filters by user_id, so a delete request from
	// the wrong user should silently no-op (returns OK but doesn't actually
	// remove anything). Verify by reading back.
	env := setupTest(t)
	alice := env.createUser(t, "alice", false, false)
	bob := env.createUser(t, "bob", false, false)
	bobCred, err := models.CreateWebAuthnCredential(env.DB, bob.ID, fixtureCredential("bob"), "Bob's phone")
	if err != nil {
		t.Fatalf("create bob's credential: %v", err)
	}

	// Alice tries to delete Bob's credential by ID.
	cookies := env.loginAs(t, alice)
	rr := env.do(t, "DELETE", fmt.Sprintf("/api/passkeys/%d", bobCred.ID), nil, cookies)
	requireStatus(t, rr, http.StatusOK)

	// Bob's credential should still exist.
	bobCookies := env.loginAs(t, bob)
	rr = env.do(t, "GET", "/api/passkeys", nil, bobCookies)
	requireStatus(t, rr, http.StatusOK)
	var got []PasskeyResponse
	decodeJSON(t, rr, &got)
	if len(got) != 1 {
		t.Errorf("bob's passkey should still exist after alice's attempt, got %d", len(got))
	}
}

func TestDeletePasskey_BadID(t *testing.T) {
	env := setupTest(t)
	user := env.createUser(t, "alice", false, false)
	cookies := env.loginAs(t, user)

	rr := env.do(t, "DELETE", "/api/passkeys/notanumber", nil, cookies)
	requireStatus(t, rr, http.StatusBadRequest)
}

func TestSetPasskeyLabel_StoresInSession(t *testing.T) {
	env := setupTest(t)
	user := env.createUser(t, "alice", false, false)
	cookies := env.loginAs(t, user)

	rr := env.do(t, "POST", "/api/passkeys/label",
		`{"label":"My YubiKey"}`, cookies)
	requireStatus(t, rr, http.StatusOK)

	var status map[string]string
	decodeJSON(t, rr, &status)
	if status["status"] != "ok" {
		t.Errorf("got status %q, want ok", status["status"])
	}
}

func TestSetPasskeyLabel_MalformedJSON(t *testing.T) {
	env := setupTest(t)
	user := env.createUser(t, "alice", false, false)
	cookies := env.loginAs(t, user)

	rr := env.do(t, "POST", "/api/passkeys/label", "not json", cookies)
	requireStatus(t, rr, http.StatusBadRequest)
}

func TestSetPasskeyLabel_Unauthenticated(t *testing.T) {
	env := setupTest(t)
	rr := env.do(t, "POST", "/api/passkeys/label", `{"label":"x"}`, nil)
	requireStatus(t, rr, http.StatusUnauthorized)
}
