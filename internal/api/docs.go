package api

import (
	_ "embed"
	"net/http"
)

//go:embed swagger-ui.html
var swaggerHTML []byte

//go:embed openapi.yaml
var openapiSpec []byte

// DocsHandler serves the Swagger UI page.
func DocsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(swaggerHTML)
}

// SpecHandler serves the OpenAPI YAML spec.
func SpecHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/yaml; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Write(openapiSpec)
}
