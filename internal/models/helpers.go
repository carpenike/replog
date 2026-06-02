package models

import (
	"database/sql"
	"strings"
)

// isUniqueViolation checks if a SQLite error is a unique constraint violation.
func isUniqueViolation(err error) bool {
	return err != nil && (errContains(err, "UNIQUE constraint failed") || errContains(err, "constraint failed: UNIQUE"))
}

// errContains checks whether an error's message contains the given substring.
func errContains(err error, substr string) bool {
	return err != nil && strings.Contains(err.Error(), substr)
}

// normalizeDate trims any time suffix from a date string (e.g. "2025-01-01T00:00:00Z" → "2025-01-01").
func normalizeDate(d string) string {
	if len(d) >= 10 {
		return d[:10]
	}
	return d
}

// boolToInt maps a Go bool to the 0/1 integer SQLite uses for boolean columns.
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// nullableInt64 wraps an optional int64 pointer as a sql.NullInt64.
func nullableInt64(p *int64) sql.NullInt64 {
	if p == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *p, Valid: true}
}

// nullableFloat64 wraps an optional float64 pointer as a sql.NullFloat64.
func nullableFloat64(p *float64) sql.NullFloat64 {
	if p == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: *p, Valid: true}
}

// nullableString wraps a string as a sql.NullString, treating "" as NULL.
func nullableString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
