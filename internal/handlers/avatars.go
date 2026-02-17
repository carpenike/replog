package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// maxAvatarSize is the maximum allowed avatar file size (2 MB).
const maxAvatarSize = 2 << 20

// allowedAvatarTypes maps MIME types to file extensions for allowed avatar formats.
var allowedAvatarTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

// Avatars handles avatar upload, deletion, and serving.
type Avatars struct {
	DB        *sql.DB
	Templates TemplateCache
	AvatarDir string // Filesystem directory where avatar files are stored.
}

// Upload handles avatar file upload for the current user.
func (h *Avatars) Upload(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())

	// Limit request body size.
	r.Body = http.MaxBytesReader(w, r.Body, maxAvatarSize+1024) // extra for form overhead

	if err := r.ParseMultipartForm(maxAvatarSize); err != nil {
		log.Printf("handlers: parse avatar upload: %v", err)
		h.renderPrefsWithError(w, r, "File too large. Maximum size is 2 MB.", user.ID)
		return
	}

	file, header, err := r.FormFile("avatar")
	if err != nil {
		h.renderPrefsWithError(w, r, "Please select an image file.", user.ID)
		return
	}
	defer file.Close()

	// Validate file size.
	if header.Size > maxAvatarSize {
		h.renderPrefsWithError(w, r, "File too large. Maximum size is 2 MB.", user.ID)
		return
	}

	// Detect content type from file contents (more reliable than header).
	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		log.Printf("handlers: read avatar for detection: %v", err)
		h.renderPrefsWithError(w, r, "Failed to read uploaded file.", user.ID)
		return
	}
	contentType := http.DetectContentType(buf[:n])
	ext, ok := allowedAvatarTypes[contentType]
	if !ok {
		h.renderPrefsWithError(w, r, "Unsupported file type. Use JPEG, PNG, GIF, or WebP.", user.ID)
		return
	}

	// Seek back to beginning for the copy.
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		log.Printf("handlers: seek avatar file: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Generate a unique filename.
	randBytes := make([]byte, 16)
	if _, err := rand.Read(randBytes); err != nil {
		log.Printf("handlers: generate avatar filename: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	filename := fmt.Sprintf("%d_%s%s", user.ID, hex.EncodeToString(randBytes), ext)

	// Ensure avatar directory exists.
	if err := os.MkdirAll(h.AvatarDir, 0o755); err != nil {
		log.Printf("handlers: create avatar dir: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Write file to disk.
	destPath := filepath.Join(h.AvatarDir, filename)
	dst, err := os.Create(destPath)
	if err != nil {
		log.Printf("handlers: create avatar file: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		log.Printf("handlers: write avatar file: %v", err)
		os.Remove(destPath)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Delete old avatar file if one exists.
	if user.HasAvatar() {
		oldPath := filepath.Join(h.AvatarDir, user.AvatarPath.String)
		os.Remove(oldPath) // best-effort cleanup
	}

	// Update database.
	if err := models.UpdateAvatarPath(h.DB, user.ID, sql.NullString{String: filename, Valid: true}); err != nil {
		log.Printf("handlers: update avatar path for user %d: %v", user.ID, err)
		os.Remove(destPath)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/preferences", http.StatusSeeOther)
}

// Delete removes the current user's avatar.
func (h *Avatars) Delete(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())

	if user.HasAvatar() {
		oldPath := filepath.Join(h.AvatarDir, user.AvatarPath.String)
		os.Remove(oldPath) // best-effort cleanup
	}

	if err := models.UpdateAvatarPath(h.DB, user.ID, sql.NullString{}); err != nil {
		if !errors.Is(err, models.ErrNotFound) {
			log.Printf("handlers: clear avatar for user %d: %v", user.ID, err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}

	http.Redirect(w, r, "/preferences", http.StatusSeeOther)
}

// Serve serves avatar image files from the avatar directory.
func (h *Avatars) Serve(w http.ResponseWriter, r *http.Request) {
	filename := r.PathValue("filename")

	// Sanitize filename — prevent directory traversal.
	filename = filepath.Base(filename)
	if filename == "." || filename == "/" || strings.Contains(filename, "..") {
		http.NotFound(w, r)
		return
	}

	filePath := filepath.Join(h.AvatarDir, filename)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeFile(w, r, filePath)
}

// renderPrefsWithError re-renders the preferences form with an error message.
func (h *Avatars) renderPrefsWithError(w http.ResponseWriter, r *http.Request, msg string, userID int64) {
	prefs, _ := models.GetUserPreferences(h.DB, userID)
	passkeys, _ := models.ListWebAuthnCredentialsByUser(h.DB, userID)
	user, _ := models.GetUserByID(h.DB, userID)
	data := map[string]any{
		"Error":           msg,
		"EditPrefs":       prefs,
		"WeightUnits":     models.ValidWeightUnits,
		"DateFormats":     models.ValidDateFormats,
		"CommonTimezones": commonTimezones,
		"Passkeys":        passkeys,
		"UserID":          userID,
		"AvatarUser":      user,
	}
	w.WriteHeader(http.StatusUnprocessableEntity)
	if err := h.Templates.Render(w, r, "preferences_form.html", data); err != nil {
		log.Printf("handlers: render preferences form with avatar error: %v", err)
	}
}
