package models

import (
	"database/sql"
	"encoding/binary"

	"github.com/go-webauthn/webauthn/webauthn"
)

// WebAuthnUser wraps a User with its stored credentials to satisfy the
// webauthn.User interface required by the go-webauthn library.
type WebAuthnUser struct {
	User        *User
	DB          *sql.DB
	credentials []webauthn.Credential // cached after first load
}

// NewWebAuthnUser creates a WebAuthnUser from an existing User.
func NewWebAuthnUser(user *User, db *sql.DB) *WebAuthnUser {
	return &WebAuthnUser{User: user, DB: db}
}

// WebAuthnID returns the user handle — an opaque byte sequence derived from the
// user's integer ID. This is NOT the username.
func (u *WebAuthnUser) WebAuthnID() []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(u.User.ID))
	return buf
}

// WebAuthnName returns the username for display during registration.
func (u *WebAuthnUser) WebAuthnName() string {
	return u.User.Username
}

// WebAuthnDisplayName returns a human-readable display name.
func (u *WebAuthnUser) WebAuthnDisplayName() string {
	return u.User.Username
}

// WebAuthnCredentials returns all registered credentials for this user.
func (u *WebAuthnUser) WebAuthnCredentials() []webauthn.Credential {
	if u.credentials != nil {
		return u.credentials
	}

	creds, err := GetWebAuthnCredentialsByUser(u.DB, u.User.ID)
	if err != nil {
		return nil
	}
	u.credentials = creds
	return u.credentials
}

// LoadCredentials eagerly loads credentials from the database, returning any error.
func (u *WebAuthnUser) LoadCredentials() error {
	creds, err := GetWebAuthnCredentialsByUser(u.DB, u.User.ID)
	if err != nil {
		return err
	}
	u.credentials = creds
	return nil
}

// UserIDFromWebAuthnID converts a WebAuthn user handle back to a user ID.
func UserIDFromWebAuthnID(id []byte) int64 {
	if len(id) < 8 {
		return 0
	}
	return int64(binary.BigEndian.Uint64(id))
}
