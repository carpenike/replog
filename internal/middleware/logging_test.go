package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestLogger_LogsRequestAndSetsStatus(t *testing.T) {
	handler := RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	req := httptest.NewRequest("GET", "/test-path", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rr.Code)
	}
}

func TestRequestLogger_DefaultsTo200(t *testing.T) {
	handler := RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello"))
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestStatusWriter_CapturesFirstWriteHeader(t *testing.T) {
	rr := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: rr, status: http.StatusOK}

	sw.WriteHeader(http.StatusCreated)
	sw.WriteHeader(http.StatusNotFound) // second call should be ignored by our tracking

	if sw.status != http.StatusCreated {
		t.Errorf("expected captured status 201, got %d", sw.status)
	}
}
