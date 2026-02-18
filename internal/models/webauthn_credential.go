package models

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

// WebAuthnCredential represents a stored WebAuthn/passkey credential.
type WebAuthnCredential struct {
	ID               int64
	UserID           int64
	CredentialID     []byte
	PublicKey        []byte
	AttestationType  string
	Transport        []protocol.AuthenticatorTransport
	SignCount        uint32
	CloneWarning     bool
	Attachment       protocol.AuthenticatorAttachment
	AAGUID           []byte
	UserPresent      bool
	UserVerified     bool
	BackupEligible   bool
	BackupState      bool
	Label            sql.NullString
	CreatedAt        time.Time
}

// ToLibrary converts a stored credential to the go-webauthn library Credential type.
func (c *WebAuthnCredential) ToLibrary() webauthn.Credential {
	return webauthn.Credential{
		ID:              c.CredentialID,
		PublicKey:       c.PublicKey,
		AttestationType: c.AttestationType,
		Transport:       c.Transport,
		Flags: webauthn.CredentialFlags{
			UserPresent:    c.UserPresent,
			UserVerified:   c.UserVerified,
			BackupEligible: c.BackupEligible,
			BackupState:    c.BackupState,
		},
		Authenticator: webauthn.Authenticator{
			AAGUID:       c.AAGUID,
			SignCount:    c.SignCount,
			CloneWarning: c.CloneWarning,
			Attachment:   c.Attachment,
		},
	}
}

// marshalTransport serializes transport list to JSON for storage.
func marshalTransport(transports []protocol.AuthenticatorTransport) (sql.NullString, error) {
	if len(transports) == 0 {
		return sql.NullString{}, nil
	}
	b, err := json.Marshal(transports)
	if err != nil {
		return sql.NullString{}, fmt.Errorf("models: marshal transport: %w", err)
	}
	return sql.NullString{String: string(b), Valid: true}, nil
}

// unmarshalTransport deserializes transport JSON from storage.
func unmarshalTransport(s sql.NullString) ([]protocol.AuthenticatorTransport, error) {
	if !s.Valid || s.String == "" {
		return nil, nil
	}
	var transports []protocol.AuthenticatorTransport
	if err := json.Unmarshal([]byte(s.String), &transports); err != nil {
		return nil, fmt.Errorf("models: unmarshal transport: %w", err)
	}
	return transports, nil
}

// CreateWebAuthnCredential stores a new WebAuthn credential for a user.
func CreateWebAuthnCredential(db *sql.DB, userID int64, cred *webauthn.Credential, label string) (*WebAuthnCredential, error) {
	transport, err := marshalTransport(cred.Transport)
	if err != nil {
		return nil, err
	}

	var labelNull sql.NullString
	if label != "" {
		labelNull = sql.NullString{String: label, Valid: true}
	}

	var wc WebAuthnCredential
	var transportStr sql.NullString
	err = db.QueryRow(`
		INSERT INTO webauthn_credentials (
			user_id, credential_id, public_key, attestation_type, transport,
			sign_count, clone_warning, attachment, aaguid,
			flags_user_present, flags_user_verified, flags_backup_eligible, flags_backup_state,
			label
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id, user_id, credential_id, public_key, attestation_type, transport,
		          sign_count, clone_warning, attachment, aaguid,
		          flags_user_present, flags_user_verified, flags_backup_eligible, flags_backup_state,
		          label, created_at`,
		userID, cred.ID, cred.PublicKey, cred.AttestationType, transport,
		cred.Authenticator.SignCount,
		boolToInt(cred.Authenticator.CloneWarning),
		string(cred.Authenticator.Attachment),
		cred.Authenticator.AAGUID,
		boolToInt(cred.Flags.UserPresent),
		boolToInt(cred.Flags.UserVerified),
		boolToInt(cred.Flags.BackupEligible),
		boolToInt(cred.Flags.BackupState),
		labelNull,
	).Scan(
		&wc.ID, &wc.UserID, &wc.CredentialID, &wc.PublicKey, &wc.AttestationType,
		&transportStr, &wc.SignCount, &wc.CloneWarning, &wc.Attachment, &wc.AAGUID,
		&wc.UserPresent, &wc.UserVerified, &wc.BackupEligible, &wc.BackupState,
		&wc.Label, &wc.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("models: create webauthn credential: %w", err)
	}

	wc.Transport, err = unmarshalTransport(transportStr)
	if err != nil {
		return nil, err
	}

	return &wc, nil
}

// ListWebAuthnCredentialsByUser returns all WebAuthn credentials for a user.
func ListWebAuthnCredentialsByUser(db *sql.DB, userID int64) ([]*WebAuthnCredential, error) {
	rows, err := db.Query(`
		SELECT id, user_id, credential_id, public_key, attestation_type, transport,
		       sign_count, clone_warning, attachment, aaguid,
		       flags_user_present, flags_user_verified, flags_backup_eligible, flags_backup_state,
		       label, created_at
		FROM webauthn_credentials
		WHERE user_id = ?
		ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("models: list webauthn credentials for user %d: %w", userID, err)
	}
	defer rows.Close()

	var creds []*WebAuthnCredential
	for rows.Next() {
		var wc WebAuthnCredential
		var transportStr sql.NullString
		if err := rows.Scan(
			&wc.ID, &wc.UserID, &wc.CredentialID, &wc.PublicKey, &wc.AttestationType,
			&transportStr, &wc.SignCount, &wc.CloneWarning, &wc.Attachment, &wc.AAGUID,
			&wc.UserPresent, &wc.UserVerified, &wc.BackupEligible, &wc.BackupState,
			&wc.Label, &wc.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("models: scan webauthn credential: %w", err)
		}
		wc.Transport, err = unmarshalTransport(transportStr)
		if err != nil {
			return nil, err
		}
		creds = append(creds, &wc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("models: iterate webauthn credentials: %w", err)
	}
	return creds, nil
}

// GetWebAuthnCredentialsByUser returns all credentials as webauthn.Credential for the library.
func GetWebAuthnCredentialsByUser(db *sql.DB, userID int64) ([]webauthn.Credential, error) {
	creds, err := ListWebAuthnCredentialsByUser(db, userID)
	if err != nil {
		return nil, err
	}

	result := make([]webauthn.Credential, len(creds))
	for i, c := range creds {
		result[i] = c.ToLibrary()
	}
	return result, nil
}

// GetUserByCredentialID looks up a user by a WebAuthn credential ID.
func GetUserByCredentialID(db *sql.DB, credentialID []byte) (*User, error) {
	var user User
	err := db.QueryRow(`
		SELECT u.id, u.username, u.name, u.email, COALESCE(u.password_hash, ''), u.athlete_id, u.is_coach,
		       u.is_admin, u.avatar_path, u.created_at, u.updated_at
		FROM users u
		INNER JOIN webauthn_credentials wc ON wc.user_id = u.id
		WHERE wc.credential_id = ?`, credentialID,
	).Scan(&user.ID, &user.Username, &user.Name, &user.Email, &user.PasswordHash,
		&user.AthleteID, &user.IsCoach, &user.IsAdmin, &user.AvatarPath, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("models: get user by credential id: %w", err)
	}
	return &user, nil
}

// UpdateWebAuthnCredentialSignCount updates the sign count and clone warning for a credential.
func UpdateWebAuthnCredentialSignCount(db *sql.DB, credentialID []byte, signCount uint32, cloneWarning bool) error {
	_, err := db.Exec(`
		UPDATE webauthn_credentials
		SET sign_count = ?, clone_warning = ?
		WHERE credential_id = ?`,
		signCount, boolToInt(cloneWarning), credentialID)
	if err != nil {
		return fmt.Errorf("models: update webauthn sign count: %w", err)
	}
	return nil
}

// DeleteWebAuthnCredential removes a WebAuthn credential by its row ID.
func DeleteWebAuthnCredential(db *sql.DB, id int64) error {
	result, err := db.Exec(`DELETE FROM webauthn_credentials WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("models: delete webauthn credential %d: %w", id, err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteWebAuthnCredentialsByUser removes all WebAuthn credentials for a user.
func DeleteWebAuthnCredentialsByUser(db *sql.DB, userID int64) error {
	_, err := db.Exec(`DELETE FROM webauthn_credentials WHERE user_id = ?`, userID)
	if err != nil {
		return fmt.Errorf("models: delete webauthn credentials for user %d: %w", userID, err)
	}
	return nil
}

// boolToInt converts a bool to 0 or 1 for SQLite storage.
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
