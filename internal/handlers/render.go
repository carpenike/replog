package handlers

import (
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// templateFuncs contains custom template helper functions.
var templateFuncs = template.FuncMap{
	"userInitials": func(username string) string {
		if username == "" {
			return "?"
		}
		r := []rune(strings.ToUpper(username))
		if len(r) >= 2 {
			return string(r[:2])
		}
		return string(r[:1])
	},
	"tierLabel": func(tier string) string {
		switch tier {
		case "foundational":
			return "Foundational"
		case "intermediate":
			return "Intermediate"
		case "sport_performance":
			return "Sport Performance"
		default:
			if tier == "" {
				return ""
			}
			return strings.ToUpper(tier[:1]) + tier[1:]
		}
	},
	"nextTier": func(tier string) string {
		switch tier {
		case "foundational":
			return "intermediate"
		case "intermediate":
			return "sport_performance"
		default:
			return ""
		}
	},
	"nextTierLabel": func(tier string) string {
		switch tier {
		case "foundational":
			return "Intermediate"
		case "intermediate":
			return "Sport Performance"
		default:
			return ""
		}
	},
	// weightUnit returns the user's preferred weight unit label from the
	// UserPreferences injected into the template data. Templates call it as
	// {{ weightUnit .Prefs }} to get "lbs" or "kg".
	"weightUnit": func(prefs *models.UserPreferences) string {
		if prefs == nil {
			return models.DefaultWeightUnit
		}
		return prefs.WeightUnit
	},
	// formatDate formats a time.Time using the user's preferred date format
	// and timezone. Call as {{ formatDate .Prefs .SomeTime }}.
	"formatDate": func(prefs *models.UserPreferences, t time.Time) string {
		if t.IsZero() {
			return ""
		}
		format := models.DefaultDateFormat
		tz := "America/New_York"
		if prefs != nil {
			format = prefs.DateFormat
			tz = prefs.Timezone
		}
		loc, err := time.LoadLocation(tz)
		if err != nil {
			loc = time.UTC
		}
		return t.In(loc).Format(format)
	},
	// formatWeight formats a float64 weight value for display (no trailing zeros).
	// Call as {{ formatWeight 185.0 }}.
	"formatWeight": func(w float64) string {
		if w == float64(int(w)) {
			return fmt.Sprintf("%.0f", w)
		}
		return fmt.Sprintf("%.1f", w)
	},
	// formatVolume formats a large volume number with comma separators.
	// Call as {{ formatVolume 48200.0 }}.
	"formatVolume": func(v float64) string {
		n := int64(v)
		if n < 1000 {
			return fmt.Sprintf("%d", n)
		}
		s := fmt.Sprintf("%d", n)
		var result []byte
		for i, c := range s {
			if i > 0 && (len(s)-i)%3 == 0 {
				result = append(result, ',')
			}
			result = append(result, byte(c))
		}
		return string(result)
	},
	// subtract returns a - b. Used in range loops for accessing previous index.
	"subtract": func(a, b int) int {
		return a - b
	},
	// add returns a + b for template arithmetic.
	"add": func(a, b int) int {
		return a + b
	},
	// multiply returns a * b for template arithmetic.
	"multiply": func(a, b int) int {
		return a * b
	},
	// seq returns a slice of ints from start to end (inclusive) for range loops.
	"seq": func(start, end int) []int {
		if end < start {
			return nil
		}
		s := make([]int, 0, end-start+1)
		for i := start; i <= end; i++ {
			s = append(s, i)
		}
		return s
	},
	// formatDateStr formats a YYYY-MM-DD date string using the user's preferred
	// date format. Call as {{ formatDateStr .Prefs "2025-01-15" }}.
	"formatDateStr": func(prefs *models.UserPreferences, dateStr string) string {
		if dateStr == "" {
			return ""
		}
		t, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			return dateStr
		}
		format := models.DefaultDateFormat
		if prefs != nil {
			format = prefs.DateFormat
		}
		return t.Format(format)
	},
	// lastSet returns the last WorkoutSet in a slice. Used by the quick-add
	// form to pre-fill reps/weight from the most recent set in a group.
	"lastSet": func(sets []*models.WorkoutSet) *models.WorkoutSet {
		if len(sets) == 0 {
			return nil
		}
		return sets[len(sets)-1]
	},
}

// TemplateCache maps page filenames to parsed template sets. Each set contains
// the base layout combined with a single page template.
type TemplateCache map[string]*template.Template

// NewTemplateCache parses all page templates from the embedded filesystem.
// Each page is combined with the base layout; the login page is parsed
// standalone since it has no auth context.
func NewTemplateCache(fsys fs.FS) (TemplateCache, error) {
	cache := TemplateCache{}

	pages, err := fs.Glob(fsys, "templates/pages/*.html")
	if err != nil {
		return nil, fmt.Errorf("handlers: glob page templates: %w", err)
	}

	for _, page := range pages {
		name := filepath.Base(page)

		// Login page is standalone — no base layout needed.
		if name == "login.html" {
			ts, err := template.New(name).Funcs(templateFuncs).ParseFS(fsys, page)
			if err != nil {
				return nil, fmt.Errorf("handlers: parse %s: %w", name, err)
			}
			cache[name] = ts
			continue
		}

		// All other pages extend the base layout.
		ts, err := template.New(name).Funcs(templateFuncs).ParseFS(fsys, "templates/layouts/base.html", page)
		if err != nil {
			return nil, fmt.Errorf("handlers: parse %s with layout: %w", name, err)
		}
		cache[name] = ts
	}

	// Parse standalone partial templates (error fragments, etc.).
	partials, err := fs.Glob(fsys, "templates/partials/*.html")
	if err != nil {
		return nil, fmt.Errorf("handlers: glob partial templates: %w", err)
	}
	for _, partial := range partials {
		ts, err := template.New(filepath.Base(partial)).Funcs(templateFuncs).ParseFS(fsys, partial)
		if err != nil {
			return nil, fmt.Errorf("handlers: parse partial %s: %w", partial, err)
		}
		// Store with underscore prefix to distinguish from pages.
		name := "_" + filepath.Base(partial)
		name = name[:len(name)-len(filepath.Ext(name))]
		cache[name] = ts
	}

	return cache, nil
}

// RenderErrorFragment writes a standalone error message HTML fragment. Used for
// htmx responses that need to display an inline error (e.g., delete conflicts).
func (tc TemplateCache) RenderErrorFragment(w http.ResponseWriter, msg string) error {
	ts, ok := tc["_error_fragment"]
	if !ok {
		return fmt.Errorf("handlers: error fragment template not found in cache")
	}
	return ts.ExecuteTemplate(w, "error-fragment", msg)
}

// Render executes a page template with the base layout. It automatically injects
// the authenticated User into the template data for nav rendering. For non-boosted
// htmx requests, only the content fragment is returned.
func (tc TemplateCache) Render(w http.ResponseWriter, r *http.Request, name string, data map[string]any) error {
	ts, ok := tc[name]
	if !ok {
		return fmt.Errorf("handlers: template %q not found in cache", name)
	}

	if data == nil {
		data = map[string]any{}
	}

	// Inject authenticated user for base layout nav rendering.
	if _, exists := data["User"]; !exists {
		if user := middleware.UserFromContext(r.Context()); user != nil {
			data["User"] = user
		}
	}

	// Inject user preferences for template helpers (weightUnit, formatDate, etc.).
	if _, exists := data["Prefs"]; !exists {
		if prefs := middleware.PrefsFromContext(r.Context()); prefs != nil {
			data["Prefs"] = prefs
		}
	}

	// Inject CSRF token for forms.
	if _, exists := data["CSRFToken"]; !exists {
		if token := middleware.CSRFTokenFromContext(r.Context()); token != "" {
			data["CSRFToken"] = token
		}
	}

	// Non-boosted htmx requests get just the content fragment.
	if r.Header.Get("HX-Request") == "true" && r.Header.Get("HX-Boosted") != "true" {
		return ts.ExecuteTemplate(w, "content", data)
	}

	return ts.ExecuteTemplate(w, "base", data)
}
