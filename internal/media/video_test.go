package media

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ftyp builds the start of an ISO base media file with the given major brand.
func ftyp(brand string) []byte {
	b := []byte{0, 0, 0, 0x18}
	b = append(b, []byte("ftyp")...)
	b = append(b, []byte(brand)...)
	b = append(b, 0, 0, 0, 0)
	b = append(b, []byte("isommp41")...)
	return b
}

// webmHead is an EBML header whose DocType is webm.
func webmHead() []byte {
	b := []byte{0x1A, 0x45, 0xDF, 0xA3, 0x9F, 0x42, 0x86, 0x81, 0x01, 0x42, 0x82, 0x84}
	return append(b, []byte("webm")...)
}

func TestSniffVideo(t *testing.T) {
	cases := []struct {
		name string
		head []byte
		want string
	}{
		{"mp4 isom", ftyp("isom"), TypeMP4},
		{"mp4 mp42", ftyp("mp42"), TypeMP4},
		{"iphone mov", ftyp("qt  "), TypeQuickTime},
		{"webm", webmHead(), TypeWebM},
		{"heic image", ftyp("heic"), ""},
		{"m4a audio", ftyp("M4A "), ""},
		{"matroska not webm", append([]byte{0x1A, 0x45, 0xDF, 0xA3, 0x42, 0x82}, []byte("matroska")...), ""},
		{"jpeg", []byte{0xFF, 0xD8, 0xFF, 0xE0, 0, 0, 0, 0, 0, 0, 0, 0}, ""},
		{"html", []byte("<html><script>alert(1)</script>"), ""},
		{"short", []byte{0, 0}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, _, ok := SniffVideo(c.head)
			if got != c.want || ok != (c.want != "") {
				t.Fatalf("SniffVideo = %q,%v; want %q", got, ok, c.want)
			}
		})
	}
}

func TestStoreSaveOpenRemove(t *testing.T) {
	s := &Store{Dir: t.TempDir(), MaxBytes: 4096}
	body := append(ftyp("qt  "), bytes.Repeat([]byte{7}, 2000)...)
	saved, err := s.Save(bytes.NewReader(body), "42")
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if saved.ContentType != TypeQuickTime || saved.Size != int64(len(body)) || filepath.Ext(saved.Name) != ".mov" {
		t.Fatalf("saved = %+v", saved)
	}
	f, err := s.Open(saved.Name)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	got := new(bytes.Buffer)
	_, _ = got.ReadFrom(f)
	f.Close()
	if !bytes.Equal(got.Bytes(), body) {
		t.Fatal("stored bytes differ from upload")
	}
	if err := s.Remove(saved.Name); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if err := s.Remove(saved.Name); err != nil {
		t.Fatalf("Remove of a missing file should be a no-op: %v", err)
	}
}

func TestStoreSaveRejects(t *testing.T) {
	dir := t.TempDir()
	s := &Store{Dir: dir, MaxBytes: 1024}

	if _, err := s.Save(bytes.NewReader(nil), "1"); !errors.Is(err, ErrEmpty) {
		t.Fatalf("empty: err = %v", err)
	}
	if _, err := s.Save(bytes.NewReader([]byte("<svg onload=alert(1)>")), "1"); !errors.Is(err, ErrUnsupportedType) {
		t.Fatalf("svg: err = %v", err)
	}
	big := append(ftyp("isom"), bytes.Repeat([]byte{1}, 2048)...)
	if _, err := s.Save(bytes.NewReader(big), "1"); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("too large: err = %v", err)
	}
	// Nothing is left behind by a refused upload, not even the temp file.
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatalf("refused uploads left %d file(s) behind", len(entries))
	}
}

func TestStorePathTraversal(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	for _, name := range []string{"../secret", "a/b.mp4", "", ".upload-1"} {
		if _, err := s.Open(name); !errors.Is(err, ErrBadName) {
			t.Errorf("Open(%q) err = %v, want ErrBadName", name, err)
		}
	}
}

func TestSweepOrphans(t *testing.T) {
	dir := t.TempDir()
	s := &Store{Dir: dir}
	old := time.Now().Add(-3 * time.Hour)
	for _, name := range []string{"kept.mp4", "orphan.mp4", "fresh.mp4"} {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		if name != "fresh.mp4" {
			_ = os.Chtimes(p, old, old)
		}
	}
	keep := func(n string) bool { return n == "kept.mp4" }
	n, err := s.SweepOrphans(keep, time.Hour, time.Now())
	if err != nil || n != 1 {
		t.Fatalf("SweepOrphans = %d, %v; want 1 removed", n, err)
	}
	for name, want := range map[string]bool{"kept.mp4": true, "orphan.mp4": false, "fresh.mp4": true} {
		_, err := os.Stat(filepath.Join(dir, name))
		if (err == nil) != want {
			t.Errorf("%s exists=%v, want %v", name, err == nil, want)
		}
	}
	if n, err := (&Store{Dir: filepath.Join(dir, "missing")}).SweepOrphans(keep, 0, time.Now()); n != 0 || err != nil {
		t.Fatalf("missing dir: %d, %v", n, err)
	}
}
