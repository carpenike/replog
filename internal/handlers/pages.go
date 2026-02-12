package handlers

import (
	"html/template"
	"log"
	"net/http"

	"github.com/carpenike/replog/internal/middleware"
)

// Pages holds dependencies for page handlers.
type Pages struct {
	Templates *template.Template
}

// Index renders the home page for an authenticated user.
func (p *Pages) Index(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())

	data := map[string]any{
		"User": user,
	}

	if err := p.Templates.ExecuteTemplate(w, "base", data); err != nil {
		log.Printf("handlers: index template error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
