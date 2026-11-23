package push

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/carpenike/replog/internal/database"
	"github.com/carpenike/replog/internal/models"
	"github.com/carpenike/replog/internal/webpush"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// fakeService is a push service endpoint that records hits and answers
// with status.
type fakeService struct {
	srv    *httptest.Server
	hits   atomic.Int32
	status atomic.Int32
}

func newFakeService(t *testing.T) *fakeService {
	f := &fakeService{}
	f.status.Store(http.StatusCreated)
	f.srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.hits.Add(1)
		w.WriteHeader(int(f.status.Load()))
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func browserKeys(t *testing.T) (p256dh, auth string) {
	t.Helper()
	k, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	a := make([]byte, 16)
	_, _ = rand.Read(a)
	return base64.RawURLEncoding.EncodeToString(k.PublicKey().Bytes()), base64.RawURLEncoding.EncodeToString(a)
}

func newUser(t *testing.T, db *sql.DB, name string) *models.User {
	t.Helper()
	u, err := models.CreateUser(context.Background(), db, name, "", "", "", false, false, sql.NullInt64{})
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestNewPersistsGeneratedKeys(t *testing.T) {
	db := testDB(t)
	a, err := New(context.Background(), db, Config{})
	if err != nil {
		t.Fatal(err)
	}
	b, err := New(context.Background(), db, Config{})
	if err != nil {
		t.Fatal(err)
	}
	if a.PublicKey() == "" || a.PublicKey() != b.PublicKey() {
		t.Fatalf("keys not persisted: %q vs %q", a.PublicKey(), b.PublicKey())
	}

	pub, priv, err := webpush.GenerateKeys()
	if err != nil {
		t.Fatal(err)
	}
	c, err := New(context.Background(), db, Config{PublicKey: pub, PrivateKey: priv})
	if err != nil {
		t.Fatal(err)
	}
	if c.PublicKey() != pub {
		t.Fatal("env keys should win over the stored pair")
	}
	if _, err := New(context.Background(), db, Config{PublicKey: pub}); err == nil {
		t.Fatal("half a key pair should fail")
	}
	if _, err := New(context.Background(), db, Config{Subject: "coach@example.test"}); err == nil {
		t.Fatal("a subject without mailto:/https: should fail")
	}
}

func TestNewEncryptsStoredPrivateKey(t *testing.T) {
	t.Setenv("REPLOG_SECRET_KEY", "test-secret")
	db := testDB(t)
	s, err := New(context.Background(), db, Config{})
	if err != nil {
		t.Fatal(err)
	}
	var stored string
	if err := db.QueryRow(`SELECT private_key FROM vapid_keys`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if len(stored) < 4 || stored[:4] != "enc:" {
		t.Fatalf("private key stored in the clear: %q", stored)
	}
	again, err := New(context.Background(), db, Config{})
	if err != nil || again.PublicKey() != s.PublicKey() {
		t.Fatalf("reload with secret: %v", err)
	}
	t.Setenv("REPLOG_SECRET_KEY", "")
	if _, err := New(context.Background(), db, Config{}); err == nil {
		t.Fatal("an encrypted key without the secret should fail to load")
	}
}

func TestValidateEndpoint(t *testing.T) {
	s, err := New(context.Background(), testDB(t), Config{ExtraAllowedHosts: []string{"push.example.org"}})
	if err != nil {
		t.Fatal(err)
	}
	ok := []string{
		"https://fcm.googleapis.com/fcm/send/abc",
		"https://updates.push.services.mozilla.com/wpush/v2/abc",
		"https://web.push.apple.com/QH0",
		"https://wns2-bl2p.notify.windows.com/w/?token=x",
		"https://push.example.org/x",
	}
	for _, u := range ok {
		if err := s.ValidateEndpoint(u); err != nil {
			t.Errorf("%s: %v", u, err)
		}
	}
	bad := []string{
		"http://fcm.googleapis.com/fcm/send/abc",
		"https://127.0.0.1/push",
		"https://evilfcm.googleapis.com.attacker.test/x",
		"https://user:pw@fcm.googleapis.com/x",
		"not a url",
	}
	for _, u := range bad {
		if err := s.ValidateEndpoint(u); err == nil {
			t.Errorf("%s should be rejected", u)
		}
	}
}

func TestSendToUserDropsGoneSubscriptions(t *testing.T) {
	db := testDB(t)
	fake := newFakeService(t)
	s, err := New(context.Background(), db, Config{AllowAnyHTTPSHost: true, Client: fake.srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	u := newUser(t, db, "ryan")
	p, a := browserKeys(t)
	if _, err := models.UpsertPushSubscription(context.Background(), db, u.ID, fake.srv.URL+"/1", p, a, "test"); err != nil {
		t.Fatal(err)
	}
	n, err := s.SendToUser(context.Background(), u.ID, Payload{Kind: "test", Title: "Hi"}, 30, "normal")
	if err != nil || n != 1 {
		t.Fatalf("sent %d, err %v", n, err)
	}
	fake.status.Store(http.StatusGone)
	if n, _ := s.SendToUser(context.Background(), u.ID, Payload{Kind: "test", Title: "Hi"}, 30, "normal"); n != 0 {
		t.Fatalf("sent %d to a gone subscription", n)
	}
	subs, _ := models.ListPushSubscriptions(context.Background(), db, u.ID)
	if len(subs) != 0 {
		t.Fatalf("gone subscription kept: %d", len(subs))
	}
}

// manualTimer records the callback so the test decides when it fires.
type manualTimer struct {
	mu      sync.Mutex
	fn      func()
	stopped bool
}

func (m *manualTimer) Stop() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	was := !m.stopped
	m.stopped = true
	return was
}

func TestRestAlertReplaceCancelAndFire(t *testing.T) {
	db := testDB(t)
	fake := newFakeService(t)
	s, err := New(context.Background(), db, Config{AllowAnyHTTPSHost: true, Client: fake.srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	var timers []*manualTimer
	s.afterFunc = func(_ time.Duration, f func()) stopper {
		mt := &manualTimer{fn: f}
		timers = append(timers, mt)
		return mt
	}
	u := newUser(t, db, "kid")
	p, a := browserKeys(t)
	if _, err := models.UpsertPushSubscription(context.Background(), db, u.ID, fake.srv.URL+"/1", p, a, ""); err != nil {
		t.Fatal(err)
	}

	if _, err := s.ScheduleRestAlert(u.ID, 0, Payload{}); err == nil {
		t.Fatal("zero duration should fail")
	}
	if _, err := s.ScheduleRestAlert(u.ID, 2*time.Hour, Payload{}); err == nil {
		t.Fatal("over an hour should fail")
	}
	if _, err := s.ScheduleRestAlert(u.ID, 90*time.Second, Payload{Kind: "rest", Title: "Rest done"}); err != nil {
		t.Fatal(err)
	}
	// +30s replaces the first alert.
	firesAt, err := s.ScheduleRestAlert(u.ID, 120*time.Second, Payload{Kind: "rest", Title: "Rest done"})
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := s.PendingRestAlert(u.ID); !ok || !got.Equal(firesAt) {
		t.Fatalf("pending = %v %v", got, ok)
	}
	if !timers[0].stopped {
		t.Fatal("replaced timer was not stopped")
	}
	// The stale callback must not send even if its goroutine already ran.
	timers[0].fn()
	if fake.hits.Load() != 0 {
		t.Fatal("replaced alert fired")
	}
	timers[1].fn()
	if fake.hits.Load() != 1 {
		t.Fatalf("hits = %d, want 1", fake.hits.Load())
	}
	if _, ok := s.PendingRestAlert(u.ID); ok {
		t.Fatal("fired alert still pending")
	}

	if _, err := s.ScheduleRestAlert(u.ID, time.Minute, Payload{Kind: "rest"}); err != nil {
		t.Fatal(err)
	}
	if !s.CancelRestAlert(u.ID) || s.CancelRestAlert(u.ID) {
		t.Fatal("cancel should report pending once")
	}
	timers[2].fn()
	if fake.hits.Load() != 1 {
		t.Fatal("cancelled alert fired")
	}
	s.Close()
	if _, err := s.ScheduleRestAlert(u.ID, time.Minute, Payload{}); err == nil {
		t.Fatal("schedule after Close should fail")
	}
}
