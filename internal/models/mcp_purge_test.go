package models

import (
	"database/sql"
	"testing"
	"time"
)

func countRows(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func TestDeleteExpiredMCPTokens(t *testing.T) {
	db := testDB(t)
	u, err := CreateUser(db, "tokuser", "", "pw123456", "", false, false, sql.NullInt64{})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// A live token (expires 90 days out).
	_, live, err := CreateMCPToken(db, u.ID, "", "live")
	if err != nil {
		t.Fatalf("create live token: %v", err)
	}
	// A token that expired 40 days ago (past the grace cutoff).
	_, expOld, _ := CreateMCPToken(db, u.ID, "", "expired-old")
	if _, err := db.Exec(`UPDATE mcp_tokens SET expires_at = ? WHERE id = ?`,
		time.Now().Add(-40*24*time.Hour), expOld.ID); err != nil {
		t.Fatalf("backdate expiry: %v", err)
	}
	// A token that expired only 10 days ago (still within the grace window).
	_, expRecent, _ := CreateMCPToken(db, u.ID, "", "expired-recent")
	if _, err := db.Exec(`UPDATE mcp_tokens SET expires_at = ? WHERE id = ?`,
		time.Now().Add(-10*24*time.Hour), expRecent.ID); err != nil {
		t.Fatalf("backdate expiry: %v", err)
	}

	cutoff := time.Now().Add(-30 * 24 * time.Hour)
	n, err := DeleteExpiredMCPTokens(db, cutoff)
	if err != nil {
		t.Fatalf("delete expired: %v", err)
	}
	if n != 1 {
		t.Fatalf("deleted %d, want 1 (only the 40-day-old token)", n)
	}
	if got := countRows(t, db, "mcp_tokens"); got != 2 {
		t.Errorf("remaining tokens = %d, want 2 (live + within-grace)", got)
	}
	// The live token must still validate.
	if _, err := ValidateMCPTokenByID(db, live.ID); err != nil {
		t.Errorf("live token should survive: %v", err)
	}
}

// ValidateMCPTokenByID is a tiny test-only existence probe (avoids re-hashing).
func ValidateMCPTokenByID(db *sql.DB, id int64) (int64, error) {
	var uid int64
	err := db.QueryRow(`SELECT user_id FROM mcp_tokens WHERE id = ?`, id).Scan(&uid)
	return uid, err
}

func TestDeleteOrphanDCRClients(t *testing.T) {
	db := testDB(t)
	u, err := CreateUser(db, "dcruser", "", "pw123456", "", false, false, sql.NullInt64{})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	backdate := func(clientID string) {
		if _, err := db.Exec(`UPDATE dcr_clients SET created_at = ? WHERE client_id = ?`,
			time.Now().Add(-40*24*time.Hour), clientID); err != nil {
			t.Fatalf("backdate client: %v", err)
		}
	}

	// Orphan: old, never produced a token.
	orphan, _, err := RegisterDCRClient(db, "orphan", []string{"https://claude.ai/cb"}, "client_secret_post")
	if err != nil {
		t.Fatalf("register orphan: %v", err)
	}
	backdate(orphan.ClientID)

	// Active: old, but owns a token → must be retained.
	active, _, err := RegisterDCRClient(db, "active", []string{"https://claude.ai/cb"}, "client_secret_post")
	if err != nil {
		t.Fatalf("register active: %v", err)
	}
	backdate(active.ClientID)
	if _, _, err := CreateMCPToken(db, u.ID, active.ClientID, "tok"); err != nil {
		t.Fatalf("create token for active client: %v", err)
	}

	// Fresh: orphan but registered just now → within grace, must be retained.
	fresh, _, err := RegisterDCRClient(db, "fresh", []string{"https://claude.ai/cb"}, "client_secret_post")
	if err != nil {
		t.Fatalf("register fresh: %v", err)
	}

	n, err := DeleteOrphanDCRClients(db, time.Now().Add(-30*24*time.Hour))
	if err != nil {
		t.Fatalf("delete orphans: %v", err)
	}
	if n != 1 {
		t.Fatalf("deleted %d, want 1 (only the old orphan)", n)
	}

	if _, err := GetDCRClient(db, orphan.ClientID); err != ErrNotFound {
		t.Errorf("orphan should be deleted, got err=%v", err)
	}
	if _, err := GetDCRClient(db, active.ClientID); err != nil {
		t.Errorf("active client should survive: %v", err)
	}
	if _, err := GetDCRClient(db, fresh.ClientID); err != nil {
		t.Errorf("fresh client should survive: %v", err)
	}
}
