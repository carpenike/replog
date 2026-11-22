package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestRequestLoggerUnwrap checks a handler behind RequestLogger can still
// set per-request deadlines (the form-check video routes rely on this).
func TestRequestLoggerUnwrap(t *testing.T) {
	var deadlineErr error
	h := RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rc := http.NewResponseController(w)
		deadlineErr = rc.SetReadDeadline(time.Now().Add(time.Minute))
		if deadlineErr == nil {
			deadlineErr = rc.SetWriteDeadline(time.Now().Add(time.Minute))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	srv := httptest.NewServer(h)
	defer srv.Close()
	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if deadlineErr != nil {
		t.Fatalf("set deadline through RequestLogger: %v", deadlineErr)
	}
}
