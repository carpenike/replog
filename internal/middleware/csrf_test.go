package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCSRFProtect_GeneratesToken(t *testing.T) {
	sm := testSessionManager()

	var gotToken string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = CSRFTokenFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := sm.LoadAndSave(CSRFProtect(sm, inner))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if gotToken == "" {
		t.Fatal("expected CSRF token in context, got empty")
	}
	if len(gotToken) != 64 { // 32 bytes hex-encoded
		t.Errorf("expected 64-char token, got %d chars", len(gotToken))
	}
}

func TestCSRFProtect_RejectsPostWithoutToken(t *testing.T) {
	sm := testSessionManager()

	var postCalled bool
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			postCalled = true
		}
		w.WriteHeader(http.StatusOK)
	})

	handler := sm.LoadAndSave(CSRFProtect(sm, inner))

	// First GET to establish session with CSRF token.
	getReq := httptest.NewRequest("GET", "/", nil)
	getRR := httptest.NewRecorder()
	handler.ServeHTTP(getRR, getReq)

	cookies := getRR.Result().Cookies()

	// POST without token.
	postReq := httptest.NewRequest("POST", "/", nil)
	for _, c := range cookies {
		postReq.AddCookie(c)
	}
	postRR := httptest.NewRecorder()
	handler.ServeHTTP(postRR, postReq)

	if postCalled {
		t.Error("handler should not be called for POST without CSRF token")
	}
	if postRR.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", postRR.Code)
	}
}

func TestCSRFProtect_AcceptsPostWithHeader(t *testing.T) {
	sm := testSessionManager()

	var called bool
	var gotToken string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		gotToken = CSRFTokenFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := sm.LoadAndSave(CSRFProtect(sm, inner))

	// GET to establish session.
	getReq := httptest.NewRequest("GET", "/", nil)
	getRR := httptest.NewRecorder()
	handler.ServeHTTP(getRR, getReq)

	cookies := getRR.Result().Cookies()
	token := gotToken

	// POST with correct header token.
	called = false
	postReq := httptest.NewRequest("POST", "/", nil)
	postReq.Header.Set("X-CSRF-Token", token)
	for _, c := range cookies {
		postReq.AddCookie(c)
	}
	postRR := httptest.NewRecorder()
	handler.ServeHTTP(postRR, postReq)

	if !called {
		t.Fatal("expected handler to be called with valid CSRF token")
	}
	if postRR.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", postRR.Code)
	}
}

func TestCSRFProtect_AcceptsPostWithFormField(t *testing.T) {
	sm := testSessionManager()

	var gotToken string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = CSRFTokenFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := sm.LoadAndSave(CSRFProtect(sm, inner))

	// GET to establish session.
	getReq := httptest.NewRequest("GET", "/", nil)
	getRR := httptest.NewRecorder()
	handler.ServeHTTP(getRR, getReq)

	cookies := getRR.Result().Cookies()
	token := gotToken

	// POST with form field.
	body := strings.NewReader("csrf_token=" + token)
	postReq := httptest.NewRequest("POST", "/", body)
	postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		postReq.AddCookie(c)
	}
	postRR := httptest.NewRecorder()
	handler.ServeHTTP(postRR, postReq)

	if postRR.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", postRR.Code)
	}
}

func TestCSRFProtect_RejectsWrongToken(t *testing.T) {
	sm := testSessionManager()

	var postCalled bool
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			postCalled = true
		}
		w.WriteHeader(http.StatusOK)
	})

	handler := sm.LoadAndSave(CSRFProtect(sm, inner))

	// GET to establish session.
	getReq := httptest.NewRequest("GET", "/", nil)
	getRR := httptest.NewRecorder()
	handler.ServeHTTP(getRR, getReq)

	cookies := getRR.Result().Cookies()

	// POST with wrong token.
	postReq := httptest.NewRequest("POST", "/", nil)
	postReq.Header.Set("X-CSRF-Token", "wrong-token-value")
	for _, c := range cookies {
		postReq.AddCookie(c)
	}
	postRR := httptest.NewRecorder()
	handler.ServeHTTP(postRR, postReq)

	if postCalled {
		t.Error("handler should not be called for POST with wrong CSRF token")
	}
	if postRR.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", postRR.Code)
	}
}

func TestCSRFProtect_AllowsGetWithoutToken(t *testing.T) {
	sm := testSessionManager()

	var called bool
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := sm.LoadAndSave(CSRFProtect(sm, inner))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Fatal("expected handler to be called for GET request")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestCSRFProtect_RejectsPutDeletePatchWithoutToken(t *testing.T) {
	sm := testSessionManager()

	for _, method := range []string{http.MethodPut, http.MethodDelete, http.MethodPatch} {
		t.Run(method, func(t *testing.T) {
			var mutatingCallCount int
			inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					mutatingCallCount++
				}
				w.WriteHeader(http.StatusOK)
			})

			handler := sm.LoadAndSave(CSRFProtect(sm, inner))

			// GET to establish session with CSRF token.
			getReq := httptest.NewRequest("GET", "/", nil)
			getRR := httptest.NewRecorder()
			handler.ServeHTTP(getRR, getReq)

			cookies := getRR.Result().Cookies()

			// State-changing request without token.
			req := httptest.NewRequest(method, "/", nil)
			for _, c := range cookies {
				req.AddCookie(c)
			}
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if mutatingCallCount > 0 {
				t.Errorf("handler should not be called for %s without CSRF token", method)
			}
			if rr.Code != http.StatusForbidden {
				t.Errorf("expected 403 for %s, got %d", method, rr.Code)
			}
		})
	}
}

func TestCSRFProtect_AcceptsPutDeletePatchWithToken(t *testing.T) {
	sm := testSessionManager()

	for _, method := range []string{http.MethodPut, http.MethodDelete, http.MethodPatch} {
		t.Run(method, func(t *testing.T) {
			var called bool
			var gotToken string
			inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				gotToken = CSRFTokenFromContext(r.Context())
				w.WriteHeader(http.StatusOK)
			})

			handler := sm.LoadAndSave(CSRFProtect(sm, inner))

			// GET to establish session.
			getReq := httptest.NewRequest("GET", "/", nil)
			getRR := httptest.NewRecorder()
			handler.ServeHTTP(getRR, getReq)

			cookies := getRR.Result().Cookies()
			token := gotToken

			// State-changing request with correct header token.
			called = false
			req := httptest.NewRequest(method, "/", nil)
			req.Header.Set("X-CSRF-Token", token)
			for _, c := range cookies {
				req.AddCookie(c)
			}
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if !called {
				t.Fatalf("expected handler to be called for %s with valid CSRF token", method)
			}
			if rr.Code != http.StatusOK {
				t.Errorf("expected 200 for %s, got %d", method, rr.Code)
			}
		})
	}
}