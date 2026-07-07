package api

import (
	"encoding/json"
	"log"
	"net/http"
)

// WriteJSON writes a JSON response with the given status code. An encode
// failure (e.g. the client hung up mid-write, or v contains an unencodable
// value) is logged rather than silently discarded — the status/headers are
// already committed so we cannot change the response, but the log line makes
// the truncated body diagnosable.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("api: write json response (status %d): %v", status, err)
	}
}

// WriteError writes a JSON error response.
func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, APIError{
		Error: message,
		Code:  status,
	})
}
