package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/carpenike/replog/internal/importers"
	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// ImportUploadResponse is returned after parsing an uploaded file.
type ImportUploadResponse struct {
	Format    string                `json:"format"`
	Exercises []ImportMappingItem   `json:"exercises"`
	Equipment []ImportMappingItem   `json:"equipment,omitempty"`
	Programs  []ImportMappingItem   `json:"programs,omitempty"`
}

// ImportMappingItem represents one item that needs mapping.
type ImportMappingItem struct {
	Name     string `json:"name"`
	MappedID int64  `json:"mapped_id"`
	Create   bool   `json:"create"`
}

// ImportPreviewResponse shows what will be created.
type ImportPreviewResponse struct {
	WorkoutsCount  int `json:"workouts_count"`
	ExercisesNew   int `json:"exercises_new"`
	EquipmentNew   int `json:"equipment_new"`
	ProgramsNew    int `json:"programs_new"`
}

// ImportUpload parses an uploaded workout file and returns mapping data.
//
//	@Summary      Upload workouts file for import
//	@Description  Multipart upload. 'format' field selects parser ('strong', 'hevy', 'replog').
//	@Tags         Athletes
//	@Accept       multipart/form-data
//	@Produce      json
//	@Param        id      path      int     true   "Athlete ID"
//	@Param        format  formData  string  true   "strong | hevy | replog"
//	@Param        file    formData  file    true   "Workout file"
//	@Success      200  {object}  api.ImportUploadResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/import/upload [post]
func (h *Handlers) ImportUpload(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return
	}

	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid athlete ID")
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		WriteError(w, http.StatusBadRequest, "file too large (max 10MB)")
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		WriteError(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to read file")
		return
	}

	format := r.FormValue("format")

	var ms *importers.MappingState
	switch format {
	case "strong":
		parsed, err := importers.ParseStrongCSV(bytes.NewReader(data))
		if err != nil {
			WriteError(w, http.StatusBadRequest, "failed to parse Strong CSV: "+err.Error())
			return
		}
		existing, _ := models.ListExercises(h.DB, "")
		entities := exercisesToEntities(existing)
		ms = &importers.MappingState{
			Format:    importers.FormatStrongCSV,
			Parsed:    parsed,
			Exercises: importers.BuildExerciseMappings(parsed.Exercises, entities),
		}
	case "hevy":
		parsed, err := importers.ParseHevyCSV(bytes.NewReader(data))
		if err != nil {
			WriteError(w, http.StatusBadRequest, "failed to parse Hevy CSV: "+err.Error())
			return
		}
		existing, _ := models.ListExercises(h.DB, "")
		entities := exercisesToEntities(existing)
		ms = &importers.MappingState{
			Format:    importers.FormatHevyCSV,
			Parsed:    parsed,
			Exercises: importers.BuildExerciseMappings(parsed.Exercises, entities),
		}
	case "replog":
		parsed, err := importers.ParseRepLogJSON(bytes.NewReader(data))
		if err != nil {
			WriteError(w, http.StatusBadRequest, "failed to parse RepLog JSON: "+err.Error())
			return
		}
		existing, _ := models.ListExercises(h.DB, "")
		entities := exercisesToEntities(existing)
		ms = &importers.MappingState{
			Format:    importers.FormatRepLogJSON,
			Parsed:    parsed,
			Exercises: importers.BuildExerciseMappings(parsed.Exercises, entities),
		}
	default:
		WriteError(w, http.StatusBadRequest, "format must be 'strong', 'hevy', or 'replog'")
		return
	}

	// Store in session for next step.
	h.Sessions.Put(r.Context(), "api_import_mapping_"+strconv.FormatInt(athleteID, 10), ms)

	// Build response.
	resp := ImportUploadResponse{Format: format}
	for _, ex := range ms.Exercises {
		resp.Exercises = append(resp.Exercises, ImportMappingItem{
			Name: ex.ImportName, MappedID: ex.MappedID, Create: ex.Create,
		})
	}
	if ms.Equipment != nil {
		for _, eq := range ms.Equipment {
			resp.Equipment = append(resp.Equipment, ImportMappingItem{
				Name: eq.ImportName, MappedID: eq.MappedID, Create: eq.Create,
			})
		}
	}

	WriteJSON(w, http.StatusOK, resp)
}

// ImportExecute commits the import with finalized mappings.
//
//	@Summary      Commit workout import
//	@Description  Requires a prior successful upload (state lives in the session).
//	@Tags         Athletes
//	@Accept       json
//	@Produce      json
//	@Param        id    path      int                        true   "Athlete ID"
//	@Param        body  body      api.ImportExecuteRequest   true   "Mapping decisions"
//	@Success      200  {object}  map[string]interface{}
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/import/execute [post]
func (h *Handlers) ImportExecute(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "coach access required")
		return
	}

	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid athlete ID")
		return
	}

	sessionKey := "api_import_mapping_" + strconv.FormatInt(athleteID, 10)
	ms, ok := h.Sessions.Get(r.Context(), sessionKey).(*importers.MappingState)
	if !ok || ms == nil {
		WriteError(w, http.StatusBadRequest, "no import in progress — upload a file first")
		return
	}

	// Parse mapping updates from request.
	var req struct {
		Exercises []struct {
			Name     string `json:"name"`
			MappedID int64  `json:"mapped_id"`
			Create   bool   `json:"create"`
		} `json:"exercises"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Update mappings.
	for i, ex := range req.Exercises {
		if i < len(ms.Exercises) {
			ms.Exercises[i].MappedID = ex.MappedID
			ms.Exercises[i].Create = ex.Create
		}
	}

	// Execute import.
	result, err := models.ExecuteImport(h.DB, athleteID, user.ID, ms)
	if err != nil {
		log.Printf("api: execute import for athlete %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "import failed: "+err.Error())
		return
	}

	// Clean up session.
	h.Sessions.Remove(r.Context(), sessionKey)

	WriteJSON(w, http.StatusOK, map[string]any{
		"workouts_created":  result.WorkoutsCreated,
		"sets_created":      result.SetsCreated,
		"exercises_created": result.ExercisesCreated,
	})
}

// exercisesToEntities converts model exercises to the importer's ExistingEntity type.
func exercisesToEntities(exercises []*models.Exercise) []importers.ExistingEntity {
	entities := make([]importers.ExistingEntity, len(exercises))
	for i, e := range exercises {
		entities[i] = importers.ExistingEntity{ID: e.ID, Name: e.Name}
	}
	return entities
}
