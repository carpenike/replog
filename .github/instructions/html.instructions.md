---
applyTo: "**/*.html,**/*.gohtml,**/*.tmpl"
---

# HTML Template Conventions for RepLog

## Template Engine

- Use Go's `html/template` — never `text/template` for HTML output
- Templates are embedded via `embed.FS` and parsed once at startup
- Use `{{ template "name" . }}` for partials/fragments
- Use `{{ block "name" . }}...{{ end }}` for overridable sections in layouts

## htmx Patterns

- All interactivity uses htmx attributes — no custom JavaScript
- htmx is loaded from a single `<script>` tag in the base layout
- Use `hx-get` / `hx-post` / `hx-put` / `hx-delete` for CRUD operations
- Use `hx-target` to specify where the response HTML goes
- Use `hx-swap="outerHTML"` for replacing the current element (forms, cards)
- Use `hx-swap="innerHTML"` for updating container contents (lists, tables)
- Use `hx-boost="true"` on navigation links for SPA-like page transitions
- Use `hx-indicator` for loading states on slow operations

## Response Patterns

- Full page loads: return complete HTML document with layout
- htmx requests (check `HX-Request` header): return HTML fragment only
- After successful form submission: return the updated element or redirect
- Validation errors: return the form with error messages, HTTP 422
- Not found: HTTP 404 with a user-friendly message

## Accessibility & Semantics

- Use semantic HTML: `<main>`, `<nav>`, `<section>`, `<form>`, `<table>`
- Forms use `<label>` elements linked to inputs via `for`/`id`
- Tables use `<thead>`, `<tbody>`, `<th scope="col|row">`
- Use `<button type="submit">` inside forms, not `<a>` tags for actions
