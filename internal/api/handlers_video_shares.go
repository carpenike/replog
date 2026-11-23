package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// --- Form-check video share links ---
//
// Someone who can see a set's clip (the athlete, their coach or an admin:
// the same athleteAccess rule as the clip routes) can mint a link that lets a
// person with no login watch that one clip until the link expires. The token
// is 32 random bytes, shown once; only its SHA-256 is stored. The public
// routes (the /s/{token} viewer page and /api/share/{token}...) sit outside
// the auth middleware, behind their own per-IP rate limit, and expose only
// the clip, its exercise and its date. Every unusable token (unknown,
// malformed, expired, revoked, sharing disabled) gets the same 404.

// shareNotFoundMessage is the one body every unusable share token receives.
const shareNotFoundMessage = "this link is not available"

// serviceShareRefusal is the 403 a service token receives on a share route.
const serviceShareRefusal = "service token cannot manage video share links"

// shareNow is the clock for share-link expiry. Tests replace it.
var shareNow = time.Now

// shareViewerPath is the viewer URL for a token, relative to the origin.
func shareViewerPath(token string) string { return "/s/" + token }

// shareVideoURL is the public clip URL for a token.
func shareVideoURL(token string) string { return "/api/share/" + token + "/video" }

// clientIP is the caller's address for access logs.
func (h *Handlers) clientIP(r *http.Request) string {
	if h.ClientIP != nil {
		return h.ClientIP(r)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// logShareAccess records one public share-link request. The token is never
// logged; a resolved link is identified by its row id.
func (h *Handlers) logShareAccess(r *http.Request, endpoint string, shareID int64, outcome string) {
	attrs := []any{"endpoint", endpoint, "ip", h.clientIP(r), "outcome", outcome}
	if shareID != 0 {
		attrs = append(attrs, "share_id", shareID)
	}
	slog.Info("video share access", attrs...)
}

// auditShare writes a best-effort audit record of a share-link change. The
// acting identity is the real user when an admin is impersonating.
func (h *Handlers) auditShare(r *http.Request, user *models.User, action, details string) {
	realID := user.ID
	if h.Sessions != nil {
		if id := h.Sessions.GetInt64(r.Context(), "impersonating_real_user_id"); id != 0 {
			realID = id
		}
	}
	target := sql.NullInt64{}
	if realID != user.ID {
		target = sql.NullInt64{Int64: user.ID, Valid: true}
	}
	if err := models.WriteAuditLog(r.Context(), h.DB, realID, target, action, details); err != nil {
		log.Printf("api: audit %s: %v", action, err)
	}
}

// shareableVideo authorizes a share-management request and loads the set's
// clip. It writes 400/403/404 and returns ok=false on any failure.
func (h *Handlers) shareableVideo(w http.ResponseWriter, r *http.Request) (*models.SetVideo, *models.User, bool) {
	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return nil, nil, false
	}
	if middleware.IsServiceCredential(r.Context()) {
		WriteError(w, http.StatusForbidden, serviceShareRefusal)
		return nil, nil, false
	}
	workoutID, setID, ok := h.ownedSet(w, r, athleteID)
	if !ok {
		return nil, nil, false
	}
	video, err := models.GetSetVideo(r.Context(), h.DB, setID, workoutID, athleteID)
	if errors.Is(err, models.ErrNotFound) {
		WriteError(w, http.StatusNotFound, "no video on this set")
		return nil, nil, false
	}
	if err != nil {
		log.Printf("api: get video for set %d: %v", setID, err)
		WriteError(w, http.StatusInternalServerError, "failed to load video")
		return nil, nil, false
	}
	return video, middleware.UserFromContext(r.Context()), true
}

// CreateVideoShare mints a no-login share link to a set's clip.
//
//	@Summary      Create a form-check video share link
//	@Description  Same access as the clip (the athlete, their coach or an admin), browser session only. Returns the link's secret token and viewer URL (/s/{token}) exactly once: only the token's SHA-256 hash is stored, so it cannot be shown again. expires_in_hours is 24, 168 (default) or 720; anything else, including more than 30 days, is a 400. Refused with 403 when REPLOG_VIDEO_SHARING_DISABLED is set. Replacing or deleting the clip revokes all of its links.
//	@Tags         Workouts
//	@Accept       json
//	@Produce      json
//	@Param        id         path  int                          true  "Athlete ID"
//	@Param        workoutID  path  int                          true  "Workout ID"
//	@Param        setID      path  int                          true  "Set ID"
//	@Param        body       body  api.VideoShareCreateRequest  true  "Expiry and optional label"
//	@Success      201  {object}  api.CreatedVideoShare
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /athletes/{id}/workouts/{workoutID}/sets/{setID}/video/shares [post]
func (h *Handlers) CreateVideoShare(w http.ResponseWriter, r *http.Request) {
	video, user, ok := h.shareableVideo(w, r)
	if !ok {
		return
	}
	if h.VideoSharingDisabled {
		WriteError(w, http.StatusForbidden, "video sharing is turned off on this server")
		return
	}
	var req VideoShareCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ExpiresInHours == 0 {
		req.ExpiresInHours = int(models.DefaultVideoShareLifetime / time.Hour)
	}
	lifetime := time.Duration(req.ExpiresInHours) * time.Hour
	if req.ExpiresInHours < 0 || req.ExpiresInHours > int(models.MaxVideoShareLifetime/time.Hour) || !models.ValidVideoShareLifetime(lifetime) {
		WriteValidationError(w, "expires_in_hours", "must be 24, 168 or 720 (at most 30 days)")
		return
	}
	label := strings.TrimSpace(req.Label)
	if utf8.RuneCountInString(label) > models.MaxVideoShareLabelLen {
		WriteValidationError(w, "label", fmt.Sprintf("must be at most %d characters", models.MaxVideoShareLabelLen))
		return
	}
	now := shareNow()
	share, token, err := models.CreateVideoShare(r.Context(), h.DB, video.ID, user.ID, label, lifetime, now)
	if err != nil {
		log.Printf("api: create share for video %d: %v", video.ID, err)
		WriteError(w, http.StatusInternalServerError, "failed to create the link")
		return
	}
	h.auditShare(r, user, "video_share_create", fmt.Sprintf("share=%d video=%d set=%d athlete=%d expires=%s",
		share.ID, video.ID, video.SetID, video.AthleteID, share.ExpiresAt.UTC().Format(time.RFC3339)))
	// The body carries the only copy of the token: keep it out of caches.
	w.Header().Set("Cache-Control", "no-store")
	WriteJSON(w, http.StatusCreated, CreatedVideoShare{
		Share: VideoShareFromModel(share, now),
		Token: token,
		URL:   shareViewerPath(token),
	})
}

// ListVideoShares lists the share links on a set's clip.
//
//	@Summary      List a form-check video's share links
//	@Description  Same access as the clip, browser session only. Active links by default; include_inactive=true adds expired and revoked ones. Tokens are never returned. sharing_enabled is false when REPLOG_VIDEO_SHARING_DISABLED is set.
//	@Tags         Workouts
//	@Produce      json
//	@Param        id                path   int   true   "Athlete ID"
//	@Param        workoutID         path   int   true   "Workout ID"
//	@Param        setID             path   int   true   "Set ID"
//	@Param        include_inactive  query  bool  false  "Include expired and revoked links"
//	@Success      200  {object}  api.VideoShareList
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /athletes/{id}/workouts/{workoutID}/sets/{setID}/video/shares [get]
func (h *Handlers) ListVideoShares(w http.ResponseWriter, r *http.Request) {
	video, _, ok := h.shareableVideo(w, r)
	if !ok {
		return
	}
	includeInactive, _ := strconv.ParseBool(r.URL.Query().Get("include_inactive"))
	now := shareNow()
	shares, err := models.ListVideoShares(r.Context(), h.DB, video.ID, includeInactive, now)
	if err != nil {
		log.Printf("api: list shares for video %d: %v", video.ID, err)
		WriteError(w, http.StatusInternalServerError, "failed to load links")
		return
	}
	out := VideoShareList{SharingEnabled: !h.VideoSharingDisabled, Shares: make([]*VideoShare, 0, len(shares))}
	for _, s := range shares {
		out.Shares = append(out.Shares, VideoShareFromModel(s, now))
	}
	w.Header().Set("Cache-Control", "no-store")
	WriteJSON(w, http.StatusOK, out)
}

// RevokeVideoShare revokes one share link on a set's clip.
//
//	@Summary      Revoke a form-check video share link
//	@Description  Same access as the clip, browser session only. The link stops working at once. Revoking an already revoked link is a no-op that returns it. A link on another clip is a 404.
//	@Tags         Workouts
//	@Produce      json
//	@Param        id         path  int  true  "Athlete ID"
//	@Param        workoutID  path  int  true  "Workout ID"
//	@Param        setID      path  int  true  "Set ID"
//	@Param        shareID    path  int  true  "Share link ID"
//	@Success      200  {object}  api.VideoShare
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Failure      404  {object}  api.APIError
//	@Router       /athletes/{id}/workouts/{workoutID}/sets/{setID}/video/shares/{shareID} [delete]
func (h *Handlers) RevokeVideoShare(w http.ResponseWriter, r *http.Request) {
	video, user, ok := h.shareableVideo(w, r)
	if !ok {
		return
	}
	shareID, err := strconv.ParseInt(r.PathValue("shareID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid share ID")
		return
	}
	now := shareNow()
	share, err := models.RevokeVideoShare(r.Context(), h.DB, shareID, video.ID, user.ID, now)
	if errors.Is(err, models.ErrNotFound) {
		WriteError(w, http.StatusNotFound, "share link not found")
		return
	}
	if err != nil {
		log.Printf("api: revoke share %d: %v", shareID, err)
		WriteError(w, http.StatusInternalServerError, "failed to revoke the link")
		return
	}
	h.auditShare(r, user, "video_share_revoke", fmt.Sprintf("share=%d video=%d set=%d athlete=%d",
		share.ID, video.ID, video.SetID, video.AthleteID))
	WriteJSON(w, http.StatusOK, VideoShareFromModel(share, now))
}

// resolveShare resolves the {token} path value. Every failure, including
// the kill switch, is reported as not found; database errors are logged
// (without the token) and also answered as not found.
func (h *Handlers) resolveShare(r *http.Request) (*models.SharedVideo, bool) {
	if h.VideoSharingDisabled {
		return nil, false
	}
	sv, err := models.ResolveVideoShare(r.Context(), h.DB, r.PathValue("token"), shareNow())
	if err != nil {
		if !errors.Is(err, models.ErrNotFound) {
			log.Printf("api: resolve share link: %v", err)
		}
		return nil, false
	}
	return sv, true
}

// writeShareNotFound is the single 404 every unusable token gets.
func writeShareNotFound(w http.ResponseWriter) {
	WriteError(w, http.StatusNotFound, shareNotFoundMessage)
}

// recordShareAccess stamps the link's last access, best effort.
func (h *Handlers) recordShareAccess(r *http.Request, shareID int64, countView bool) {
	if err := models.RecordVideoShareAccess(r.Context(), h.DB, shareID, countView, shareNow()); err != nil {
		log.Printf("api: record share %d access: %v", shareID, err)
	}
}

// GetSharedVideo returns what a share-link viewer shows.
//
//	@Summary      Open a form-check video share link
//	@Description  Public, no login. Returns only the exercise name, the workout date, when the link expires and the clip URL; no athlete name or account data. Unknown, malformed, expired and revoked tokens (and every token when sharing is disabled) get the same 404. Each call counts as one view. Rate limited per client IP; responses are never cached.
//	@Tags         Share
//	@Produce      json
//	@Param        token  path  string  true  "Share token"
//	@Success      200  {object}  api.SharedVideo
//	@Failure      404  {object}  api.APIError
//	@Failure      429  {string}  string  "rate limited"
//	@Router       /share/{token} [get]
func (h *Handlers) GetSharedVideo(w http.ResponseWriter, r *http.Request) {
	middleware.SetShareHeaders(w.Header())
	sv, ok := h.resolveShare(r)
	if !ok {
		h.logShareAccess(r, "meta", 0, "not_found")
		writeShareNotFound(w)
		return
	}
	h.recordShareAccess(r, sv.ShareID, true)
	h.logShareAccess(r, "meta", sv.ShareID, "ok")
	WriteJSON(w, http.StatusOK, SharedVideo{
		ExerciseName: sv.ExerciseName,
		Date:         sv.Date,
		ExpiresAt:    fmtTime(sv.ExpiresAt),
		ContentType:  sv.ContentType,
		VideoURL:     shareVideoURL(r.PathValue("token")),
	})
}

// GetSharedVideoFile streams the clip behind a share link.
//
//	@Summary      Stream a shared form-check video
//	@Description  Public, no login. Streams the one clip the link opens, with its sniffed Content-Type and HTTP Range support. The file is found only through the stored clip row, never from the token. Unusable tokens get the same 404 as the metadata route. Responses are private, no-store and noindex.
//	@Tags         Share
//	@Produce      video/mp4
//	@Produce      video/quicktime
//	@Produce      video/webm
//	@Param        token  path  string  true  "Share token"
//	@Success      200  {file}    file
//	@Success      206  {file}    file
//	@Failure      404  {object}  api.APIError
//	@Failure      429  {string}  string  "rate limited"
//	@Router       /share/{token}/video [get]
func (h *Handlers) GetSharedVideoFile(w http.ResponseWriter, r *http.Request) {
	middleware.SetShareHeaders(w.Header())
	sv, ok := h.resolveShare(r)
	store := h.videoStore()
	if !ok || store == nil {
		h.logShareAccess(r, "video", 0, "not_found")
		writeShareNotFound(w)
		return
	}
	f, err := store.Open(sv.FileName)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			log.Printf("api: open shared video %d: %v", sv.VideoID, err)
		}
		h.logShareAccess(r, "video", sv.ShareID, "file_missing")
		writeShareNotFound(w)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		log.Printf("api: stat shared video %d: %v", sv.VideoID, err)
		WriteError(w, http.StatusInternalServerError, "failed to load video")
		return
	}
	// A plain GET (a download, or the first request of a player that does
	// not use ranges) counts as a view; range requests while playing and
	// seeking only refresh last_accessed_at.
	h.recordShareAccess(r, sv.ShareID, r.Header.Get("Range") == "")
	h.logShareAccess(r, "video", sv.ShareID, "ok")
	ext := ""
	if i := strings.LastIndex(sv.FileName, "."); i >= 0 {
		ext = sv.FileName[i:]
	}
	w.Header().Set("Content-Type", sv.ContentType)
	w.Header().Set("Content-Disposition", `inline; filename="form-check`+ext+`"`)
	extendVideoDeadlines(w, false)
	http.ServeContent(w, r, "", info.ModTime(), f)
}

// SharePage serves the SPA shell for the /s/{token} viewer page with the
// share headers: 200 for a usable token, 404 (same body) otherwise. The SPA
// then loads the clip through GetSharedVideo; nothing here is cached by the
// browser or the service worker. index is the built index.html; nil serves
// a plain message (frontend not embedded, as in dev).
func (h *Handlers) SharePage(index []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		middleware.SetShareHeaders(w.Header())
		sv, ok := h.resolveShare(r)
		status := http.StatusNotFound
		if ok {
			status = http.StatusOK
			h.logShareAccess(r, "page", sv.ShareID, "ok")
		} else {
			h.logShareAccess(r, "page", 0, "not_found")
		}
		if index == nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(status)
			_, _ = w.Write([]byte("Frontend not available\n"))
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(status)
		if r.Method != http.MethodHead {
			_, _ = w.Write(index)
		}
	}
}
