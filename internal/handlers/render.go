package handlers

import (
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// cachedAppName holds the application name, set at startup and refreshed
// when admin settings are saved. Safe for concurrent reads.
var cachedAppName atomic.Value

func init() {
	cachedAppName.Store("RepLog")
}

// InitAppName reads the app name from settings and caches it.
func InitAppName(name string) {
	if name != "" {
		cachedAppName.Store(name)
	}
}

// RefreshAppName re-reads the app name from settings and updates the cache.
func RefreshAppName(name string) {
	if name != "" {
		cachedAppName.Store(name)
	} else {
		cachedAppName.Store("RepLog")
	}
}

// AppName returns the current cached application name.
func AppName() string {
	return cachedAppName.Load().(string)
}

// templateFuncs contains custom template helper functions.
var templateFuncs = template.FuncMap{
	// appName returns the current application name (configurable via Settings).
	"appName": func() string {
		return AppName()
	},
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
	"displayName": func(user *models.User) string {
		if user.Name.Valid && user.Name.String != "" {
			return user.Name.String
		}
		return user.Username
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
		// Try YYYY-MM-DD first, then RFC3339 (modernc.org/sqlite stores DATE
		// columns as full timestamps like "2026-02-17T00:00:00Z").
		t, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			t, err = time.Parse(time.RFC3339, dateStr)
		}
		if err != nil {
			// Truncate to date portion if it looks like an ISO timestamp.
			if len(dateStr) > 10 {
				dateStr = dateStr[:10]
			}
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
	// deref dereferences a *float64, returning 0 if nil. Useful in templates
	// that receive optional numeric values from model structs.
	"deref": func(p *float64) float64 {
		if p == nil {
			return 0
		}
		return *p
	},
	// derefStr dereferences a *string, returning "" if nil. Useful in templates
	// that receive optional string values from context/model structs.
	"derefStr": func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	},
	// settingFieldType returns the HTML input type for a setting key.
	"settingFieldType": func(key string) string {
		def := models.GetSettingDefinition(key)
		if def == nil {
			return "text"
		}
		return def.FieldType
	},
	// settingDescription returns the description for a setting key.
	"settingDescription": func(key string) string {
		def := models.GetSettingDefinition(key)
		if def == nil {
			return ""
		}
		return def.Description
	},
	// settingIsSensitive returns true if the setting stores sensitive data.
	"settingIsSensitive": func(key string) bool {
		def := models.GetSettingDefinition(key)
		if def == nil {
			return false
		}
		return def.Sensitive
	},
	// settingOptions returns the valid options for a select-type setting.
	"settingOptions": func(key string) []string {
		def := models.GetSettingDefinition(key)
		if def == nil {
			return nil
		}
		return def.Options
	},
	// settingLabel returns the human-readable label for a setting key.
	"settingLabel": func(key string) string {
		def := models.GetSettingDefinition(key)
		if def == nil {
			return key
		}
		return def.Label
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

// RenderErrorPage renders a full-page styled error with the base layout.
// It sets the HTTP status code and displays a user-friendly error page with
// title, message, and a link back home. For non-boosted htmx requests, only
// the content fragment is returned.
func (tc TemplateCache) RenderErrorPage(w http.ResponseWriter, r *http.Request, status int, title, message string) {
	data := map[string]any{
		"ErrorCode":    status,
		"ErrorTitle":   title,
		"ErrorMessage": message,
	}

	// Inject authenticated user for base layout nav rendering.
	if user := middleware.UserFromContext(r.Context()); user != nil {
		data["User"] = user
	}

	// Inject user preferences for template helpers.
	if prefs := middleware.PrefsFromContext(r.Context()); prefs != nil {
		data["Prefs"] = prefs
	}

	// Inject CSRF token.
	if token := middleware.CSRFTokenFromContext(r.Context()); token != "" {
		data["CSRFToken"] = token
	}

	w.WriteHeader(status)

	ts, ok := tc["error.html"]
	if !ok {
		// Fallback to plain text if error template is missing.
		http.Error(w, fmt.Sprintf("%d %s", status, title), status)
		return
	}

	// Non-boosted htmx requests get just the content fragment.
	if r.Header.Get("HX-Request") == "true" && r.Header.Get("HX-Boosted") != "true" {
		ts.ExecuteTemplate(w, "content", data)
		return
	}

	ts.ExecuteTemplate(w, "base", data)
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

// Forbidden renders a 403 error page. Convenience wrapper around RenderErrorPage.
func (tc TemplateCache) Forbidden(w http.ResponseWriter, r *http.Request) {
	tc.RenderErrorPage(w, r, http.StatusForbidden, "Access Denied",
		"You don't have permission to perform this action.")
}

// NotFound renders a 404 error page. Convenience wrapper around RenderErrorPage.
func (tc TemplateCache) NotFound(w http.ResponseWriter, r *http.Request) {
	tc.RenderErrorPage(w, r, http.StatusNotFound, "Not Found",
		"The page you're looking for doesn't exist or has been moved.")
}

// ServerError renders a 500 error page. Convenience wrapper around RenderErrorPage.
func (tc TemplateCache) ServerError(w http.ResponseWriter, r *http.Request) {
	tc.RenderErrorPage(w, r, http.StatusInternalServerError, "Server Error",
		"Something went wrong. Please try again later.")
}
