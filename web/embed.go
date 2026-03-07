package frontend

import "embed"

// DistFS holds the compiled React frontend, embedded at build time.
// In development, web/dist/ contains a placeholder; the production build
// populates it with the Vite output.
//
//go:embed all:dist
var DistFS embed.FS
