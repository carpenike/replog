package api

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// --- Avatar Upload/Delete ---

// avatarMaxBytes is the hard cap on the avatar request body. Larger requests
// are rejected before any disk I/O.
const avatarMaxBytes = 2 << 20 // 2 MiB

// avatarAllowedTypes maps MIME types we accept for avatars to the canonical
// extension we will write to disk. The extension is chosen server-side from
// the sniffed content, never from the client-supplied filename, to prevent
// a client from uploading e.g. an SVG (XSS sink) or HTML file with a .jpg
// name and having the FileServer hand it back with a guessed Content-Type.
var avatarAllowedTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

// AvatarUpload handles avatar file upload for the authenticated user.
// AvatarUpload handles avatar file upload for the authenticated user.
//
//	@Summary      Upload avatar
//	@Description  Multipart upload; field name is 'avatar'. Server sniffs the file's content type and accepts only JPEG, PNG, WebP, or GIF. Returns the new avatar URL.
//	@Tags         Avatars
//	@Accept       multipart/form-data
//	@Produce      json
//	@Param        avatar formData file true "Avatar image file (JPEG/PNG/WebP/GIF, ≤ 2 MiB)"
//	@Success      200  {object}  map[string]string  "e.g. {\"avatar_url\": \"/avatars/abc.jpg\"}"
//	@Failure      400  {object}  api.APIError
//	@Failure      413  {object}  api.APIError
//	@Failure      415  {object}  api.APIError
//	@Router       /avatars/upload [post]
func (h *Handlers) AvatarUpload(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())

	// Cap the *entire* request body, not just the in-memory portion of
	// ParseMultipartForm. Otherwise, a client streaming a multi-GB body
	// would spill the excess to /tmp before being rejected.
	r.Body = http.MaxBytesReader(w, r.Body, avatarMaxBytes)

	if err := r.ParseMultipartForm(avatarMaxBytes); err != nil {
		WriteError(w, http.StatusRequestEntityTooLarge, "file too large (max 2 MiB)")
		return
	}

	file, _, err := r.FormFile("avatar")
	if err != nil {
		WriteError(w, http.StatusBadRequest, "avatar file is required")
		return
	}
	defer file.Close()

	// Sniff the first 512 bytes — net/http.DetectContentType requires
	// exactly that many bytes (or all of the file, whichever is smaller).
	sniffBuf := make([]byte, 512)
	n, err := io.ReadFull(file, sniffBuf)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		WriteError(w, http.StatusBadRequest, "failed to read upload")
		return
	}
	contentType := http.DetectContentType(sniffBuf[:n])
	ext, ok := avatarAllowedTypes[contentType]
	if !ok {
		WriteError(w, http.StatusUnsupportedMediaType, "avatar must be JPEG, PNG, WebP, or GIF")
		return
	}

	// Random suffix so avatar URLs are not enumerable from a numeric user
	// ID. The /avatars/{filename} route is public.
	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		log.Printf("api: avatar random suffix: %v", err)
		WriteError(w, http.StatusInternalServerError, "failed to save avatar")
		return
	}
	filename := fmt.Sprintf("%d_%s%s", user.ID, hex.EncodeToString(suffix), ext)
	fullPath := filepath.Join(h.AvatarDir, filename)

	if err := os.MkdirAll(h.AvatarDir, 0750); err != nil {
		log.Printf("api: create avatar dir: %v", err)
		WriteError(w, http.StatusInternalServerError, "failed to save avatar")
		return
	}

	dst, err := os.Create(fullPath)
	if err != nil {
		log.Printf("api: create avatar file: %v", err)
		WriteError(w, http.StatusInternalServerError, "failed to save avatar")
		return
	}
	defer dst.Close()

	// Write the sniffed prefix first, then stream the rest.
	if _, err := dst.Write(sniffBuf[:n]); err != nil {
		log.Printf("api: write avatar prefix: %v", err)
		WriteError(w, http.StatusInternalServerError, "failed to save avatar")
		return
	}
	if _, err := io.Copy(dst, file); err != nil {
		// MaxBytesReader returns an error here if the body exceeded the cap.
		log.Printf("api: write avatar file: %v", err)
		WriteError(w, http.StatusRequestEntityTooLarge, "file too large (max 2 MiB)")
		return
	}

	// Best-effort cleanup of the previous avatar so we don't leak files
	// on disk every time the user uploads a new one.
	if user.HasAvatar() {
		if old := filepath.Join(h.AvatarDir, user.AvatarPath.String); old != fullPath {
			_ = os.Remove(old)
		}
	}

	if err := models.UpdateAvatarPath(h.DB, user.ID, sql.NullString{String: filename, Valid: true}); err != nil {
		log.Printf("api: update avatar path for user %d: %v", user.ID, err)
		WriteError(w, http.StatusInternalServerError, "failed to update avatar")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"avatar_url": "/avatars/" + filename})
}

// AvatarDelete deletes the authenticated user's avatar.
// AvatarDelete deletes the authenticated user's avatar file.
//
//	@Summary      Delete avatar
//	@Tags         Avatars
//	@Produce      json
//	@Success      200  {object}  api.StatusResponse
//	@Router       /avatars/delete [post]
func (h *Handlers) AvatarDelete(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())

	if user.HasAvatar() {
		fullPath := filepath.Join(h.AvatarDir, user.AvatarPath.String)
		os.Remove(fullPath)
	}

	if err := models.UpdateAvatarPath(h.DB, user.ID, sql.NullString{}); err != nil {
		log.Printf("api: delete avatar for user %d: %v", user.ID, err)
		WriteError(w, http.StatusInternalServerError, "failed to delete avatar")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
