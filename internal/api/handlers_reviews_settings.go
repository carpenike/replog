package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
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
func (h *Handlers) UpdateSetting(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "admin access required")
		return
	}

	var req struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
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
