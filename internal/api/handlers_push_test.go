package api

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/carpenike/replog/internal/models"
	"github.com/carpenike/replog/internal/push"
)

// enablePush gives the harness a push service whose pushes land on a local
// TLS server. It returns the endpoint base and a delivered-push counter.
func (e *testEnv) enablePush(t *testing.T) (string, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(srv.Close)
	svc, err := push.New(context.Background(), e.DB, push.Config{AllowAnyHTTPSHost: true, Client: srv.Client()})
	if err != nil {
		t.Fatalf("push service: %v", err)
	}
	t.Cleanup(svc.Close)
	e.Handlers.Push = svc
	return srv.URL, &hits
}

func browserSubscription(t *testing.T, endpoint string) PushSubscriptionRequest {
	t.Helper()
	k, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	auth := make([]byte, 16)
	_, _ = rand.Read(auth)
	return PushSubscriptionRequest{
		Endpoint: endpoint,
		Keys: PushSubscriptionKeys{
			P256dh: base64.RawURLEncoding.EncodeToString(k.PublicKey().Bytes()),
			Auth:   base64.RawURLEncoding.EncodeToString(auth),
		},
	}
}

func TestPush_DisabledFallsBackGracefully(t *testing.T) {
	env := setupTest(t)
	user := env.createUser(t, "kid", false, false)
	cookies := env.loginAs(t, user)

	rr := env.do(t, "GET", "/api/push/config", nil, cookies)
	requireStatus(t, rr, http.StatusOK)
	var cfg PushConfigResponse
	decodeJSON(t, rr, &cfg)
	if cfg.Enabled || cfg.PublicKey != "" {
		t.Fatalf("config with no push service = %+v", cfg)
	}
	requireStatus(t, env.do(t, "POST", "/api/push/subscriptions", browserSubscription(t, "https://fcm.googleapis.com/x"), cookies), http.StatusServiceUnavailable)
	requireStatus(t, env.do(t, "POST", "/api/push/rest-alert", `{"seconds":90}`, cookies), http.StatusServiceUnavailable)
	// Skip still answers, so the SPA never errors on the timer bar.
	requireStatus(t, env.do(t, "DELETE", "/api/push/rest-alert", nil, cookies), http.StatusOK)
}

func TestPush_SubscribeRestAlertAndUnsubscribe(t *testing.T) {
	env := setupTest(t)
	base, hits := env.enablePush(t)
	user := env.createUser(t, "ryan", false, false)
	other := env.createUser(t, "other", false, false)
	cookies := env.loginAs(t, user)

	rr := env.do(t, "GET", "/api/push/config", nil, cookies)
	requireStatus(t, rr, http.StatusOK)
	var cfg PushConfigResponse
	decodeJSON(t, rr, &cfg)
	if !cfg.Enabled || cfg.PublicKey != env.Handlers.Push.PublicKey() || cfg.Subscriptions != 0 {
		t.Fatalf("config = %+v", cfg)
	}

	// No device yet: the alert is not scheduled.
	rr = env.do(t, "POST", "/api/push/rest-alert", `{"seconds":90}`, cookies)
	requireStatus(t, rr, http.StatusOK)
	var alert RestAlertResponse
	decodeJSON(t, rr, &alert)
	if alert.Scheduled {
		t.Fatal("scheduled with no subscription")
	}

	sub := browserSubscription(t, base+"/push/ryan")
	requireStatus(t, env.do(t, "POST", "/api/push/subscriptions", sub, cookies), http.StatusCreated)
	// Re-subscribing the same endpoint updates in place.
	requireStatus(t, env.do(t, "POST", "/api/push/subscriptions", sub, cookies), http.StatusCreated)
	subs, _ := models.ListPushSubscriptions(context.Background(), env.DB, user.ID)
	if len(subs) != 1 {
		t.Fatalf("subscriptions = %d, want 1", len(subs))
	}

	rr = env.do(t, "POST", "/api/push/test", nil, cookies)
	requireStatus(t, rr, http.StatusOK)
	var tested PushTestResponse
	decodeJSON(t, rr, &tested)
	if tested.Sent != 1 || hits.Load() != 1 {
		t.Fatalf("test push sent %d, hits %d", tested.Sent, hits.Load())
	}

	rr = env.do(t, "POST", "/api/push/rest-alert", `{"seconds":120,"label":"Back Squat","url":"/athletes/1/workouts/2/active"}`, cookies)
	requireStatus(t, rr, http.StatusOK)
	decodeJSON(t, rr, &alert)
	if !alert.Scheduled || alert.FiresAt == nil {
		t.Fatalf("alert = %+v", alert)
	}
	if _, ok := env.Handlers.Push.PendingRestAlert(user.ID); !ok {
		t.Fatal("no pending alert after schedule")
	}
	requireStatus(t, env.do(t, "DELETE", "/api/push/rest-alert", nil, cookies), http.StatusOK)
	if _, ok := env.Handlers.Push.PendingRestAlert(user.ID); ok {
		t.Fatal("alert still pending after cancel")
	}

	// Another user cannot remove this device.
	otherCookies := env.loginAs(t, other)
	requireStatus(t, env.do(t, "DELETE", "/api/push/subscriptions", PushUnsubscribeRequest{Endpoint: sub.Endpoint}, otherCookies), http.StatusNotFound)
	requireStatus(t, env.do(t, "DELETE", "/api/push/subscriptions", PushUnsubscribeRequest{Endpoint: sub.Endpoint}, cookies), http.StatusOK)
	subs, _ = models.ListPushSubscriptions(context.Background(), env.DB, user.ID)
	if len(subs) != 0 {
		t.Fatalf("subscriptions after unsubscribe = %d", len(subs))
	}
}

func TestPush_Validation(t *testing.T) {
	env := setupTest(t)
	env.enablePush(t)
	// Use the production host allow-list for this test.
	svc, err := push.New(context.Background(), env.DB, push.Config{})
	if err != nil {
		t.Fatal(err)
	}
	env.Handlers.Push = svc
	user := env.createUser(t, "kid", false, false)
	cookies := env.loginAs(t, user)

	for _, endpoint := range []string{"http://fcm.googleapis.com/x", "https://10.0.0.5/push", "https://attacker.test/fcm.googleapis.com"} {
		rr := env.do(t, "POST", "/api/push/subscriptions", browserSubscription(t, endpoint), cookies)
		requireStatus(t, rr, http.StatusBadRequest)
	}
	missingKeys := PushSubscriptionRequest{Endpoint: "https://fcm.googleapis.com/fcm/send/abc"}
	requireStatus(t, env.do(t, "POST", "/api/push/subscriptions", missingKeys, cookies), http.StatusBadRequest)
	requireStatus(t, env.do(t, "POST", "/api/push/subscriptions", browserSubscription(t, "https://fcm.googleapis.com/fcm/send/abc"), cookies), http.StatusCreated)

	for _, body := range []string{
		`{"seconds":0}`,
		`{"seconds":3601}`,
		`{"seconds":60,"url":"https://evil.test/"}`,
		`{"seconds":60,"url":"//evil.test/"}`,
	} {
		requireStatus(t, env.do(t, "POST", "/api/push/rest-alert", body, cookies), http.StatusBadRequest)
	}
}

func TestPush_ServiceTokenRefused(t *testing.T) {
	env := setupTest(t)
	env.enablePush(t)
	coach := env.createUser(t, "coach", true, false)
	token := env.coachServiceToken(t, coach.ID)
	for _, tc := range []struct{ method, path, body string }{
		{"GET", "/api/push/config", ""},
		{"POST", "/api/push/subscriptions", `{"endpoint":"https://fcm.googleapis.com/x","keys":{"p256dh":"a","auth":"b"}}`},
		{"POST", "/api/push/rest-alert", `{"seconds":60}`},
		{"DELETE", "/api/push/rest-alert", ""},
	} {
		rr := env.serveBearer(t, tc.method, tc.path, token, tc.body, nil)
		if rr.Code != http.StatusForbidden {
			t.Errorf("%s %s = %d, want 403", tc.method, tc.path, rr.Code)
		}
	}
}
