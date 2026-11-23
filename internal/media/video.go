// Package media stores form-check video clips (gap 12) on local disk.
//
// Clips are kept under one directory (REPLOG_VIDEO_DIR), never the public
// avatar directory, and are only ever served back through an authenticated
// API route. File names are random and chosen here; a client's filename or
// claimed Content-Type is never trusted. The container type is sniffed from
// the first bytes of the upload.
package media

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DefaultMaxVideoBytes is the default cap on one clip: enough for roughly a
// minute of 1080p phone video, which is longer than a form check needs.
const DefaultMaxVideoBytes int64 = 100 << 20 // 100 MiB

// sniffLen is how many leading bytes are read to identify the container.
const sniffLen = 512

var (
	// ErrUnsupportedType is returned when the upload is not MP4, QuickTime or WebM.
	ErrUnsupportedType = errors.New("media: video must be MP4, MOV or WebM")
	// ErrTooLarge is returned when the upload exceeds the store's cap.
	ErrTooLarge = errors.New("media: video is too large")
	// ErrEmpty is returned for a zero-length upload.
	ErrEmpty = errors.New("media: video is empty")
	// ErrBadName is returned for a stored name that is not a bare file name.
	ErrBadName = errors.New("media: invalid file name")
)

// Content types a stored clip can have. They match the CHECK constraint on
// workout_set_videos.content_type.
const (
	TypeMP4       = "video/mp4"
	TypeQuickTime = "video/quicktime"
	TypeWebM      = "video/webm"
)

var extByType = map[string]string{
	TypeMP4:       ".mp4",
	TypeQuickTime: ".mov",
	TypeWebM:      ".webm",
}

// mp4Brands are ISO base-media major brands accepted as MP4 video. Image
// (heic, avif) and audio-only (M4A) brands are not on the list.
var mp4BrandPrefixes = []string{"iso", "mp4", "avc1", "M4V", "MSNV", "3gp", "3g2", "mmp4", "dash", "f4v", "hvc1"}

// SniffVideo identifies a clip from its first bytes. It recognises ISO base
// media files (an ftyp box: MP4, or QuickTime with the "qt  " brand, which
// is what an iPhone records) and WebM (an EBML header with the webm
// DocType). It returns the content type and the extension to store under.
func SniffVideo(head []byte) (contentType, ext string, ok bool) {
	if len(head) >= 12 && string(head[4:8]) == "ftyp" {
		brand := string(head[8:12])
		if brand == "qt  " {
			return TypeQuickTime, extByType[TypeQuickTime], true
		}
		for _, p := range mp4BrandPrefixes {
			if strings.HasPrefix(brand, p) {
				return TypeMP4, extByType[TypeMP4], true
			}
		}
		return "", "", false
	}
	if len(head) >= 4 && bytes.Equal(head[:4], []byte{0x1A, 0x45, 0xDF, 0xA3}) {
		// The DocType element (0x4282) follows within the EBML header.
		window := head
		if len(window) > 64 {
			window = window[:64]
		}
		if bytes.Contains(window, []byte("webm")) {
			return TypeWebM, extByType[TypeWebM], true
		}
	}
	return "", "", false
}

// Store saves and opens clips in Dir. MaxBytes caps one clip; zero means
// DefaultMaxVideoBytes.
type Store struct {
	Dir      string
	MaxBytes int64
}

func (s *Store) maxBytes() int64 {
	if s.MaxBytes <= 0 {
		return DefaultMaxVideoBytes
	}
	return s.MaxBytes
}

// Saved describes a clip written by Save.
type Saved struct {
	Name        string
	ContentType string
	Size        int64
}

// Save streams r to a new file in Dir. The container type is sniffed from
// the first bytes; anything other than MP4, QuickTime or WebM is refused
// before a file is created. prefix (for example the set ID) starts the
// random file name. A partial file is removed on any error, including
// exceeding MaxBytes.
func (s *Store) Save(r io.Reader, prefix string) (*Saved, error) {
	head := make([]byte, sniffLen)
	n, err := io.ReadFull(r, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("media: read upload: %w", err)
	}
	if n == 0 {
		return nil, ErrEmpty
	}
	head = head[:n]
	ctype, ext, ok := SniffVideo(head)
	if !ok {
		return nil, ErrUnsupportedType
	}

	if err := os.MkdirAll(s.Dir, 0o750); err != nil {
		return nil, fmt.Errorf("media: create video dir: %w", err)
	}
	suffix := make([]byte, 12)
	if _, err := rand.Read(suffix); err != nil {
		return nil, fmt.Errorf("media: random name: %w", err)
	}
	name := prefix + "_" + hex.EncodeToString(suffix) + ext

	tmp, err := os.CreateTemp(s.Dir, ".upload-*")
	if err != nil {
		return nil, fmt.Errorf("media: create temp file: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}

	limit := s.maxBytes()
	if int64(n) > limit {
		cleanup()
		return nil, ErrTooLarge
	}
	if _, err := tmp.Write(head); err != nil {
		cleanup()
		return nil, fmt.Errorf("media: write video: %w", err)
	}
	// Copy at most one byte past the cap so an oversize upload is detected
	// without reading the rest of it.
	copied, err := io.Copy(tmp, io.LimitReader(r, limit-int64(n)+1))
	if err != nil {
		cleanup()
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			return nil, ErrTooLarge
		}
		return nil, fmt.Errorf("media: write video: %w", err)
	}
	size := int64(n) + copied
	if size > limit {
		cleanup()
		return nil, ErrTooLarge
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return nil, fmt.Errorf("media: close video: %w", err)
	}
	if err := os.Rename(tmpName, filepath.Join(s.Dir, name)); err != nil {
		_ = os.Remove(tmpName)
		return nil, fmt.Errorf("media: store video: %w", err)
	}
	return &Saved{Name: name, ContentType: ctype, Size: size}, nil
}

// path resolves a stored name inside Dir, refusing anything that is not a
// bare file name.
func (s *Store) path(name string) (string, error) {
	if name == "" || name != filepath.Base(name) || strings.HasPrefix(name, ".") {
		return "", ErrBadName
	}
	return filepath.Join(s.Dir, name), nil
}

// Open opens a stored clip for reading.
func (s *Store) Open(name string) (*os.File, error) {
	p, err := s.path(name)
	if err != nil {
		return nil, err
	}
	return os.Open(p)
}

// Remove deletes a stored clip. A missing file is not an error.
func (s *Store) Remove(name string) error {
	p, err := s.path(name)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("media: remove video: %w", err)
	}
	return nil
}

// SweepOrphans removes files in Dir that keep(name) does not claim and that
// are older than minAge, so an upload in progress (or a row about to be
// written) is never swept. It returns how many files it removed. A missing
// Dir is not an error.
func (s *Store) SweepOrphans(keep func(name string) bool, minAge time.Duration, now time.Time) (int, error) {
	entries, err := os.ReadDir(s.Dir)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("media: read video dir: %w", err)
	}
	removed := 0
	for _, e := range entries {
		if !e.Type().IsRegular() {
			continue
		}
		name := e.Name()
		if keep(name) {
			continue
		}
		info, err := e.Info()
		if err != nil || now.Sub(info.ModTime()) < minAge {
			continue
		}
		if err := os.Remove(filepath.Join(s.Dir, name)); err == nil {
			removed++
		}
	}
	return removed, nil
}
