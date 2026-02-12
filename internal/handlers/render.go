package handlers

import (
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"path/filepath"

	"github.com/carpenike/replog/internal/middleware"
)

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
			ts, err := template.ParseFS(fsys, page)
			if err != nil {
				return nil, fmt.Errorf("handlers: parse %s: %w", name, err)
			}
			cache[name] = ts
			continue
		}

		// All other pages extend the base layout.
		ts, err := template.ParseFS(fsys, "templates/layouts/base.html", page)
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
		ts, err := template.ParseFS(fsys, partial)
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

	// Non-boosted htmx requests get just the content fragment.
	if r.Header.Get("HX-Request") == "true" && r.Header.Get("HX-Boosted") != "true" {
		return ts.ExecuteTemplate(w, "content", data)
	}

	return ts.ExecuteTemplate(w, "base", data)
}
