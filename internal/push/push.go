// Package push delivers Web Push notifications to a user's subscribed
// browsers and schedules the rest-timer alert that fires when the phone is
// locked (gap 4).
//
// Rest alerts are process-local timers: a rest period is minutes long, so a
// restart drops at most the alert in flight and the in-page timer still
// runs. Each user has at most one pending alert; scheduling again replaces
// it (that is how "+30s" works), and cancelling removes it (Skip).
package push

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/carpenike/replog/internal/models"
	"github.com/carpenike/replog/internal/webpush"
)

// DefaultAllowedHosts are the push services browsers hand out endpoints
// for. Endpoints must be on one of these (or a subdomain) so a stored
// subscription cannot point the server at an internal address.
var DefaultAllowedHosts = []string{
	"fcm.googleapis.com",        // Chrome, Edge on Android, Samsung Internet
	"android.googleapis.com",    // legacy Chrome endpoints
	"push.services.mozilla.com", // Firefox
	"push.apple.com",            // Safari and iOS/iPadOS home-screen apps
	"notify.windows.com",        // Edge on Windows (WNS)
}

// MaxRestAlert bounds how far ahead a rest alert can be scheduled.
const MaxRestAlert = time.Hour

// sendTimeout bounds one fan-out to a user's devices.
const sendTimeout = 15 * time.Second

// Config is the push setup resolved from the environment.
type Config struct {
	// PublicKey and PrivateKey come from REPLOG_VAPID_PUBLIC_KEY and
	// REPLOG_VAPID_PRIVATE_KEY. Both empty means use (or generate and
	// persist) the pair in the database.
	PublicKey  string
	PrivateKey string
	// Subject is the VAPID contact (REPLOG_VAPID_SUBJECT): mailto: or https:.
	Subject string
	// ExtraAllowedHosts adds push-service hosts (REPLOG_PUSH_ALLOWED_HOSTS).
	ExtraAllowedHosts []string
	// AllowAnyHTTPSHost skips the host allow-list. Tests only.
	AllowAnyHTTPSHost bool
	// Client sends the pushes. Nil uses a client with a 10s timeout.
	Client *http.Client
}

// Payload is the JSON the service worker receives.
type Payload struct {
	Kind  string `json:"kind"` // "rest", "test"
	Title string `json:"title"`
	Body  string `json:"body,omitempty"`
	Tag   string `json:"tag,omitempty"`
	URL   string `json:"url,omitempty"`
	// SentAt lets the worker drop an alert that arrives long after its
	// moment (a phone that was offline when it fired).
	SentAt int64 `json:"sent_at"`
}

// Service sends pushes and owns the pending rest alerts.
type Service struct {
	db      *sql.DB
	keys    webpush.Keys
	client  *http.Client
	hosts   []string
	anyHost bool

	mu     sync.Mutex
	alerts map[int64]*restAlert
	seq    uint64
	closed bool
	wg     sync.WaitGroup

	// afterFunc is time.AfterFunc; tests swap it to fire immediately.
	afterFunc func(time.Duration, func()) stopper
}

type stopper interface{ Stop() bool }

type restAlert struct {
	id      uint64
	firesAt time.Time
	timer   stopper
}

// New resolves the VAPID keys and returns a ready Service. With no keys
// in cfg it loads the persisted pair, generating and storing one on first
// start so browser subscriptions survive restarts.
func New(ctx context.Context, db *sql.DB, cfg Config) (*Service, error) {
	keys := webpush.Keys{Public: cfg.PublicKey, Private: cfg.PrivateKey, Subject: cfg.Subject}
	switch {
	case keys.Public != "" && keys.Private != "":
	case keys.Public != "" || keys.Private != "":
		return nil, errors.New("push: set both REPLOG_VAPID_PUBLIC_KEY and REPLOG_VAPID_PRIVATE_KEY, or neither")
	default:
		pub, priv, created, err := models.LoadOrCreateVAPIDKeys(ctx, db, webpush.GenerateKeys)
		if err != nil {
			return nil, err
		}
		if created {
			log.Printf("push: generated a VAPID key pair and stored it in the database")
		}
		keys.Public, keys.Private = pub, priv
	}
	if _, err := webpush.ParseKeys(keys); err != nil {
		return nil, fmt.Errorf("push: VAPID keys: %w", err)
	}
	if keys.Subject == "" {
		keys.Subject = "https://github.com/carpenike/replog"
	}
	if !strings.HasPrefix(keys.Subject, "mailto:") && !strings.HasPrefix(keys.Subject, "https://") {
		return nil, fmt.Errorf("push: VAPID subject %q must start with mailto: or https://", keys.Subject)
	}
	client := cfg.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	hosts := append(append([]string{}, DefaultAllowedHosts...), cfg.ExtraAllowedHosts...)
	return &Service{
		db:      db,
		keys:    keys,
		client:  client,
		hosts:   hosts,
		anyHost: cfg.AllowAnyHTTPSHost,
		alerts:  map[int64]*restAlert{},
		afterFunc: func(d time.Duration, f func()) stopper {
			return time.AfterFunc(d, f)
		},
	}, nil
}

// PublicKey is the applicationServerKey the browser subscribes with.
func (s *Service) PublicKey() string { return s.keys.Public }

// ValidateEndpoint checks a subscription endpoint before it is stored.
func (s *Service) ValidateEndpoint(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return errors.New("endpoint must be an https URL")
	}
	if s.anyHost {
		return nil
	}
	host := strings.ToLower(u.Hostname())
	for _, allowed := range s.hosts {
		allowed = strings.ToLower(strings.TrimSpace(allowed))
		if allowed != "" && (host == allowed || strings.HasSuffix(host, "."+allowed)) {
			return nil
		}
	}
	return fmt.Errorf("endpoint host %q is not a known push service", host)
}

// SendToUser pushes p to every device userID subscribed. Subscriptions the
// push service reports as gone are deleted. It returns how many devices
// accepted the message.
func (s *Service) SendToUser(ctx context.Context, userID int64, p Payload, ttl int, urgency string) (int, error) {
	subs, err := models.ListPushSubscriptions(ctx, s.db, userID)
	if err != nil {
		return 0, err
	}
	if p.SentAt == 0 {
		p.SentAt = time.Now().Unix()
	}
	body, err := json.Marshal(p)
	if err != nil {
		return 0, fmt.Errorf("push: payload: %w", err)
	}
	msg := webpush.Message{Payload: body, TTL: ttl, Urgency: urgency, Topic: topic(p.Tag)}
	sent := 0
	var errs []string
	for _, sub := range subs {
		err := webpush.Send(ctx, s.client, webpush.Subscription{Endpoint: sub.Endpoint, P256dh: sub.P256dh, Auth: sub.Auth}, s.keys, msg)
		switch {
		case err == nil:
			sent++
			_ = models.MarkPushSubscriptionSuccess(ctx, s.db, sub.ID)
		case errors.Is(err, webpush.ErrGone):
			log.Printf("push: removing expired subscription %d for user %d", sub.ID, userID)
			_ = models.DeletePushSubscriptionByID(ctx, s.db, sub.ID)
		default:
			errs = append(errs, err.Error())
		}
	}
	if sent == 0 && len(errs) > 0 {
		return 0, fmt.Errorf("push: %s", strings.Join(errs, "; "))
	}
	return sent, nil
}

// topic turns a tag into a valid Topic header (base64url alphabet, ≤32).
func topic(tag string) string {
	var b strings.Builder
	for _, r := range tag {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		}
		if b.Len() == 32 {
			break
		}
	}
	return b.String()
}

// ScheduleRestAlert replaces userID's pending rest alert with one that
// fires after d. It returns when the alert will fire.
func (s *Service) ScheduleRestAlert(userID int64, d time.Duration, p Payload) (time.Time, error) {
	if d <= 0 || d > MaxRestAlert {
		return time.Time{}, fmt.Errorf("push: rest alert must be within %s", MaxRestAlert)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return time.Time{}, errors.New("push: service is shut down")
	}
	if prev := s.alerts[userID]; prev != nil {
		prev.timer.Stop()
	}
	s.seq++
	alert := &restAlert{id: s.seq, firesAt: time.Now().Add(d)}
	alert.timer = s.afterFunc(d, func() { s.fireRestAlert(userID, alert.id, p) })
	s.alerts[userID] = alert
	return alert.firesAt, nil
}

// CancelRestAlert drops userID's pending rest alert. It reports whether
// one was pending.
func (s *Service) CancelRestAlert(userID int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	alert := s.alerts[userID]
	if alert == nil {
		return false
	}
	alert.timer.Stop()
	delete(s.alerts, userID)
	return true
}

// PendingRestAlert returns when userID's rest alert fires, if one is set.
func (s *Service) PendingRestAlert(userID int64) (time.Time, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if alert := s.alerts[userID]; alert != nil {
		return alert.firesAt, true
	}
	return time.Time{}, false
}

func (s *Service) fireRestAlert(userID int64, id uint64, p Payload) {
	s.mu.Lock()
	alert := s.alerts[userID]
	// A replaced or cancelled alert whose timer had already started.
	if alert == nil || alert.id != id || s.closed {
		s.mu.Unlock()
		return
	}
	delete(s.alerts, userID)
	s.wg.Add(1)
	s.mu.Unlock()
	defer s.wg.Done()

	ctx, cancel := context.WithTimeout(context.Background(), sendTimeout)
	defer cancel()
	// TTL 60: a rest alert that cannot be delivered within a minute is
	// stale, so the push service may drop it.
	if _, err := s.SendToUser(ctx, userID, p, 60, "high"); err != nil {
		log.Printf("push: rest alert for user %d: %v", userID, err)
	}
}

// Close stops pending alerts and waits for in-flight sends.
func (s *Service) Close() {
	s.mu.Lock()
	s.closed = true
	for id, alert := range s.alerts {
		alert.timer.Stop()
		delete(s.alerts, id)
	}
	s.mu.Unlock()
	s.wg.Wait()
}
