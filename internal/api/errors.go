package api

import (
	"net/http"
	"strconv"
	"strings"
)

// parsePage reads the standard `limit` and `offset` query params and clamps
// them into a safe range: limit falls back to defaultLimit when missing,
// garbage, or < 1 and is capped at maxLimit; a negative or garbage offset
// clamps to 0. This prevents a caller-supplied limit=-1 from reaching SQLite
// (where LIMIT -1 means "unlimited") or a negative OFFSET from erroring.
func parsePage(r *http.Request, defaultLimit, maxLimit int) (limit, offset int) {
	limit = defaultLimit
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			limit = n
		}
	}
	if limit < 1 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if n, err := strconv.Atoi(o); err == nil && n > 0 {
			offset = n
		}
	}
	return limit, offset
}

// WriteValidationError writes a 400 with a per-field detail map so the SPA can
// surface the problem inline. Use this for boundary validation of enums and
// numeric ranges that would otherwise fail a DB CHECK constraint as a 500.
func WriteValidationError(w http.ResponseWriter, field, message string) {
	WriteJSON(w, http.StatusBadRequest, APIError{
		Error:   "validation failed",
		Code:    http.StatusBadRequest,
		Details: map[string]string{field: message},
	})
}

// WriteDBError maps a database error to an appropriate HTTP status as a
// last-resort classifier for CHECK/UNIQUE constraint violations that slip past
// boundary validation. A CHECK failure becomes 400, a UNIQUE collision becomes
// 409, and anything else becomes a 500 with the caller-supplied generic
// message (the detailed error is expected to already be logged by the caller).
func WriteDBError(w http.ResponseWriter, err error, fallbackMsg string) {
	switch {
	case isCheckConstraintErr(err):
		WriteError(w, http.StatusBadRequest, "one or more fields have invalid values")
	case isUniqueConstraintErr(err):
		WriteError(w, http.StatusConflict, "conflicts with an existing record")
	default:
		WriteError(w, http.StatusInternalServerError, fallbackMsg)
	}
}

func isCheckConstraintErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "CHECK constraint failed")
}

func isUniqueConstraintErr(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "UNIQUE constraint failed") ||
		strings.Contains(err.Error(), "constraint failed: UNIQUE"))
}

// --- CHECK-constrained enum validators (schema: 0001_initial_schema.sql) ---

func validSetRepType(v string) bool {
	switch v {
	case "reps", "each_side", "seconds", "distance":
		return true
	}
	return false
}

func validSetCategory(v string) bool {
	switch v {
	case "main", "supplemental", "accessory":
		return true
	}
	return false
}

// validGender reports whether v is an accepted athlete gender. Empty is allowed
// (the column is nullable); non-empty must match the CHECK constraint.
func validGender(v string) bool {
	return v == "" || v == "male" || v == "female"
}

// validTier reports whether v is an accepted athlete/methodology tier. Empty is
// allowed (nullable column).
func validTier(v string) bool {
	switch v {
	case "", "foundational", "intermediate", "sport_performance":
		return true
	}
	return false
}

// validateAthleteFields validates the CHECK-constrained/format-sensitive fields
// of an AthleteRequest at the boundary, writing a 400 and returning false on the
// first problem so a bad value becomes a clear validation error instead of a
// 500 from a DB CHECK failure. Returns true when the request is acceptable.
func validateAthleteFields(w http.ResponseWriter, req *AthleteRequest) bool {
	if !validGender(req.Gender) {
		WriteValidationError(w, "gender", "must be 'male' or 'female'")
		return false
	}
	if !validTier(req.Tier) {
		WriteValidationError(w, "tier", "must be one of foundational, intermediate, sport_performance")
		return false
	}
	if req.DateOfBirth != "" && !validDate(req.DateOfBirth) {
		WriteValidationError(w, "date_of_birth", "must be a valid date in YYYY-MM-DD format")
		return false
	}
	return true
}
