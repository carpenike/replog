package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
	"github.com/carpenike/replog/internal/notify"
)

// --- Reviews (Coach Only) ---

// ListPendingReviews returns workouts that need coach review.
//
//	@Summary      List unreviewed workouts
//	@Tags         Reviews
//	@Produce      json
//	@Success      200  {array}   api.UnreviewedWorkout
//	@Failure      403  {object}  api.APIError
//	@Router       /reviews/pending [get]
func (h *Handlers) ListPendingReviews(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return
	}

	workouts, err := models.ListUnreviewedWorkouts(h.DB)
	if err != nil {
		log.Printf("api: list unreviewed workouts: %v", err)
		WriteError(w, http.StatusInternalServerError, "failed to list pending reviews")
		return
	}

	result := make([]*UnreviewedWorkout, len(workouts))
	for i, w := range workouts {
		result[i] = &UnreviewedWorkout{
			WorkoutID:   w.WorkoutID,
			AthleteID:   w.AthleteID,
			AthleteName: w.AthleteName,
			Date:        w.Date,
			SetCount:    w.SetCount,
			Notes:       nullStr(w.Notes),
		}
	}
	WriteJSON(w, http.StatusOK, result)
}

// --- Admin Settings ---

// SettingValueResponse is the JSON representation for a setting.
type SettingValueResponse struct {
	Key         string   `json:"key"`
	Value       string   `json:"value"`
	Source      string   `json:"source"`
	Masked      string   `json:"masked"`
	ReadOnly    bool     `json:"read_only"`
	FieldType   string   `json:"field_type"`
	Options     []string `json:"options,omitempty"`
	Description string   `json:"description"`
}

// SettingCategoryResponse groups settings by category.
type SettingCategoryResponse struct {
	Category string                 `json:"category"`
	Settings []SettingValueResponse `json:"settings"`
}

// ListSettings returns all application settings grouped by category.
//
//	@Summary      List app settings (admin)
//	@Description  Sensitive settings (LLM keys, SMTP password) return empty `value` and only a masked preview — plaintext is never sent over the wire.
//	@Tags         Admin
//	@Produce      json
//	@Success      200  {array}   api.SettingCategoryResponse
//	@Failure      403  {object}  api.APIError
//	@Router       /admin/settings [get]
func (h *Handlers) ListSettings(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "admin access required")
		return
	}

	categories := models.ListSettingsByCategory(h.DB)
	result := make([]SettingCategoryResponse, 0, len(categories))

	for _, catName := range models.CategoryOrder {
		settings, ok := categories[catName]
		if !ok {
			continue
		}
		cat := SettingCategoryResponse{Category: catName}
		for _, s := range settings {
			resp := SettingValueResponse{
				Key:      s.Key,
				Value:    s.Value,
				Source:   s.Source,
				Masked:   s.Masked,
				ReadOnly: s.ReadOnly,
			}
			if def := models.GetSettingDefinition(s.Key); def != nil {
				resp.FieldType = def.FieldType
				resp.Options = def.Options
				resp.Description = def.Description
				// Never return plaintext for sensitive settings — admins see only
				// the masked preview, and editing is write-only (clear-and-set).
				if def.Sensitive {
					resp.Value = ""
				}
			}
			cat.Settings = append(cat.Settings, resp)
		}
		result = append(result, cat)
	}

	WriteJSON(w, http.StatusOK, result)
}

// UpdateSetting updates a single application setting.
//
//	@Summary      Update app setting (admin)
//	@Description  Sensitive values (Sensitive=true in the registry) are encrypted with REPLOG_SECRET_KEY before storage.
//	@Tags         Admin
//	@Accept       json
//	@Produce      json
//	@Param        body  body      api.SettingUpdateRequest  true  "Setting"
//	@Success      200  {object}  api.StatusResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /admin/settings [put]
func (h *Handlers) UpdateSetting(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "admin access required")
		return
	}

	var req SettingUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Key == "" {
		WriteError(w, http.StatusBadRequest, "key is required")
		return
	}

	if err := models.SetSetting(h.DB, req.Key, req.Value); err != nil {
		log.Printf("api: set setting %s: %v", req.Key, err)
		WriteError(w, http.StatusInternalServerError, "failed to update setting")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// TestLLMConnection tests the LLM provider connection.
//
//	@Summary      Test LLM provider connection (admin)
//	@Description  Always returns 200; check `success` field for actual result. Errors are surfaced via `error` field for the SPA to display inline.
//	@Tags         Admin
//	@Produce      json
//	@Success      200  {object}  map[string]interface{}
//	@Failure      403  {object}  api.APIError
//	@Router       /admin/settings/test-llm [post]
// testLLMPingTimeout caps Ping's wall-clock budget. The provider HTTP
// clients allow 5 minutes by default, which exceeds the HTTP server's
// WriteTimeout (60s in main.go) — a hung provider would otherwise leave
// the handler still running when the TCP write deadline fires, producing
// a reverse-proxy 502 with a misleading 200 in our access log. Exposed
// as a var so tests can shrink it.
var testLLMPingTimeout = 30 * time.Second

func (h *Handlers) TestLLMConnection(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "admin access required")
		return
	}

	provider, err := h.llmProvider()
	if err != nil {
		WriteJSON(w, http.StatusOK, map[string]any{"success": false, "error": err.Error()})
		return
	}

	pingCtx, cancel := context.WithTimeout(r.Context(), testLLMPingTimeout)
	defer cancel()

	if err := provider.Ping(pingCtx); err != nil {
		msg := err.Error()
		if errors.Is(err, context.DeadlineExceeded) {
			msg = "Provider did not respond in time. Check the base URL and network."
		}
		WriteJSON(w, http.StatusOK, map[string]any{"success": false, "error": msg})
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{"success": true})
}

// testNotifyTimeout caps each individual SMTP/webhook test send. Same
// rationale as testLLMPingTimeout (handlers_remaining.go): shoutrrr's
// default socket timeouts can exceed the HTTP server's WriteTimeout, which
// would silently turn into Caddy 502s with replog logging 200. Exposed as
// a var so tests can shrink it.
var testNotifyTimeout = 30 * time.Second

// TestNotifyConnection tests the notification provider connection.
//
//	@Summary      Test notification provider connection (admin)
//	@Description  Always returns 200; check `success` field. Errors are surfaced via `error` field for the SPA to display inline.
//	@Tags         Admin
//	@Produce      json
//	@Success      200  {object}  map[string]interface{}
//	@Failure      403  {object}  api.APIError
//	@Router       /admin/settings/test-notify [post]
func (h *Handlers) TestNotifyConnection(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "admin access required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), testNotifyTimeout)
	defer cancel()

	if err := notify.TestConnection(ctx, h.DB); err != nil {
		msg := err.Error()
		if errors.Is(err, context.DeadlineExceeded) {
			msg = "Notification provider did not respond in time. Check SMTP host and broadcast URLs."
		}
		WriteJSON(w, http.StatusOK, map[string]any{"success": false, "error": msg})
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{"success": true})
}
