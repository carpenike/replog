package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/carpenike/replog/internal/media"
	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// --- Form-check videos (gap 12) ---
//
// An athlete (or their coach) attaches one short clip to a logged set; the
// coach watches it and leaves a comment in the workout review. Clips are
// stored under REPLOG_VIDEO_DIR and streamed back only through the
// authenticated set route below, scoped to the path athlete like every
// other child resource (ADR 021). A comment is review feedback; it never
// changes the program.

// multipartOverhead is the allowance on top of the clip cap for multipart
// boundaries and part headers.
const multipartOverhead = 1 << 20

// videoTransferTimeout replaces the server's 30s read / 60s write deadlines
// for a clip upload or stream: a 100 MiB clip on gym wifi takes minutes.
const videoTransferTimeout = 15 * time.Minute

// extendVideoDeadlines lifts the server-wide deadlines for one video
// transfer. Errors (a writer that cannot set deadlines, as in tests) are
// ignored; the server defaults then apply.
func extendVideoDeadlines(w http.ResponseWriter, read bool) {
	rc := http.NewResponseController(w)
	deadline := time.Now().Add(videoTransferTimeout)
	if read {
		_ = rc.SetReadDeadline(deadline)
	}
	_ = rc.SetWriteDeadline(deadline)
}

// serviceVideoRefusal is the 403 a service token receives on a video write.
const serviceVideoRefusal = "service token cannot change form-check videos"

// videoStore returns the clip store, or nil when video storage is off.
func (h *Handlers) videoStore() *media.Store {
	if h.VideoDir == "" {
		return nil
	}
	return &media.Store{Dir: h.VideoDir, MaxBytes: h.VideoMaxBytes}
}

// videoMaxBytes is the effective per-clip cap.
func (h *Handlers) videoMaxBytes() int64 {
	if h.VideoMaxBytes > 0 {
		return h.VideoMaxBytes
	}
	return media.DefaultMaxVideoBytes
}

// ownedSet parses {workoutID} and {setID} and checks the set is in that
// workout and the workout belongs to the path athlete. It writes 400/404.
func (h *Handlers) ownedSet(w http.ResponseWriter, r *http.Request, athleteID int64) (workoutID, setID int64, ok bool) {
	workoutID, err := strconv.ParseInt(r.PathValue("workoutID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid workout ID")
		return 0, 0, false
	}
	setID, err = strconv.ParseInt(r.PathValue("setID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid set ID")
		return 0, 0, false
	}
	if err := models.CheckSetOwnership(r.Context(), h.DB, setID, workoutID, athleteID); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "set not found")
			return 0, 0, false
		}
		log.Printf("api: check set %d: %v", setID, err)
		WriteError(w, http.StatusInternalServerError, "failed to load set")
		return 0, 0, false
	}
	return workoutID, setID, true
}

// UploadSetVideo attaches a form-check clip to a logged set.
//
//	@Summary      Upload a form-check video
//	@Description  Multipart upload; field name is 'video'. The server sniffs the container and accepts only MP4, QuickTime (.mov, as an iPhone records) or WebM, up to the configured cap (REPLOG_VIDEO_MAX_MB, default 100 MiB). Uploading again replaces the set's clip and clears its coach comment. The clip is served back only through the authenticated set route. Browser session only.
//	@Tags         Workouts
//	@Accept       multipart/form-data
//	@Produce      json
//	@Param        id         path      int   true  "Athlete ID"
//	@Param        workoutID  path      int   true  "Workout ID"
//	@Param        setID      path      int   true  "Set ID"
//	@Param        video      formData  file  true  "Video clip (MP4/MOV/WebM)"
//	@Success      201  {object}  api.SetVideo
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Failure      413  {object}  api.APIError
//	@Failure      415  {object}  api.APIError
//	@Failure      503  {object}  api.APIError  "video storage not configured"
//	@Router       /athletes/{id}/workouts/{workoutID}/sets/{setID}/video [post]
func (h *Handlers) UploadSetVideo(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}
	if middleware.IsServiceCredential(r.Context()) {
		WriteError(w, http.StatusForbidden, serviceVideoRefusal)
		return
	}
	_, setID, ok := h.ownedSet(w, r, athleteID)
	if !ok {
		return
	}
	store := h.videoStore()
	if store == nil {
		WriteError(w, http.StatusServiceUnavailable, "form-check video storage is not configured")
		return
	}
	maxBytes := h.videoMaxBytes()
	extendVideoDeadlines(w, true)
	tooLarge := fmt.Sprintf("video too large (max %d MiB)", maxBytes>>20)

	// Cap the whole body and stream the part straight to disk, so a large
	// clip is never buffered in memory or spilled to a temp dir first.
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes+multipartOverhead)
	mr, err := r.MultipartReader()
	if err != nil {
		WriteError(w, http.StatusBadRequest, "multipart form with a 'video' file is required")
		return
	}
	var saved *media.Saved
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			WriteError(w, http.StatusBadRequest, "video file is required")
			return
		}
		if err != nil {
			var mbe *http.MaxBytesError
			if errors.As(err, &mbe) {
				WriteError(w, http.StatusRequestEntityTooLarge, tooLarge)
				return
			}
			WriteError(w, http.StatusBadRequest, "malformed multipart body")
			return
		}
		if part.FormName() != "video" {
			_ = part.Close()
			continue
		}
		saved, err = store.Save(part, strconv.FormatInt(setID, 10))
		_ = part.Close()
		switch {
		case err == nil:
		case errors.Is(err, media.ErrTooLarge):
			WriteError(w, http.StatusRequestEntityTooLarge, tooLarge)
			return
		case errors.Is(err, media.ErrUnsupportedType):
			WriteError(w, http.StatusUnsupportedMediaType, "video must be MP4, MOV or WebM")
			return
		case errors.Is(err, media.ErrEmpty):
			WriteError(w, http.StatusBadRequest, "video file is empty")
			return
		default:
			log.Printf("api: save video for set %d: %v", setID, err)
			WriteError(w, http.StatusInternalServerError, "failed to save video")
			return
		}
		break
	}

	user := middleware.UserFromContext(r.Context())
	video, old, err := models.ReplaceSetVideo(r.Context(), h.DB, setID, models.SetVideoInput{
		FileName:    saved.Name,
		ContentType: saved.ContentType,
		SizeBytes:   saved.Size,
		UploadedBy:  user.ID,
	})
	if err != nil {
		_ = store.Remove(saved.Name)
		log.Printf("api: record video for set %d: %v", setID, err)
		WriteError(w, http.StatusInternalServerError, "failed to save video")
		return
	}
	if old != "" {
		if err := store.Remove(old); err != nil {
			log.Printf("api: remove replaced video %q: %v", old, err)
		}
	}
	WriteJSON(w, http.StatusCreated, SetVideoFromModel(video))
}

// GetSetVideoFile streams a set's form-check clip.
//
//	@Summary      Stream a form-check video
//	@Description  Returns the clip bytes with its sniffed Content-Type. Supports HTTP Range requests, so a video element can seek. Same access as the workout read; the file is never served from a public path.
//	@Tags         Workouts
//	@Produce      video/mp4
//	@Produce      video/quicktime
//	@Produce      video/webm
//	@Param        id         path  int  true  "Athlete ID"
//	@Param        workoutID  path  int  true  "Workout ID"
//	@Param        setID      path  int  true  "Set ID"
//	@Success      200  {file}    file
//	@Success      206  {file}    file
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /athletes/{id}/workouts/{workoutID}/sets/{setID}/video [get]
func (h *Handlers) GetSetVideoFile(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}
	workoutID, setID, ok := h.ownedSet(w, r, athleteID)
	if !ok {
		return
	}
	video, err := models.GetSetVideo(r.Context(), h.DB, setID, workoutID, athleteID)
	if errors.Is(err, models.ErrNotFound) {
		WriteError(w, http.StatusNotFound, "no video on this set")
		return
	}
	if err != nil {
		log.Printf("api: get video for set %d: %v", setID, err)
		WriteError(w, http.StatusInternalServerError, "failed to load video")
		return
	}
	store := h.videoStore()
	if store == nil {
		WriteError(w, http.StatusNotFound, "no video on this set")
		return
	}
	f, err := store.Open(video.FileName)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			log.Printf("api: open video %q: %v", video.FileName, err)
		}
		WriteError(w, http.StatusNotFound, "video file is missing")
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		log.Printf("api: stat video %q: %v", video.FileName, err)
		WriteError(w, http.StatusInternalServerError, "failed to load video")
		return
	}
	ext := video.FileName[strings.LastIndex(video.FileName, "."):]
	w.Header().Set("Content-Type", video.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="form-check-%d%s"`, setID, ext))
	w.Header().Set("Cache-Control", "private, max-age=3600")
	extendVideoDeadlines(w, false)
	http.ServeContent(w, r, "", info.ModTime(), f)
}

// DeleteSetVideo removes a set's form-check clip and its file.
//
//	@Summary      Delete a form-check video
//	@Description  Removes the clip and its coach comment. The set itself is unchanged. Browser session only.
//	@Tags         Workouts
//	@Produce      json
//	@Param        id         path  int  true  "Athlete ID"
//	@Param        workoutID  path  int  true  "Workout ID"
//	@Param        setID      path  int  true  "Set ID"
//	@Success      200  {object}  api.StatusResponse
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /athletes/{id}/workouts/{workoutID}/sets/{setID}/video [delete]
func (h *Handlers) DeleteSetVideo(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}
	if middleware.IsServiceCredential(r.Context()) {
		WriteError(w, http.StatusForbidden, serviceVideoRefusal)
		return
	}
	workoutID, setID, ok := h.ownedSet(w, r, athleteID)
	if !ok {
		return
	}
	name, err := models.DeleteSetVideo(r.Context(), h.DB, setID, workoutID, athleteID)
	if errors.Is(err, models.ErrNotFound) {
		WriteError(w, http.StatusNotFound, "no video on this set")
		return
	}
	if err != nil {
		log.Printf("api: delete video for set %d: %v", setID, err)
		WriteError(w, http.StatusInternalServerError, "failed to delete video")
		return
	}
	h.removeVideoFiles(name)
	WriteJSON(w, http.StatusOK, StatusResponse{Status: "ok"})
}

// CommentSetVideo sets the coach's comment on a set's clip.
//
//	@Summary      Comment on a form-check video
//	@Description  Coach or admin only (for an athlete they can access), browser session only. The comment is review feedback shown with the clip; an empty comment clears it. It never changes the program.
//	@Tags         Workouts
//	@Accept       json
//	@Produce      json
//	@Param        id         path  int                         true  "Athlete ID"
//	@Param        workoutID  path  int                         true  "Workout ID"
//	@Param        setID      path  int                         true  "Set ID"
//	@Param        body       body  api.SetVideoCommentRequest  true  "Comment"
//	@Success      200  {object}  api.SetVideo
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /athletes/{id}/workouts/{workoutID}/sets/{setID}/video/comment [put]
func (h *Handlers) CommentSetVideo(w http.ResponseWriter, r *http.Request) {
	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}
	if middleware.IsServiceCredential(r.Context()) {
		WriteError(w, http.StatusForbidden, serviceVideoRefusal)
		return
	}
	user := middleware.UserFromContext(r.Context())
	if !user.IsCoach && !user.IsAdmin {
		WriteError(w, http.StatusForbidden, "only a coach can comment on a form-check video")
		return
	}
	workoutID, setID, ok := h.ownedSet(w, r, athleteID)
	if !ok {
		return
	}
	var req SetVideoCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Comment = strings.TrimSpace(req.Comment)
	if len(req.Comment) > models.MaxVideoCommentLen {
		WriteValidationError(w, "comment", "must be at most 2000 characters")
		return
	}
	video, err := models.CommentSetVideo(r.Context(), h.DB, setID, workoutID, athleteID, user.ID, req.Comment)
	if errors.Is(err, models.ErrNotFound) {
		WriteError(w, http.StatusNotFound, "no video on this set")
		return
	}
	if err != nil {
		log.Printf("api: comment on video for set %d: %v", setID, err)
		WriteError(w, http.StatusInternalServerError, "failed to save comment")
		return
	}
	WriteJSON(w, http.StatusOK, SetVideoFromModel(video))
}

// removeVideoFiles deletes clip files best-effort after their rows are gone.
// Anything missed is caught by the scheduler's orphan sweep.
func (h *Handlers) removeVideoFiles(names ...string) {
	store := h.videoStore()
	if store == nil {
		return
	}
	for _, n := range names {
		if n == "" {
			continue
		}
		if err := store.Remove(n); err != nil {
			log.Printf("api: remove video %q: %v", n, err)
		}
	}
}
