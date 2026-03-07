package api

import (
	"encoding/json"
	"net/http"
	"strings"
)

// WantsJSON returns true if the request prefers a JSON response.
func WantsJSON(r *http.Request) bool {
	if r.URL.Query().Get("format") == "json" {
		return true
	}
	accept := r.Header.Get("Accept")
	for _, part := range strings.Split(accept, ",") {
		mt := strings.TrimSpace(strings.SplitN(part, ";", 2)[0])
		if mt == "application/json" {
			return true
		}
	}
	return false
}

// WriteJSON writes a JSON response with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// WriteError writes a JSON error response.
func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, APIError{
		Error: message,
		Code:  status,
	})
}

// WriteValidationError writes a 422 JSON error with field-level details.
func WriteValidationError(w http.ResponseWriter, message string, details map[string]string) {
	WriteJSON(w, http.StatusUnprocessableEntity, APIError{
		Error:   message,
		Code:    http.StatusUnprocessableEntity,
		Details: details,
	})
}
