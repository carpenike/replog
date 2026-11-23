package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"

	"github.com/carpenike/replog/internal/models"
)

// --- Health import (gap 14) ---

// bioImportMaxBody caps an import request. A year of daily readings for
// four metrics in Health Auto Export's shape is well under this.
const bioImportMaxBody = 5 << 20 // 5 MiB

// bioImportMaxErrors bounds the per-sample error list in the response.
const bioImportMaxErrors = 50

// ImportBioSamples records a batch of watch/phone health readings.
//
//	@Summary      Import health samples
//	@Description  Batched import of sleep, resting HR, HRV and step readings into bio_samples with source watch_import, for an iOS Shortcut or Health Auto Export automation. Idempotent per (athlete, metric, recorded_at): a reading already stored (imported or entered by hand) is counted as a duplicate, so overlapping windows can be re-sent. Accepts the generic samples array and/or Health Auto Export's data.metrics object. Invalid readings are skipped and listed; the rest are stored. Authenticate with a browser session or a service token (Authorization: Bearer rlcst_...) that can see the athlete. Readings inform the coach; they never change a plan. See docs/health-import.md.
//	@Tags         Athletes
//	@Accept       json
//	@Produce      json
//	@Param        id    path      int                         true  "Athlete ID"
//	@Param        body  body      api.BioSampleImportRequest  true  "Readings"
//	@Success      200  {object}  api.BioSampleImportResponse
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      413  {object}  api.APIError
//	@Router       /athletes/{id}/bio-samples/import [post]
func (h *Handlers) ImportBioSamples(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, bioImportMaxBody)
	var req BioSampleImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			WriteError(w, http.StatusRequestEntityTooLarge, "import too large (max 5 MiB); send smaller batches")
			return
		}
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	raw, ignored := flattenBioImport(&req)
	if len(raw) == 0 && len(ignored) == 0 {
		WriteError(w, http.StatusBadRequest, "no samples: send samples[] or Health Auto Export data.metrics[]")
		return
	}
	if len(raw) > models.MaxBioImportSamples {
		WriteError(w, http.StatusRequestEntityTooLarge,
			fmt.Sprintf("too many samples (%d); send at most %d per request", len(raw), models.MaxBioImportSamples))
		return
	}

	loc := userLocation(r)
	resp := BioSampleImportResponse{Received: len(raw), IgnoredMetrics: ignored}
	valid := make([]models.NormalizedBioSample, 0, len(raw))
	for i, s := range raw {
		n, err := models.NormalizeBioImportSample(s, loc)
		if err != nil {
			resp.Skipped++
			if len(resp.Errors) < bioImportMaxErrors {
				resp.Errors = append(resp.Errors, BioSampleImportError{Index: i, Metric: s.Metric, Reason: err.Error()})
			}
			continue
		}
		valid = append(valid, n)
	}

	if len(valid) > 0 {
		inserted, dups, err := models.ImportBioSamples(r.Context(), h.DB, athleteID, valid)
		if err != nil {
			log.Printf("api: import bio samples for athlete %d: %v", athleteID, err)
			WriteError(w, http.StatusInternalServerError, "failed to import samples")
			return
		}
		resp.Inserted, resp.Duplicates = inserted, dups
	}
	WriteJSON(w, http.StatusOK, resp)
}

// flattenBioImport turns both accepted request shapes into one list, in
// request order: samples first, then each Health Auto Export series.
// Health Auto Export points with no usable value are kept with a NaN value
// so they are reported as skipped rather than silently dropped. A whole
// Health Auto Export series RepLog does not import (heart_rate,
// active_energy, ...) is named once in ignored instead, so exporting every
// metric does not flood the error list.
func flattenBioImport(req *BioSampleImportRequest) (out []models.BioImportSample, ignored []string) {
	for _, s := range req.Samples {
		out = append(out, models.BioImportSample{
			Metric: s.Metric, Value: s.Value, Unit: s.Unit, RecordedAt: s.RecordedAt,
		})
	}
	if req.Data == nil {
		return out, nil
	}
	for _, m := range req.Data.Metrics {
		key, known := models.CanonicalBioMetric(m.Name)
		if !known {
			ignored = append(ignored, m.Name)
			continue
		}
		for _, p := range m.Data {
			v, ok := haePointValue(key, p)
			if !ok {
				v = math.NaN()
			}
			out = append(out, models.BioImportSample{
				Metric: m.Name, Value: v, Unit: m.Units, RecordedAt: p.Date,
			})
		}
	}
	return out, ignored
}

// haePointValue picks a Health Auto Export point's value: hours asleep for
// the sleep series, qty for everything else.
func haePointValue(key string, p HealthAutoExportPoint) (float64, bool) {
	if key == "sleep_hours" {
		switch {
		case p.Asleep != nil:
			return *p.Asleep, true
		case p.TotalSleep != nil:
			return *p.TotalSleep, true
		}
	}
	if p.Qty != nil {
		return *p.Qty, true
	}
	return 0, false
}
