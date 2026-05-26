package api

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/carpenike/replog/internal/llm"
	"github.com/carpenike/replog/internal/models"
)

// withSecretKey ensures REPLOG_SECRET_KEY is set for the duration of a test
// (so SetSetting can encrypt and GetSetting can decrypt). t.Setenv handles
// restore automatically when the test ends.
func withSecretKey(t *testing.T) {
	t.Helper()
	t.Setenv("REPLOG_SECRET_KEY", "dev-only-test-key-not-for-prod!")
}

// --- ListSettings ---

func TestListSettings_Admin(t *testing.T) {
	env := setupTest(t)
	admin := env.createUser(t, "admin", true, true)
	cookies := env.loginAs(t, admin)

	rr := env.do(t, "GET", "/api/admin/settings", nil, cookies)
	requireStatus(t, rr, http.StatusOK)

	var got []SettingCategoryResponse
	decodeJSON(t, rr, &got)
	if len(got) == 0 {
		t.Fatal("expected at least one settings category")
	}

	// Should include the canonical categories from the registry.
	wantCategories := map[string]bool{}
	for _, c := range models.CategoryOrder {
		wantCategories[c] = true
	}
	for _, c := range got {
		delete(wantCategories, c.Category)
	}
	if len(wantCategories) > 0 {
		t.Errorf("missing categories in response: %v", wantCategories)
	}
}

func TestListSettings_NonAdminForbidden(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "GET", "/api/admin/settings", nil, cookies)
	requireStatus(t, rr, http.StatusForbidden)
}

func TestListSettings_Unauthenticated(t *testing.T) {
	env := setupTest(t)
	rr := env.do(t, "GET", "/api/admin/settings", nil, nil)
	requireStatus(t, rr, http.StatusUnauthorized)
}

func TestListSettings_SensitiveValuesMasked(t *testing.T) {
	// Sensitive settings (e.g. llm.api_key) must not return their plaintext
	// value through the API even after they've been set.
	env := setupTest(t)
	withSecretKey(t)
	admin := env.createUser(t, "admin", true, true)
	cookies := env.loginAs(t, admin)

	// Persist a sensitive value via the model layer to skip the round-trip.
	if err := models.SetSetting(env.DB, "llm.api_key", "sk-super-secret-12345"); err != nil {
		t.Fatalf("set sensitive setting: %v", err)
	}

	rr := env.do(t, "GET", "/api/admin/settings", nil, cookies)
	requireStatus(t, rr, http.StatusOK)

	body := rr.Body.String()
	if strings.Contains(body, "sk-super-secret-12345") {
		t.Errorf("response leaked sensitive value:\n%s", body)
	}
}

// --- UpdateSetting ---

func TestUpdateSetting_Admin(t *testing.T) {
	env := setupTest(t)
	admin := env.createUser(t, "admin", true, true)
	cookies := env.loginAs(t, admin)

	rr := env.do(t, "PUT", "/api/admin/settings",
		`{"key":"app.name","value":"My Custom Gym"}`, cookies)
	requireStatus(t, rr, http.StatusOK)

	// Confirm the model now returns the new value.
	if got := models.GetSetting(env.DB, "app.name"); got != "My Custom Gym" {
		t.Errorf("got app.name=%q, want %q", got, "My Custom Gym")
	}
}

func TestUpdateSetting_NonAdminForbidden(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "PUT", "/api/admin/settings",
		`{"key":"app.name","value":"hijacked"}`, cookies)
	requireStatus(t, rr, http.StatusForbidden)
}

func TestUpdateSetting_RequiresKey(t *testing.T) {
	env := setupTest(t)
	admin := env.createUser(t, "admin", true, true)
	cookies := env.loginAs(t, admin)

	rr := env.do(t, "PUT", "/api/admin/settings",
		`{"key":"","value":"x"}`, cookies)
	requireStatus(t, rr, http.StatusBadRequest)
}

func TestUpdateSetting_UnknownKeyRejected(t *testing.T) {
	env := setupTest(t)
	admin := env.createUser(t, "admin", true, true)
	cookies := env.loginAs(t, admin)

	rr := env.do(t, "PUT", "/api/admin/settings",
		`{"key":"not.a.real.setting","value":"x"}`, cookies)
	if rr.Code != http.StatusInternalServerError && rr.Code != http.StatusBadRequest {
		t.Errorf("expected 4xx/5xx for unknown key, got %d", rr.Code)
	}
}

func TestUpdateSetting_SensitiveValueRoundTrip(t *testing.T) {
	// PUT a sensitive value through the API, then verify the model can read
	// it back decrypted. Confirms encryption-on-write + decryption-on-read.
	env := setupTest(t)
	withSecretKey(t)
	admin := env.createUser(t, "admin", true, true)
	cookies := env.loginAs(t, admin)

	rr := env.do(t, "PUT", "/api/admin/settings",
		`{"key":"llm.api_key","value":"sk-roundtrip-test"}`, cookies)
	requireStatus(t, rr, http.StatusOK)

	if got := models.GetSetting(env.DB, "llm.api_key"); got != "sk-roundtrip-test" {
		t.Errorf("got llm.api_key=%q, want %q", got, "sk-roundtrip-test")
	}

	// Raw DB value should be encrypted (prefixed with "enc:") and not contain
	// the plaintext.
	var raw string
	if err := env.DB.QueryRow(`SELECT value FROM app_settings WHERE key = ?`, "llm.api_key").Scan(&raw); err != nil {
		t.Fatalf("query raw setting: %v", err)
	}
	if !strings.HasPrefix(raw, "enc:") {
		t.Errorf("expected raw value to be encrypted (enc: prefix), got %q", raw)
	}
	if strings.Contains(raw, "sk-roundtrip-test") {
		t.Errorf("raw stored value contains plaintext: %q", raw)
	}
}

// --- Test-connection endpoints ---

func TestTestLLMConnection_NonAdminForbidden(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "POST", "/api/admin/settings/test-llm", nil, cookies)
	requireStatus(t, rr, http.StatusForbidden)
}

func TestTestLLMConnection_AdminWithoutProvider(t *testing.T) {
	// No LLM provider configured \u2014 the endpoint returns 200 with
	// success:false and an error message in the body. (This is the
	// expected SPA contract: the UI shows the error inline rather than
	// having to interpret 4xx.)
	env := setupTest(t)
	admin := env.createUser(t, "admin", true, true)
	cookies := env.loginAs(t, admin)

	rr := env.do(t, "POST", "/api/admin/settings/test-llm", nil, cookies)
	requireStatus(t, rr, http.StatusOK)

	var got map[string]any
	decodeJSON(t, rr, &got)
	if got["success"] != false {
		t.Errorf("expected success=false with no provider, got %v", got["success"])
	}
}

func TestTestNotifyConnection_NonAdminForbidden(t *testing.T) {
	env := setupTest(t)
	coach := env.createUser(t, "coach", true, false)
	cookies := env.loginAs(t, coach)

	rr := env.do(t, "POST", "/api/admin/settings/test-notify", nil, cookies)
	requireStatus(t, rr, http.StatusForbidden)
}

func TestTestNotifyConnection_AdminWithoutConfig(t *testing.T) {
	env := setupTest(t)
	admin := env.createUser(t, "admin", true, true)
	cookies := env.loginAs(t, admin)

	rr := env.do(t, "POST", "/api/admin/settings/test-notify", nil, cookies)
	requireStatus(t, rr, http.StatusOK)

	var got map[string]any
	decodeJSON(t, rr, &got)
	// Without notify config, success should be false.
	if got["success"] != false {
		t.Errorf("expected success=false with no notify config, got %v", got["success"])
	}
}

// TestTestLLMConnection_TimesOutWhenProviderHangs verifies the handler
// caps Ping at testLLMPingTimeout so a hung provider can never sit on
// the connection past the HTTP server's WriteTimeout (60s) — the bug
// that caused our /generate Caddy 502s before ADR 015.
func TestTestLLMConnection_TimesOutWhenProviderHangs(t *testing.T) {
	orig := testLLMPingTimeout
	testLLMPingTimeout = 50 * time.Millisecond
	t.Cleanup(func() { testLLMPingTimeout = orig })

	env := setupTest(t)
	if err := models.SetSetting(env.DB, "llm.provider", "anthropic"); err != nil {
		t.Fatalf("set llm.provider: %v", err)
	}
	// Mock provider that respects context cancellation — simulates a hung
	// upstream by blocking for 5s, which is well above the 50ms cap.
	env.Handlers.LLMProviderFactory = mockLLMFactory(&llm.MockProvider{PingDelay: 5 * time.Second})

	admin := env.createUser(t, "admin", true, true)
	cookies := env.loginAs(t, admin)

	start := time.Now()
	rr := env.do(t, "POST", "/api/admin/settings/test-llm", nil, cookies)
	elapsed := time.Since(start)

	requireStatus(t, rr, http.StatusOK)
	if elapsed > 2*time.Second {
		t.Errorf("handler took %v — timeout cap is not being honored", elapsed)
	}

	var got map[string]any
	decodeJSON(t, rr, &got)
	if got["success"] != false {
		t.Errorf("expected success=false on timeout, got %v", got["success"])
	}
	errMsg, _ := got["error"].(string)
	if !strings.Contains(strings.ToLower(errMsg), "respond") {
		t.Errorf("expected friendly timeout message, got %q", errMsg)
	}
}
