package models

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// bareDBCallRe matches context-less database calls on a *sql.DB or *sql.Tx
// handle: db.Query(, db.Exec(, db.QueryRow(, and the tx.* equivalents, plus
// db.Begin(. Every one of these has a *Context counterpart that threads a
// context.Context so the query is cancellable (client disconnect, server
// shutdown, LLM job timeout). Under SetMaxOpenConns(1) a single un-cancellable
// query blocks every other request, so the models layer must never regress to
// the bare forms.
//
// This guard is CI-enforceable (runs under `go test ./...`). If it trips, swap
// the offending call for its *Context variant and thread ctx down:
//
//	db.Query(...)     -> db.QueryContext(ctx, ...)
//	db.Exec(...)      -> db.ExecContext(ctx, ...)
//	db.QueryRow(...)  -> db.QueryRowContext(ctx, ...)
//	db.Begin()        -> db.BeginTx(ctx, nil)
var bareDBCallRe = regexp.MustCompile(`\b(?:db|tx)\.(?:Query|Exec|QueryRow)\(|\bdb\.Begin\(\)`)

// TestNoBareDBCalls fails if any non-test .go file in this package makes a
// context-less DB call. See bareDBCallRe for the rationale.
func TestNoBareDBCalls(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}

	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(filepath.Clean(name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		for i, line := range strings.Split(string(src), "\n") {
			if bareDBCallRe.MatchString(line) {
				t.Errorf("%s:%d: bare context-less DB call: %s\n"+
					"use the *Context variant (QueryContext/ExecContext/QueryRowContext/BeginTx) and thread ctx",
					name, i+1, strings.TrimSpace(line))
			}
		}
	}
}
