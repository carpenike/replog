package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
	"github.com/carpenike/replog/internal/push"
)

// pushLabelMaxRunes caps the exercise label carried in a rest alert.
const pushLabelMaxRunes = 80

// PushConfigResponse tells the SPA whether Web Push is on and which
// application-server key to subscribe with.
type PushConfigResponse struct {
	// Enabled is false when the server has no push service configured.
	Enabled bool `json:"enabled"`
	// PublicKey is the VAPID public key (base64url) for
	// PushManager.subscribe's applicationServerKey. Empty when disabled.
	PublicKey string `json:"public_key"`
	// Subscriptions is how many devices this user has subscribed.
	Subscriptions int `json:"subscriptions"`
}

// PushSubscriptionKeys is PushSubscription.toJSON().keys.
type PushSubscriptionKeys struct {
	P256dh string `json:"p256dh"`
	Auth   string `json:"auth"`
}

// PushSubscriptionRequest is the body for POST /api/push/subscriptions:
// the browser's PushSubscription.toJSON().
type PushSubscriptionRequest struct {
	Endpoint string               `json:"endpoint"`
	Keys     PushSubscriptionKeys `json:"keys"`
}

// PushUnsubscribeRequest is the body for DELETE /api/push/subscriptions.
type PushUnsubscribeRequest struct {
	Endpoint string `json:"endpoint"`
}

// PushTestResponse reports how many devices accepted a test push.
type PushTestResponse struct {
	Sent int `json:"sent"`
}

// RestAlertRequest is the body for POST /api/push/rest-alert.
type RestAlertRequest struct {
	// Seconds until the rest period ends (1–3600).
	Seconds int `json:"seconds" example:"90"`
	// Label names the next exercise in the notification (optional).
	Label string `json:"label,omitempty" example:"Back Squat"`
	// URL is the in-app path a tap on the notification opens (optional,
	// must start with "/").
	URL string `json:"url,omitempty" example:"/athletes/1/workouts/2/active"`
}

// RestAlertResponse says whether an alert is pending and when it fires.
// scheduled is false when this user has no subscribed device.
type RestAlertResponse struct {
	Scheduled bool    `json:"scheduled"`
	FiresAt   *string `json:"fires_at"`
}

// pushUser returns the signed-in browser user, refusing service tokens:
// push subscriptions belong to a browser session.
func (h *Handlers) pushUser(w http.ResponseWriter, r *http.Request, needService bool) *models.User {
	if middleware.IsServiceCredential(r.Context()) {
		WriteError(w, http.StatusForbidden, "push notifications need a browser session")
		return nil
	}
	if needService && h.Push == nil {
		WriteError(w, http.StatusServiceUnavailable, "push notifications are not configured on this server")
		return nil
	}
	return middleware.UserFromContext(r.Context())
}

// GetPushConfig returns the Web Push setup for the Preferences opt-in.
//
//	@Summary      Web Push configuration
//	@Description  enabled is false when the server has no push service, so the SPA hides the opt-in. public_key is the VAPID applicationServerKey.
//	@Tags         Notifications
//	@Produce      json
//	@Success      200  {object}  api.PushConfigResponse
//	@Failure      403  {object}  api.APIError
//	@Router       /push/config [get]
func (h *Handlers) GetPushConfig(w http.ResponseWriter, r *http.Request) {
	user := h.pushUser(w, r, false)
	if user == nil {
		return
	}
	resp := PushConfigResponse{}
	if h.Push != nil {
		resp.Enabled = true
		resp.PublicKey = h.Push.PublicKey()
		subs, err := models.ListPushSubscriptions(r.Context(), h.DB, user.ID)
		if err != nil {
			log.Printf("api: list push subscriptions for user %d: %v", user.ID, err)
			WriteError(w, http.StatusInternalServerError, "failed to read push subscriptions")
			return
		}
		resp.Subscriptions = len(subs)
	}
	WriteJSON(w, http.StatusOK, resp)
}

// SubscribePush stores this browser's push subscription.
//
//	@Summary      Subscribe this device to push
//	@Description  Stores PushSubscription.toJSON(). The endpoint must be https on a known push service. Re-subscribing the same endpoint refreshes its keys.
//	@Tags         Notifications
//	@Accept       json
//	@Produce      json
//	@Param        body  body      api.PushSubscriptionRequest  true  "Subscription"
//	@Success      201  {object}  api.StatusResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      503  {object}  api.APIError
//	@Router       /push/subscriptions [post]
func (h *Handlers) SubscribePush(w http.ResponseWriter, r *http.Request) {
	user := h.pushUser(w, r, true)
	if user == nil {
		return
	}
	var req PushSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Endpoint = strings.TrimSpace(req.Endpoint)
	if len(req.Endpoint) > 2048 {
		WriteValidationError(w, "endpoint", "is too long")
		return
	}
	if err := h.Push.ValidateEndpoint(req.Endpoint); err != nil {
		WriteValidationError(w, "endpoint", err.Error())
		return
	}
	if req.Keys.P256dh == "" || req.Keys.Auth == "" || len(req.Keys.P256dh) > 256 || len(req.Keys.Auth) > 64 {
		WriteValidationError(w, "keys", "p256dh and auth are required")
		return
	}
	ua := r.UserAgent()
	if len(ua) > 256 {
		ua = ua[:256]
	}
	if _, err := models.UpsertPushSubscription(r.Context(), h.DB, user.ID, req.Endpoint, req.Keys.P256dh, req.Keys.Auth, ua); err != nil {
		log.Printf("api: subscribe push for user %d: %v", user.ID, err)
		WriteDBError(w, err, "failed to save push subscription")
		return
	}
	WriteJSON(w, http.StatusCreated, StatusResponse{Status: "ok"})
}

// UnsubscribePush removes this browser's push subscription.
//
//	@Summary      Unsubscribe this device from push
//	@Tags         Notifications
//	@Accept       json
//	@Produce      json
//	@Param        body  body      api.PushUnsubscribeRequest  true  "Endpoint"
//	@Success      200  {object}  api.StatusResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /push/subscriptions [delete]
func (h *Handlers) UnsubscribePush(w http.ResponseWriter, r *http.Request) {
	user := h.pushUser(w, r, false)
	if user == nil {
		return
	}
	var req PushUnsubscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Endpoint) == "" {
		WriteValidationError(w, "endpoint", "is required")
		return
	}
	err := models.DeletePushSubscription(r.Context(), h.DB, user.ID, strings.TrimSpace(req.Endpoint))
	if errors.Is(err, models.ErrNotFound) {
		WriteError(w, http.StatusNotFound, "subscription not found")
		return
	}
	if err != nil {
		log.Printf("api: unsubscribe push for user %d: %v", user.ID, err)
		WriteError(w, http.StatusInternalServerError, "failed to remove push subscription")
		return
	}
	WriteJSON(w, http.StatusOK, StatusResponse{Status: "ok"})
}

// TestPush sends a test notification to this user's devices.
//
//	@Summary      Send a test push
//	@Tags         Notifications
//	@Produce      json
//	@Success      200  {object}  api.PushTestResponse
//	@Failure      403  {object}  api.APIError
//	@Failure      502  {object}  api.APIError
//	@Failure      503  {object}  api.APIError
//	@Router       /push/test [post]
func (h *Handlers) TestPush(w http.ResponseWriter, r *http.Request) {
	user := h.pushUser(w, r, true)
	if user == nil {
		return
	}
	sent, err := h.Push.SendToUser(r.Context(), user.ID, push.Payload{
		Kind:  "test",
		Title: "Rest alerts are on",
		Body:  "This device will buzz when your rest is up.",
		Tag:   "replog-test",
		URL:   "/preferences",
	}, 60, "normal")
	if err != nil {
		log.Printf("api: test push for user %d: %v", user.ID, err)
		WriteError(w, http.StatusBadGateway, "the push service did not accept the message")
		return
	}
	WriteJSON(w, http.StatusOK, PushTestResponse{Sent: sent})
}

// ScheduleRestAlert sets the push that fires when the rest timer ends.
//
//	@Summary      Schedule the rest-timer push
//	@Description  Replaces this user's pending rest alert with one that fires in seconds (1–3600), so a locked phone still buzzes. Call again to extend; DELETE to skip. Alerts are held in memory and a restart drops them.
//	@Tags         Notifications
//	@Accept       json
//	@Produce      json
//	@Param        body  body      api.RestAlertRequest  true  "Rest alert"
//	@Success      200  {object}  api.RestAlertResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      503  {object}  api.APIError
//	@Router       /push/rest-alert [post]
func (h *Handlers) ScheduleRestAlert(w http.ResponseWriter, r *http.Request) {
	user := h.pushUser(w, r, true)
	if user == nil {
		return
	}
	var req RestAlertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Seconds < 1 || req.Seconds > int(push.MaxRestAlert/time.Second) {
		WriteValidationError(w, "seconds", "must be between 1 and 3600")
		return
	}
	label := strings.TrimSpace(req.Label)
	if utf8.RuneCountInString(label) > pushLabelMaxRunes {
		WriteValidationError(w, "label", "must be 80 characters or fewer")
		return
	}
	link := req.URL
	if link != "" && (!strings.HasPrefix(link, "/") || strings.HasPrefix(link, "//") || len(link) > 512) {
		WriteValidationError(w, "url", "must be an in-app path starting with /")
		return
	}
	subs, err := models.ListPushSubscriptions(r.Context(), h.DB, user.ID)
	if err != nil {
		log.Printf("api: rest alert subscriptions for user %d: %v", user.ID, err)
		WriteError(w, http.StatusInternalServerError, "failed to schedule rest alert")
		return
	}
	if len(subs) == 0 {
		h.Push.CancelRestAlert(user.ID)
		WriteJSON(w, http.StatusOK, RestAlertResponse{Scheduled: false})
		return
	}
	body := "Next set is up."
	if label != "" {
		body = "Next up: " + label + "."
	}
	firesAt, err := h.Push.ScheduleRestAlert(user.ID, time.Duration(req.Seconds)*time.Second, push.Payload{
		Kind:  "rest",
		Title: "Rest is up",
		Body:  body,
		Tag:   "replog-rest",
		URL:   link,
	})
	if err != nil {
		WriteError(w, http.StatusServiceUnavailable, "push notifications are not available")
		return
	}
	ts := fmtTime(firesAt)
	WriteJSON(w, http.StatusOK, RestAlertResponse{Scheduled: true, FiresAt: &ts})
}

// CancelRestAlert drops the pending rest-timer push (Skip).
//
//	@Summary      Cancel the rest-timer push
//	@Tags         Notifications
//	@Produce      json
//	@Success      200  {object}  api.RestAlertResponse
//	@Failure      403  {object}  api.APIError
//	@Router       /push/rest-alert [delete]
func (h *Handlers) CancelRestAlert(w http.ResponseWriter, r *http.Request) {
	user := h.pushUser(w, r, false)
	if user == nil {
		return
	}
	if h.Push != nil {
		h.Push.CancelRestAlert(user.ID)
	}
	WriteJSON(w, http.StatusOK, RestAlertResponse{Scheduled: false})
}
