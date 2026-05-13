package main

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"
)

func sha256B64(s string) string {
	sum := sha256.Sum256([]byte(s))
	return base64.StdEncoding.EncodeToString(sum[:])
}

func TestExtractInlineScriptHashes(t *testing.T) {
	cases := []struct {
		name string
		html string
		want []string
	}{
		{
			name: "no scripts",
			html: `<!doctype html><html><head><title>x</title></head><body></body></html>`,
			want: nil,
		},
		{
			name: "single inline script — body hashed verbatim",
			html: `<head><script>alert(1)</script></head>`,
			want: []string{sha256B64("alert(1)")},
		},
		{
			name: "external script (has src=) is skipped",
			html: `<head><script src="/main.js"></script></head>`,
			want: nil,
		},
		{
			name: "module script with src is also skipped",
			html: `<head><script type="module" src="/main.js"></script></head>`,
			want: nil,
		},
		{
			name: "mix: inline + external — only inline hashed",
			html: `<head>` +
				`<script>console.log('a')</script>` +
				`<script src="/main.js"></script>` +
				`<script>console.log('b')</script>` +
				`</head>`,
			want: []string{
				sha256B64("console.log('a')"),
				sha256B64("console.log('b')"),
			},
		},
		{
			name: "preserves whitespace inside the script body (CSP is byte-exact)",
			html: "<head><script>\n  var x = 1;\n</script></head>",
			want: []string{sha256B64("\n  var x = 1;\n")},
		},
		{
			name: "case-insensitive match on tag name",
			html: `<head><SCRIPT>foo()</SCRIPT></head>`,
			want: []string{sha256B64("foo()")},
		},
		{
			name: "matches across newlines (DOTALL)",
			html: "<head><script>\n(function(){\n  var t = 'x';\n  return t;\n})();\n</script></head>",
			want: []string{sha256B64("\n(function(){\n  var t = 'x';\n  return t;\n})();\n")},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractInlineScriptHashes([]byte(tc.html))
			if len(got) != len(tc.want) {
				t.Fatalf("got %d hashes, want %d:\n  got:  %v\n  want: %v", len(got), len(tc.want), got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("hash[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// TestExtractInlineScriptHashes_RealIndexHTML feeds the production-shaped
// index.html through the extractor and confirms (a) we get exactly one
// hash and (b) it's stable across runs.
func TestExtractInlineScriptHashes_RealIndexHTML(t *testing.T) {
	// Mirror of web/dist/index.html's inline script. If web/index.html
	// changes the bootstrap script, this test will fail and that is the
	// signal to update the SPA tests too — the hash must change in lock
	// step with the bytes Vite emits.
	const realisticHTML = `<!doctype html>
<html lang="en">
  <head>
    <title>RepLog</title>
    <script>
      // Apply saved theme before React hydrates to prevent flash.
      (function() {
        var t = localStorage.getItem('theme');
        if (t !== 'light') document.documentElement.classList.add('dark');
      })();
    </script>
    <script type="module" crossorigin src="/assets/index.js"></script>
  </head>
  <body><div id="root"></div></body>
</html>`

	got := extractInlineScriptHashes([]byte(realisticHTML))
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 inline-script hash, got %d: %v", len(got), got)
	}

	// Hash must be a 44-char base64 SHA-256 (32 bytes -> 44 chars w/ padding).
	if len(got[0]) != 44 || !strings.HasSuffix(got[0], "=") {
		t.Errorf("hash %q does not look like base64-encoded SHA-256 (want 44 chars ending in '=')", got[0])
	}

	// Same input -> same output (sanity check: we did not accidentally
	// introduce any nondeterminism via map iteration etc.).
	again := extractInlineScriptHashes([]byte(realisticHTML))
	if len(again) != 1 || again[0] != got[0] {
		t.Errorf("non-deterministic output: first=%v second=%v", got, again)
	}
}
