package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/carpenike/replog/internal/importers"
	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// --- Catalog Export ---

// CatalogExportJSON returns the full exercise/equipment/program catalog as JSON.
//
//	@Summary      Export catalog as JSON
//	@Tags         Admin
//	@Produce      json
//	@Success      200  {object}  map[string]interface{}
//	@Failure      403  {object}  api.APIError
//	@Router       /catalog/export [get]
func (h *Handlers) CatalogExportJSON(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "admin access required")
		return
	}

	catalog, err := models.BuildCatalogExportJSON(h.DB)
	if err != nil {
		log.Printf("api: build catalog export: %v", err)
		WriteError(w, http.StatusInternalServerError, "failed to build catalog")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="replog-catalog.json"`)
	if err := models.WriteCatalogJSON(w, catalog); err != nil {
		log.Printf("api: write catalog JSON: %v", err)
	}
}

// --- Catalog Import ---

// CatalogImportUpload parses an uploaded catalog JSON file.
//
//	@Summary      Upload catalog JSON for import
//	@Description  Stores parsed mappings in the session; call /catalog/import/execute to commit.
//	@Tags         Admin
//	@Accept       multipart/form-data
//	@Produce      json
//	@Param        file  formData  file  true  "Catalog JSON file"
//	@Success      200  {object}  map[string]interface{}  "Counts of items detected"
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /catalog/import/upload [post]
func (h *Handlers) CatalogImportUpload(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "admin access required")
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		WriteError(w, http.StatusBadRequest, "file too large")
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

	parsed, err := importers.ParseCatalogJSON(bytes.NewReader(data))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid catalog JSON: "+err.Error())
		return
	}

	// Build mappings against existing entities.
	existingEx, _ := models.ListExercises(h.DB, "")
	exEntities := exercisesToEntities(existingEx)

	existingEq, _ := models.ListEquipment(h.DB)
	eqEntities := make([]importers.ExistingEntity, len(existingEq))
	for i, e := range existingEq {
		eqEntities[i] = importers.ExistingEntity{ID: e.ID, Name: e.Name}
	}

	ms := &importers.MappingState{
		Format:    importers.FormatCatalogJSON,
		Exercises: importers.BuildExerciseMappings(parsed.Exercises, exEntities),
		Equipment: importers.BuildEquipmentMappings(parsed.Equipment, eqEntities),
		Programs:  importers.BuildProgramMappings(parsed.Programs, nil),
		Parsed:    parsed,
	}

	// Store in session.
	h.Sessions.Put(r.Context(), "api_catalog_import", ms)

	// Build response summary.
	resp := map[string]any{
		"exercises": len(ms.Exercises),
		"equipment": len(ms.Equipment),
		"programs":  len(ms.Programs),
	}
	WriteJSON(w, http.StatusOK, resp)
}

// CatalogImportExecute commits the catalog import.
//
//	@Summary      Commit catalog import
//	@Description  Requires a prior successful upload (state lives in the session). Body can override the per-exercise mapping decisions.
//	@Tags         Admin
//	@Accept       json
//	@Produce      json
//	@Param        body  body      api.CatalogImportExecuteRequest  false  "Mapping overrides"
//	@Success      200  {object}  map[string]interface{}  "Counts of created items"
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /catalog/import/execute [post]
func (h *Handlers) CatalogImportExecute(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "admin access required")
		return
	}

	ms, ok := h.Sessions.Get(r.Context(), "api_catalog_import").(*importers.MappingState)
	if !ok || ms == nil {
		WriteError(w, http.StatusBadRequest, "no catalog import in progress")
		return
	}

	// Apply any mapping updates from the request.
	var req struct {
		Exercises []struct {
			MappedID int64 `json:"mapped_id"`
			Create   bool  `json:"create"`
		} `json:"exercises"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
		for i, ex := range req.Exercises {
			if i < len(ms.Exercises) {
				ms.Exercises[i].MappedID = ex.MappedID
				ms.Exercises[i].Create = ex.Create
			}
		}
	}

	result, err := models.ExecuteCatalogImport(h.DB, ms, nil)
	if err != nil {
		log.Printf("api: execute catalog import: %v", err)
		WriteError(w, http.StatusInternalServerError, "catalog import failed: "+err.Error())
		return
	}

	h.Sessions.Remove(r.Context(), "api_catalog_import")

	WriteJSON(w, http.StatusOK, map[string]any{
		"exercises_created":   result.ExercisesCreated,
		"equipment_created":   result.EquipmentCreated,
		"programs_created":    result.ProgramsCreated,
		"prescribed_sets":     result.PrescribedSets,
		"progression_rules":   result.ProgressionRules,
	})
}
