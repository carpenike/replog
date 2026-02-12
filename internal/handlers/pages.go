package handlers

import (
	"database/sql"
	"html/template"
	"log"
	"net/http"
	"strconv"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// Pages holds dependencies for page handlers.
type Pages struct {
	DB        *sql.DB
	Templates *template.Template
}

// Index renders the home page for an authenticated user.
// Coaches see the dashboard. Non-coaches with a linked athlete are redirected
// to their profile. Unlinked non-coaches see an informative message.
func (p *Pages) Index(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())

	// Non-coach with linked athlete → redirect to their athlete profile.
	if !user.IsCoach && user.AthleteID.Valid {
		http.Redirect(w, r, "/athletes/"+strconv.FormatInt(user.AthleteID.Int64, 10), http.StatusSeeOther)
		return
	}

	data := map[string]any{
		"User": user,
	}

	if user.IsCoach {
		// Coach dashboard — show athletes for quick navigation.
		athletes, err := models.ListAthletes(p.DB)
		if err != nil {
			log.Printf("handlers: list athletes for dashboard: %v", err)
		} else {
			data["Athletes"] = athletes
		}
	}

	if err := p.Templates.ExecuteTemplate(w, "base", data); err != nil {
		log.Printf("handlers: index template error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
