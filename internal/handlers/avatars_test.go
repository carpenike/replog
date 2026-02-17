package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

func TestAvatarUpload(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)

	avatarDir := t.TempDir()
	h := &Avatars{
		DB:        db,
		Templates: tc,
		AvatarDir: avatarDir,
	}

	user, _ := models.CreateUser(db, "avataruser", "", "password123", "", false, false, sql.NullInt64{})

	t.Run("successful upload", func(t *testing.T) {
		body, contentType := createMultipartFile(t, "avatar", "test.png", createTestPNG(t))

		req := httptest.NewRequest(http.MethodPost, "/avatars/upload", body)
		req.Header.Set("Content-Type", contentType)
		ctx := context.WithValue(req.Context(), middleware.UserContextKey, user)
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		h.Upload(rr, req)

		if rr.Code != http.StatusSeeOther {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusSeeOther)
		}

		// Verify avatar was saved in DB.
		updated, err := models.GetUserByID(db, user.ID)
		if err != nil {
			t.Fatalf("get user: %v", err)
		}
		if !updated.HasAvatar() {
			t.Error("expected user to have avatar after upload")
		}

		// Verify file exists on disk.
		avatarPath := filepath.Join(avatarDir, updated.AvatarPath.String)
		if _, err := os.Stat(avatarPath); os.IsNotExist(err) {
			t.Error("avatar file does not exist on disk")
		}
	})

	t.Run("replaces old avatar", func(t *testing.T) {
		// Get current avatar path.
		before, _ := models.GetUserByID(db, user.ID)
		oldFile := before.AvatarPath.String

		body, contentType := createMultipartFile(t, "avatar", "test2.png", createTestPNG(t))

		req := httptest.NewRequest(http.MethodPost, "/avatars/upload", body)
		req.Header.Set("Content-Type", contentType)
		// Re-fetch user to get current avatar path for deletion.
		currentUser, _ := models.GetUserByID(db, user.ID)
		ctx := context.WithValue(req.Context(), middleware.UserContextKey, currentUser)
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		h.Upload(rr, req)

		if rr.Code != http.StatusSeeOther {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusSeeOther)
		}

		// Old file should be deleted.
		if _, err := os.Stat(filepath.Join(avatarDir, oldFile)); !os.IsNotExist(err) {
			t.Error("old avatar file should have been deleted")
		}
	})

	t.Run("rejects non-image", func(t *testing.T) {
		body, contentType := createMultipartFile(t, "avatar", "test.txt", []byte("not an image"))

		req := httptest.NewRequest(http.MethodPost, "/avatars/upload", body)
		req.Header.Set("Content-Type", contentType)
		ctx := context.WithValue(req.Context(), middleware.UserContextKey, user)
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		h.Upload(rr, req)

		if rr.Code != http.StatusUnprocessableEntity {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusUnprocessableEntity)
		}
	})
}

func TestAvatarDelete(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)

	avatarDir := t.TempDir()
	h := &Avatars{
		DB:        db,
		Templates: tc,
		AvatarDir: avatarDir,
	}

	user, _ := models.CreateUser(db, "delavatar", "", "password123", "", false, false, sql.NullInt64{})

	// Upload an avatar first.
	avatarFile := filepath.Join(avatarDir, "test_avatar.png")
	os.WriteFile(avatarFile, createTestPNG(t), 0o644)
	models.UpdateAvatarPath(db, user.ID, sql.NullString{String: "test_avatar.png", Valid: true})
	user, _ = models.GetUserByID(db, user.ID) // refresh

	req := httptest.NewRequest(http.MethodPost, "/avatars/delete", nil)
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, user)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusSeeOther)
	}

	// Verify avatar cleared in DB.
	updated, _ := models.GetUserByID(db, user.ID)
	if updated.HasAvatar() {
		t.Error("expected avatar to be cleared after delete")
	}

	// Verify file deleted from disk.
	if _, err := os.Stat(avatarFile); !os.IsNotExist(err) {
		t.Error("avatar file should have been deleted from disk")
	}
}

func TestAvatarServe(t *testing.T) {
	avatarDir := t.TempDir()
	h := &Avatars{
		AvatarDir: avatarDir,
	}

	// Write a test file.
	testData := createTestPNG(t)
	os.WriteFile(filepath.Join(avatarDir, "test.png"), testData, 0o644)

	t.Run("existing file", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/avatars/test.png", nil)
		req.SetPathValue("filename", "test.png")

		rr := httptest.NewRecorder()
		h.Serve(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
		}
	})

	t.Run("non-existent file", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/avatars/missing.png", nil)
		req.SetPathValue("filename", "missing.png")

		rr := httptest.NewRecorder()
		h.Serve(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
	})

	t.Run("directory traversal blocked", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/avatars/../../../etc/passwd", nil)
		req.SetPathValue("filename", "../../../etc/passwd")

		rr := httptest.NewRecorder()
		h.Serve(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
	})
}

// createTestPNG generates a small valid PNG image for testing.
func createTestPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode test PNG: %v", err)
	}
	return buf.Bytes()
}

// createMultipartFile creates a multipart form body with a file field.
func createMultipartFile(t *testing.T, fieldName, fileName string, content []byte) (io.Reader, string) {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile(fieldName, fileName)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	writer.Close()

	return &buf, writer.FormDataContentType()
}
