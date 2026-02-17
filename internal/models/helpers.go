package models

import "strings"

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
