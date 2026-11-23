package models

import (
	"strings"
	"testing"
	"time"
)

func TestValidVideoShareLifetime(t *testing.T) {
	for _, d := range []time.Duration{24 * time.Hour, 7 * 24 * time.Hour, 30 * 24 * time.Hour} {
		if !ValidVideoShareLifetime(d) {
			t.Errorf("%v should be allowed", d)
		}
	}
	for _, d := range []time.Duration{0, -time.Hour, time.Hour, 48 * time.Hour, 30*24*time.Hour + time.Hour, 365 * 24 * time.Hour} {
		if ValidVideoShareLifetime(d) {
			t.Errorf("%v should be refused", d)
		}
	}
}

func TestVideoShareTokenShape(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		tok, err := newVideoShareToken()
		if err != nil {
			t.Fatal(err)
		}
		if len(tok) != 43 || !wellFormedShareToken(tok) || strings.ContainsAny(tok, "+/=") {
			t.Fatalf("token %q is not 32 bytes of unpadded base64url", tok)
		}
		if seen[tok] {
			t.Fatal("duplicate token")
		}
		seen[tok] = true
		if h := HashVideoShareToken(tok); len(h) != 64 || h == tok {
			t.Fatalf("hash %q", h)
		}
	}
	for _, bad := range []string{"", "abc", strings.Repeat("A", 42), strings.Repeat("A", 44), strings.Repeat("*", 43)} {
		if wellFormedShareToken(bad) {
			t.Errorf("%q accepted as well formed", bad)
		}
	}
}

func TestVideoShareStatus(t *testing.T) {
	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	s := &VideoShare{ExpiresAt: now.Add(time.Minute)}
	if got := s.Status(now); got != VideoShareActive {
		t.Fatalf("status = %s", got)
	}
	if got := s.Status(now.Add(time.Minute)); got != VideoShareExpired {
		t.Fatalf("status at expiry = %s", got)
	}
	s.RevokedAt.Valid = true
	if got := s.Status(now); got != VideoShareRevoked {
		t.Fatalf("revoked status = %s", got)
	}
}
