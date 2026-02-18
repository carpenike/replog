package handlers

import (
	"bytes"
	"database/sql"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/alexedwards/scs/v2"
	"github.com/carpenike/replog/internal/importers"
	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

func init() {
	// Register types for session serialization.
	gob.Register(&importers.MappingState{})
	gob.Register(&importers.ParsedFile{})
}

// ImportExport holds dependencies for import/export handlers.
type ImportExport struct {
	DB        *sql.DB
	Sessions  *scs.SessionManager
	Templates TemplateCache
}

const maxUploadSize = 10 << 20 // 10 MB

// --- Export Handlers ---

// ExportPage renders the export options page for an athlete.
func (h *ImportExport) ExportPage(w http.ResponseWriter, r *http.Request) {
	if !checkAthleteAccess(h.DB, h.Templates, w, r) {
		return
	}

	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
		return
	}

	athlete, err := models.GetAthleteByID(h.DB, athleteID)
	if errors.Is(err, models.ErrNotFound) {
		h.Templates.NotFound(w, r)
		return
	}
	if err != nil {
		log.Printf("handlers: get athlete %d for export: %v", athleteID, err)
		h.Templates.ServerError(w, r)
		return
	}

	data := map[string]any{
		"Athlete": athlete,
	}
	if err := h.Templates.Render(w, r, "export.html", data); err != nil {
		log.Printf("handlers: render export page: %v", err)
		h.Templates.ServerError(w, r)
	}
}

// ExportJSON downloads the full RepLog JSON export for an athlete.
func (h *ImportExport) ExportJSON(w http.ResponseWriter, r *http.Request) {
	if !checkAthleteAccess(h.DB, h.Templates, w, r) {
		return
	}

	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
		return
	}

	export, err := models.BuildExportJSON(h.DB, athleteID)
	if errors.Is(err, models.ErrNotFound) {
		h.Templates.NotFound(w, r)
		return
	}
	if err != nil {
		log.Printf("handlers: build export json for athlete %d: %v", athleteID, err)
		h.Templates.ServerError(w, r)
		return
	}

	athlete, _ := models.GetAthleteByID(h.DB, athleteID)
	filename := "replog-export.json"
	if athlete != nil {
		filename = fmt.Sprintf("replog-%s.json", sanitizeFilename(athlete.Name))
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	if err := models.WriteExportJSON(w, export); err != nil {
		log.Printf("handlers: write export json: %v", err)
	}
}

// ExportCSV downloads a Strong-compatible CSV export for an athlete.
func (h *ImportExport) ExportCSV(w http.ResponseWriter, r *http.Request) {
	if !checkAthleteAccess(h.DB, h.Templates, w, r) {
		return
	}

	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
		return
	}

	athlete, err := models.GetAthleteByID(h.DB, athleteID)
	if errors.Is(err, models.ErrNotFound) {
		h.Templates.NotFound(w, r)
		return
	}
	if err != nil {
		log.Printf("handlers: get athlete %d for csv export: %v", athleteID, err)
		h.Templates.ServerError(w, r)
		return
	}

	filename := fmt.Sprintf("replog-%s.csv", sanitizeFilename(athlete.Name))
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	if err := models.WriteExportStrongCSV(w, h.DB, athleteID); err != nil {
		log.Printf("handlers: write export csv: %v", err)
	}
}

// --- Import Handlers ---

// ImportPage renders the import upload page.
func (h *ImportExport) ImportPage(w http.ResponseWriter, r *http.Request) {
	if !checkAthleteAccess(h.DB, h.Templates, w, r) {
		return
	}

	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
		return
	}

	athlete, err := models.GetAthleteByID(h.DB, athleteID)
	if errors.Is(err, models.ErrNotFound) {
		h.Templates.NotFound(w, r)
		return
	}
	if err != nil {
		log.Printf("handlers: get athlete %d for import: %v", athleteID, err)
		h.Templates.ServerError(w, r)
		return
	}

	data := map[string]any{
		"Athlete": athlete,
	}
	if err := h.Templates.Render(w, r, "import.html", data); err != nil {
		log.Printf("handlers: render import page: %v", err)
		h.Templates.ServerError(w, r)
	}
}

// Upload handles the file upload, parses it, and redirects to the mapping step.
func (h *ImportExport) Upload(w http.ResponseWriter, r *http.Request) {
	if !checkAthleteAccess(h.DB, h.Templates, w, r) {
		return
	}

	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
		return
	}

	athlete, err := models.GetAthleteByID(h.DB, athleteID)
	if errors.Is(err, models.ErrNotFound) {
		h.Templates.NotFound(w, r)
		return
	}
	if err != nil {
		log.Printf("handlers: get athlete %d for upload: %v", athleteID, err)
		h.Templates.ServerError(w, r)
		return
	}

	// Parse multipart form.
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		data := map[string]any{
			"Athlete": athlete,
			"Error":   "File too large. Maximum size is 10 MB.",
		}
		h.Templates.Render(w, r, "import.html", data)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		data := map[string]any{
			"Athlete": athlete,
			"Error":   "Please select a file to upload.",
		}
		h.Templates.Render(w, r, "import.html", data)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxUploadSize+1))
	if err != nil {
		log.Printf("handlers: read upload file: %v", err)
		h.Templates.ServerError(w, r)
		return
	}
	if int64(len(data)) > maxUploadSize {
		tplData := map[string]any{
			"Athlete": athlete,
			"Error":   "File too large. Maximum size is 10 MB.",
		}
		h.Templates.Render(w, r, "import.html", tplData)
		return
	}

	// Detect format.
	format := importers.DetectFormat(data)
	if format == "" {
		// Try user-selected format.
		switch r.FormValue("format") {
		case "strong_csv":
			format = importers.FormatStrongCSV
		case "hevy_csv":
			format = importers.FormatHevyCSV
		case "replog_json":
			format = importers.FormatRepLogJSON
		default:
			tplData := map[string]any{
				"Athlete": athlete,
				"Error":   "Could not detect file format. Please select the format manually.",
			}
			h.Templates.Render(w, r, "import.html", tplData)
			return
		}
	}

	// Parse the file.
	var parsed *importers.ParsedFile
	var parseErr error
	switch format {
	case importers.FormatStrongCSV:
		parsed, parseErr = importers.ParseStrongCSV(bytes.NewReader(data))
	case importers.FormatHevyCSV:
		parsed, parseErr = importers.ParseHevyCSV(bytes.NewReader(data))
	case importers.FormatRepLogJSON:
		parsed, parseErr = importers.ParseRepLogJSON(bytes.NewReader(data))
	}
	if parseErr != nil {
		log.Printf("handlers: parse %s file: %v", format, parseErr)
		tplData := map[string]any{
			"Athlete": athlete,
			"Error":   fmt.Sprintf("Failed to parse file: %v", parseErr),
		}
		h.Templates.Render(w, r, "import.html", tplData)
		return
	}

	if len(parsed.Workouts) == 0 && len(parsed.Exercises) == 0 {
		tplData := map[string]any{
			"Athlete": athlete,
			"Error":   "No data found in the uploaded file.",
		}
		h.Templates.Render(w, r, "import.html", tplData)
		return
	}

	// Weight unit from form or file.
	weightUnit := r.FormValue("weight_unit")
	if weightUnit == "" && parsed.WeightUnit != "" {
		weightUnit = parsed.WeightUnit
	}
	if weightUnit == "" {
		weightUnit = "lbs"
	}

	// Build initial mappings.
	existingExercises, err := listExistingExercises(h.DB)
	if err != nil {
		log.Printf("handlers: list exercises for mapping: %v", err)
		h.Templates.ServerError(w, r)
		return
	}

	ms := &importers.MappingState{
		Format:     format,
		WeightUnit: weightUnit,
		Parsed:     parsed,
		Exercises:  importers.BuildExerciseMappings(parsed.Exercises, existingExercises),
	}

	// Equipment and program mappings (RepLog JSON only).
	if format == importers.FormatRepLogJSON {
		existingEquipment, err := listExistingEquipment(h.DB)
		if err != nil {
			log.Printf("handlers: list equipment for mapping: %v", err)
			h.Templates.ServerError(w, r)
			return
		}
		ms.Equipment = importers.BuildEquipmentMappings(parsed.Equipment, existingEquipment)

		existingPrograms, err := listExistingPrograms(h.DB)
		if err != nil {
			log.Printf("handlers: list programs for mapping: %v", err)
			h.Templates.ServerError(w, r)
			return
		}
		ms.Programs = importers.BuildProgramMappings(parsed.Programs, existingPrograms)
	}

	// Store mapping state in session.
	h.Sessions.Put(r.Context(), "import_mapping", ms)

	http.Redirect(w, r, fmt.Sprintf("/athletes/%d/import/map", athleteID), http.StatusSeeOther)
}

// MapPage renders the mapping UI.
func (h *ImportExport) MapPage(w http.ResponseWriter, r *http.Request) {
	if !checkAthleteAccess(h.DB, h.Templates, w, r) {
		return
	}

	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
		return
	}

	athlete, err := models.GetAthleteByID(h.DB, athleteID)
	if errors.Is(err, models.ErrNotFound) {
		h.Templates.NotFound(w, r)
		return
	}
	if err != nil {
		log.Printf("handlers: get athlete %d for mapping: %v", athleteID, err)
		h.Templates.ServerError(w, r)
		return
	}

	ms, ok := h.Sessions.Get(r.Context(), "import_mapping").(*importers.MappingState)
	if !ok || ms == nil {
		http.Redirect(w, r, fmt.Sprintf("/athletes/%d/import", athleteID), http.StatusSeeOther)
		return
	}

	// Load existing entities for dropdowns.
	existingExercises, _ := listExistingExercises(h.DB)
	existingEquipment, _ := listExistingEquipment(h.DB)
	existingPrograms, _ := listExistingPrograms(h.DB)

	tplData := map[string]any{
		"Athlete":           athlete,
		"MappingState":      ms,
		"ExistingExercises": existingExercises,
		"ExistingEquipment": existingEquipment,
		"ExistingPrograms":  existingPrograms,
		"IsRepLogJSON":      ms.Format == importers.FormatRepLogJSON,
	}
	if err := h.Templates.Render(w, r, "import_map.html", tplData); err != nil {
		log.Printf("handlers: render mapping page: %v", err)
		h.Templates.ServerError(w, r)
	}
}

// Preview processes the mapping form and shows a dry-run summary.
func (h *ImportExport) Preview(w http.ResponseWriter, r *http.Request) {
	if !checkAthleteAccess(h.DB, h.Templates, w, r) {
		return
	}

	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
		return
	}

	athlete, err := models.GetAthleteByID(h.DB, athleteID)
	if errors.Is(err, models.ErrNotFound) {
		h.Templates.NotFound(w, r)
		return
	}
	if err != nil {
		log.Printf("handlers: get athlete %d for preview: %v", athleteID, err)
		h.Templates.ServerError(w, r)
		return
	}

	ms, ok := h.Sessions.Get(r.Context(), "import_mapping").(*importers.MappingState)
	if !ok || ms == nil {
		http.Redirect(w, r, fmt.Sprintf("/athletes/%d/import", athleteID), http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Update exercise mappings from form.
	for i := range ms.Exercises {
		key := fmt.Sprintf("exercise_%d", i)
		val := r.FormValue(key)
		if val == "create" {
			ms.Exercises[i].MappedID = 0
			ms.Exercises[i].MappedName = ""
			ms.Exercises[i].Create = true
		} else if id, err := strconv.ParseInt(val, 10, 64); err == nil && id > 0 {
			ms.Exercises[i].MappedID = id
			ms.Exercises[i].Create = false
			// Fetch name for display.
			if ex, err := models.GetExerciseByID(h.DB, id); err == nil {
				ms.Exercises[i].MappedName = ex.Name
			}
		}
	}

	// Update equipment mappings (RepLog JSON only).
	for i := range ms.Equipment {
		key := fmt.Sprintf("equipment_%d", i)
		val := r.FormValue(key)
		if val == "create" {
			ms.Equipment[i].MappedID = 0
			ms.Equipment[i].MappedName = ""
			ms.Equipment[i].Create = true
		} else if id, err := strconv.ParseInt(val, 10, 64); err == nil && id > 0 {
			ms.Equipment[i].MappedID = id
			ms.Equipment[i].Create = false
			if eq, err := models.GetEquipmentByID(h.DB, id); err == nil {
				ms.Equipment[i].MappedName = eq.Name
			}
		}
	}

	// Update program mappings (RepLog JSON only).
	for i := range ms.Programs {
		key := fmt.Sprintf("program_%d", i)
		val := r.FormValue(key)
		if val == "create" {
			ms.Programs[i].MappedID = 0
			ms.Programs[i].MappedName = ""
			ms.Programs[i].Create = true
		} else if id, err := strconv.ParseInt(val, 10, 64); err == nil && id > 0 {
			ms.Programs[i].MappedID = id
			ms.Programs[i].Create = false
			if pt, err := models.GetProgramTemplateByID(h.DB, id); err == nil {
				ms.Programs[i].MappedName = pt.Name
			}
		}
	}

	// Save updated mapping state.
	h.Sessions.Put(r.Context(), "import_mapping", ms)

	// Build preview.
	preview, err := models.BuildImportPreview(h.DB, athleteID, ms)
	if err != nil {
		log.Printf("handlers: build import preview: %v", err)
		h.Templates.ServerError(w, r)
		return
	}

	tplData := map[string]any{
		"Athlete":      athlete,
		"Preview":      preview,
		"MappingState": ms,
		"IsRepLogJSON": ms.Format == importers.FormatRepLogJSON,
	}
	if err := h.Templates.Render(w, r, "import_preview.html", tplData); err != nil {
		log.Printf("handlers: render preview page: %v", err)
		h.Templates.ServerError(w, r)
	}
}

// Execute performs the actual import.
func (h *ImportExport) Execute(w http.ResponseWriter, r *http.Request) {
	if !checkAthleteAccess(h.DB, h.Templates, w, r) {
		return
	}

	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
		return
	}

	athlete, err := models.GetAthleteByID(h.DB, athleteID)
	if errors.Is(err, models.ErrNotFound) {
		h.Templates.NotFound(w, r)
		return
	}
	if err != nil {
		log.Printf("handlers: get athlete %d for execute: %v", athleteID, err)
		h.Templates.ServerError(w, r)
		return
	}

	ms, ok := h.Sessions.Get(r.Context(), "import_mapping").(*importers.MappingState)
	if !ok || ms == nil {
		http.Redirect(w, r, fmt.Sprintf("/athletes/%d/import", athleteID), http.StatusSeeOther)
		return
	}

	// Get the coach user ID for reviews.
	user := middleware.UserFromContext(r.Context())
	coachID := user.ID

	result, err := models.ExecuteImport(h.DB, athleteID, coachID, ms)
	if err != nil {
		log.Printf("handlers: execute import for athlete %d: %v", athleteID, err)
		tplData := map[string]any{
			"Athlete": athlete,
			"Error":   fmt.Sprintf("Import failed: %v", err),
		}
		h.Templates.Render(w, r, "import.html", tplData)
		return
	}

	// Clear mapping state from session.
	h.Sessions.Remove(r.Context(), "import_mapping")

	tplData := map[string]any{
		"Athlete": athlete,
		"Result":  result,
	}
	if err := h.Templates.Render(w, r, "import_result.html", tplData); err != nil {
		log.Printf("handlers: render import result: %v", err)
		h.Templates.ServerError(w, r)
	}
}

// --- Helpers ---

func listExistingExercises(db *sql.DB) ([]importers.ExistingEntity, error) {
	exercises, err := models.ListExercises(db, "")
	if err != nil {
		return nil, err
	}
	result := make([]importers.ExistingEntity, len(exercises))
	for i, e := range exercises {
		result[i] = importers.ExistingEntity{ID: e.ID, Name: e.Name}
	}
	return result, nil
}

func listExistingEquipment(db *sql.DB) ([]importers.ExistingEntity, error) {
	equipment, err := models.ListEquipment(db)
	if err != nil {
		return nil, err
	}
	result := make([]importers.ExistingEntity, len(equipment))
	for i, e := range equipment {
		result[i] = importers.ExistingEntity{ID: e.ID, Name: e.Name}
	}
	return result, nil
}

func listExistingPrograms(db *sql.DB) ([]importers.ExistingEntity, error) {
	programs, err := models.ListProgramTemplates(db)
	if err != nil {
		return nil, err
	}
	result := make([]importers.ExistingEntity, len(programs))
	for i, p := range programs {
		result[i] = importers.ExistingEntity{ID: p.ID, Name: p.Name}
	}
	return result, nil
}

func sanitizeFilename(name string) string {
	r := strings.NewReplacer(
		" ", "-", "/", "-", "\\", "-",
		".", "", ",", "", "'", "", "\"", "",
	)
	s := r.Replace(strings.ToLower(name))
	// Remove any remaining non-alphanumeric chars except dash.
	var clean []byte
	for _, c := range []byte(s) {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			clean = append(clean, c)
		}
	}
	return string(clean)
}

// marshalMappingState serializes mapping state to JSON for session storage.
func marshalMappingState(ms *importers.MappingState) ([]byte, error) {
	return json.Marshal(ms)
}

// unmarshalMappingState deserializes mapping state from JSON.
func unmarshalMappingState(data []byte) (*importers.MappingState, error) {
	var ms importers.MappingState
	if err := json.Unmarshal(data, &ms); err != nil {
		return nil, err
	}
	return &ms, nil
}

// --- Catalog Export/Import Handlers (global — no athlete) ---

// CatalogExportPage renders the catalog export page.
func (h *ImportExport) CatalogExportPage(w http.ResponseWriter, r *http.Request) {
	if err := h.Templates.Render(w, r, "catalog_export.html", nil); err != nil {
		log.Printf("handlers: render catalog export page: %v", err)
		h.Templates.ServerError(w, r)
	}
}

// CatalogExportJSON downloads the full catalog JSON.
func (h *ImportExport) CatalogExportJSON(w http.ResponseWriter, r *http.Request) {
	catalog, err := models.BuildCatalogExportJSON(h.DB)
	if err != nil {
		log.Printf("handlers: build catalog export json: %v", err)
		h.Templates.ServerError(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="replog-catalog.json"`)
	if err := models.WriteCatalogJSON(w, catalog); err != nil {
		log.Printf("handlers: write catalog json: %v", err)
	}
}

// CatalogImportPage renders the catalog import upload page.
func (h *ImportExport) CatalogImportPage(w http.ResponseWriter, r *http.Request) {
	if err := h.Templates.Render(w, r, "catalog_import.html", nil); err != nil {
		log.Printf("handlers: render catalog import page: %v", err)
		h.Templates.ServerError(w, r)
	}
}

// CatalogUpload handles the catalog file upload and redirects to mapping.
func (h *ImportExport) CatalogUpload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		data := map[string]any{"Error": "File too large. Maximum size is 10 MB."}
		h.Templates.Render(w, r, "catalog_import.html", data)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		data := map[string]any{"Error": "Please select a file to upload."}
		h.Templates.Render(w, r, "catalog_import.html", data)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxUploadSize+1))
	if err != nil {
		log.Printf("handlers: read catalog upload: %v", err)
		h.Templates.ServerError(w, r)
		return
	}
	if int64(len(data)) > maxUploadSize {
		tplData := map[string]any{"Error": "File too large. Maximum size is 10 MB."}
		h.Templates.Render(w, r, "catalog_import.html", tplData)
		return
	}

	// Detect and validate format — catalog must be catalog JSON.
	format := importers.DetectFormat(data)
	if format != importers.FormatCatalogJSON {
		tplData := map[string]any{"Error": "Invalid file format. Please upload a RepLog catalog JSON file."}
		h.Templates.Render(w, r, "catalog_import.html", tplData)
		return
	}

	parsed, err := importers.ParseCatalogJSON(bytes.NewReader(data))
	if err != nil {
		log.Printf("handlers: parse catalog json: %v", err)
		tplData := map[string]any{"Error": fmt.Sprintf("Failed to parse catalog file: %v", err)}
		h.Templates.Render(w, r, "catalog_import.html", tplData)
		return
	}

	if len(parsed.Exercises) == 0 && len(parsed.Equipment) == 0 && len(parsed.Programs) == 0 {
		tplData := map[string]any{"Error": "No catalog data found in the uploaded file."}
		h.Templates.Render(w, r, "catalog_import.html", tplData)
		return
	}

	// Build mappings.
	existingExercises, err := listExistingExercises(h.DB)
	if err != nil {
		log.Printf("handlers: list exercises for catalog mapping: %v", err)
		h.Templates.ServerError(w, r)
		return
	}
	existingEquipment, err := listExistingEquipment(h.DB)
	if err != nil {
		log.Printf("handlers: list equipment for catalog mapping: %v", err)
		h.Templates.ServerError(w, r)
		return
	}
	existingPrograms, err := listExistingPrograms(h.DB)
	if err != nil {
		log.Printf("handlers: list programs for catalog mapping: %v", err)
		h.Templates.ServerError(w, r)
		return
	}

	ms := &importers.MappingState{
		Format:    importers.FormatCatalogJSON,
		Parsed:    parsed,
		Exercises: importers.BuildExerciseMappings(parsed.Exercises, existingExercises),
		Equipment: importers.BuildEquipmentMappings(parsed.Equipment, existingEquipment),
		Programs:  importers.BuildProgramMappings(parsed.Programs, existingPrograms),
	}

	h.Sessions.Put(r.Context(), "catalog_import_mapping", ms)
	http.Redirect(w, r, "/catalog/import/map", http.StatusSeeOther)
}

// CatalogMapPage renders the catalog mapping UI.
func (h *ImportExport) CatalogMapPage(w http.ResponseWriter, r *http.Request) {
	ms, ok := h.Sessions.Get(r.Context(), "catalog_import_mapping").(*importers.MappingState)
	if !ok || ms == nil {
		http.Redirect(w, r, "/catalog/import", http.StatusSeeOther)
		return
	}

	existingExercises, _ := listExistingExercises(h.DB)
	existingEquipment, _ := listExistingEquipment(h.DB)
	existingPrograms, _ := listExistingPrograms(h.DB)

	tplData := map[string]any{
		"MappingState":      ms,
		"ExistingExercises": existingExercises,
		"ExistingEquipment": existingEquipment,
		"ExistingPrograms":  existingPrograms,
	}
	if err := h.Templates.Render(w, r, "catalog_import_map.html", tplData); err != nil {
		log.Printf("handlers: render catalog mapping page: %v", err)
		h.Templates.ServerError(w, r)
	}
}

// CatalogPreview processes the mapping form and shows a dry-run summary.
func (h *ImportExport) CatalogPreview(w http.ResponseWriter, r *http.Request) {
	ms, ok := h.Sessions.Get(r.Context(), "catalog_import_mapping").(*importers.MappingState)
	if !ok || ms == nil {
		http.Redirect(w, r, "/catalog/import", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Update exercise mappings from form.
	for i := range ms.Exercises {
		key := fmt.Sprintf("exercise_%d", i)
		val := r.FormValue(key)
		if val == "create" {
			ms.Exercises[i].MappedID = 0
			ms.Exercises[i].MappedName = ""
			ms.Exercises[i].Create = true
		} else if id, err := strconv.ParseInt(val, 10, 64); err == nil && id > 0 {
			ms.Exercises[i].MappedID = id
			ms.Exercises[i].Create = false
			if ex, err := models.GetExerciseByID(h.DB, id); err == nil {
				ms.Exercises[i].MappedName = ex.Name
			}
		}
	}

	// Update equipment mappings.
	for i := range ms.Equipment {
		key := fmt.Sprintf("equipment_%d", i)
		val := r.FormValue(key)
		if val == "create" {
			ms.Equipment[i].MappedID = 0
			ms.Equipment[i].MappedName = ""
			ms.Equipment[i].Create = true
		} else if id, err := strconv.ParseInt(val, 10, 64); err == nil && id > 0 {
			ms.Equipment[i].MappedID = id
			ms.Equipment[i].Create = false
			if eq, err := models.GetEquipmentByID(h.DB, id); err == nil {
				ms.Equipment[i].MappedName = eq.Name
			}
		}
	}

	// Update program mappings.
	for i := range ms.Programs {
		key := fmt.Sprintf("program_%d", i)
		val := r.FormValue(key)
		if val == "create" {
			ms.Programs[i].MappedID = 0
			ms.Programs[i].MappedName = ""
			ms.Programs[i].Create = true
		} else if id, err := strconv.ParseInt(val, 10, 64); err == nil && id > 0 {
			ms.Programs[i].MappedID = id
			ms.Programs[i].Create = false
			if pt, err := models.GetProgramTemplateByID(h.DB, id); err == nil {
				ms.Programs[i].MappedName = pt.Name
			}
		}
	}

	h.Sessions.Put(r.Context(), "catalog_import_mapping", ms)

	preview := models.BuildCatalogImportPreview(ms)

	tplData := map[string]any{
		"Preview":      preview,
		"MappingState": ms,
	}
	if err := h.Templates.Render(w, r, "catalog_import_preview.html", tplData); err != nil {
		log.Printf("handlers: render catalog preview page: %v", err)
		h.Templates.ServerError(w, r)
	}
}

// CatalogExecute performs the catalog import.
func (h *ImportExport) CatalogExecute(w http.ResponseWriter, r *http.Request) {
	ms, ok := h.Sessions.Get(r.Context(), "catalog_import_mapping").(*importers.MappingState)
	if !ok || ms == nil {
		http.Redirect(w, r, "/catalog/import", http.StatusSeeOther)
		return
	}

	result, err := models.ExecuteCatalogImport(h.DB, ms)
	if err != nil {
		log.Printf("handlers: execute catalog import: %v", err)
		tplData := map[string]any{"Error": fmt.Sprintf("Catalog import failed: %v", err)}
		h.Templates.Render(w, r, "catalog_import.html", tplData)
		return
	}

	h.Sessions.Remove(r.Context(), "catalog_import_mapping")

	tplData := map[string]any{
		"Result": result,
	}
	if err := h.Templates.Render(w, r, "catalog_import_result.html", tplData); err != nil {
		log.Printf("handlers: render catalog import result: %v", err)
		h.Templates.ServerError(w, r)
	}
}
