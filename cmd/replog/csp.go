package main

import (
	"crypto/sha256"
	"encoding/base64"
	"io/fs"
	"log"
	"regexp"

	"github.com/carpenike/replog/internal/middleware"
	frontend "github.com/carpenike/replog/web"
)

// inlineScriptRe matches a complete inline <script>...</script> block —
// the body of which we hash for the CSP script-src directive. The
// negative lookahead approach is not available in Go's regexp engine,
// so we manually filter out script tags that have a `src=` attribute
// (those are external scripts; 'self' covers them).
var inlineScriptRe = regexp.MustCompile(`(?is)<script\b([^>]*)>(.*?)</script>`)

// srcAttrRe detects an `src=...` attribute inside a <script> tag's
// attribute list. Used to skip external scripts.
var srcAttrRe = regexp.MustCompile(`(?i)\bsrc\s*=`)

// configureCSPFromFrontend reads the embedded SPA index.html, computes
// the CSP-compatible SHA-256 hash of every inline <script> body, and
// installs the resulting script-src allow-list into middleware.
//
// Per ADR (issue #8): we want CSP script-src without 'unsafe-inline' so
// a future XSS sink in the SPA cannot inject and execute arbitrary
// inline JavaScript. The single inline script we ship today (the theme
// bootstrap that runs before React hydrates to prevent flash) is
// allowed by hash, computed at startup from the bytes Vite actually
// emitted — so the source of truth stays in web/index.html, not in a
// human-maintained constant.
//
// If the embedded frontend is missing (very early dev, before any
// `npm run build`), we fall back to 'unsafe-inline' so the binary
// still serves the placeholder HTML rather than refusing to render
// any inline JS.
func configureCSPFromFrontend() {
	dist, err := fs.Sub(frontend.DistFS, "dist")
	if err != nil {
		log.Printf("CSP: frontend not embedded — using 'unsafe-inline' for script-src (dev mode)")
		middleware.SetSPAScriptHashes(nil)
		return
	}

	indexHTML, err := fs.ReadFile(dist, "index.html")
	if err != nil {
		log.Printf("CSP: cannot read embedded index.html (%v) — using 'unsafe-inline' for script-src", err)
		middleware.SetSPAScriptHashes(nil)
		return
	}

	hashes := extractInlineScriptHashes(indexHTML)
	middleware.SetSPAScriptHashes(hashes)
	if len(hashes) == 0 {
		log.Printf("CSP: index.html has no inline scripts — script-src is 'self' only")
	} else {
		log.Printf("CSP: locked script-src to 'self' + %d inline-script hash(es)", len(hashes))
	}
}

// extractInlineScriptHashes returns the base64 SHA-256 of every inline
// <script> body in the given HTML, in document order. Scripts with a
// src= attribute are skipped (those are external; 'self' covers them).
//
// Exposed (unexported, but in main package) for testing.
func extractInlineScriptHashes(html []byte) []string {
	matches := inlineScriptRe.FindAllSubmatch(html, -1)
	if len(matches) == 0 {
		return nil
	}
	hashes := make([]string, 0, len(matches))
	for _, m := range matches {
		attrs := m[1]
		body := m[2]
		// Skip external scripts — CSP 'self' already covers them.
		if srcAttrRe.Match(attrs) {
			continue
		}
		// CSP hashes the script body byte-for-byte, including
		// surrounding whitespace and the leading/trailing newline.
		// Do NOT trim or normalize — Vite emits bytes that the
		// browser sees verbatim.
		sum := sha256.Sum256(body)
		hashes = append(hashes, base64.StdEncoding.EncodeToString(sum[:]))
	}
	return hashes
}
