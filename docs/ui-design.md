# UI Design — Dashboard Iteration

> RepLog UI redesign plan — Pico CSS v2 migration + fitness-specific polish.

## Design Principles

1. **Phone-first.** The primary use case is logging sets at the gym on a phone. Touch targets must be large, forms must be quick, and the layout must work in portrait mode.
2. **Semantic over stylistic.** Rely on HTML element semantics for styling (Pico's approach). Add classes only for domain-specific components.
3. **Dark mode by default.** Gyms are often dimly lit. Support OS-preference auto-switching and a manual toggle.
4. **Information hierarchy.** The most important data (current exercise, reps, weight) should be visually dominant. Secondary data (timestamps, IDs) should recede.
5. **Speed over spectacle.** No animations that slow down set logging. Loading indicators for htmx requests, but no gratuitous transitions.

---

## Phase 1: Foundation Swap

### Base Layout (`base.html`)

- [ ] Add `<meta name="color-scheme" content="light dark">` for automatic dark mode
- [ ] Replace custom `<link>` to `style.css` with Pico CSS (`pico.min.css`) + slim `app.css`
- [ ] Restructure `<body>` with Pico layout: `<header>` → `<nav class="container">`, `<main class="container">`, `<footer>`
- [ ] Update nav to use Pico's `<nav>` with `<ul>` list pattern for links
- [ ] Add dark/light theme toggle in nav (using `data-theme` attribute)
- [ ] Preserve `hx-boost="true"` on `<body>` for SPA-like navigation
- [ ] Preserve CSRF meta tag and htmx config scripts

### CSS Files

- [ ] Vendor `pico.min.css` (v2) into `static/css/pico.min.css`
- [ ] Replace `style.css` with `app.css` containing only:
  - CSS variable overrides for RepLog brand colors
  - Tier badge component
  - Workout logging form layout
  - Promote confirmation component
  - Utility classes specific to RepLog (if any remain)
- [ ] Remove all Pico-redundant styles (button resets, table styling, form inputs, alert boxes, nav layout)

### Verification

- [ ] All 18 pages render without broken layout
- [ ] Dark mode toggles correctly
- [ ] htmx fragment swaps still work (non-boosted requests return content fragment only)
- [ ] Login page renders standalone (no base layout)
- [ ] Mobile viewport looks correct on 375px width (iPhone SE)
- [ ] All tests still pass (`go test ./...`)

---

## Phase 2: Page-by-Page Template Updates

### Priority order (by user impact):

#### 2a. Login page (`login.html`)
- [ ] Center card with Pico's `<article>` inside a flex container
- [ ] Use Pico's `<form>` styling (labels wrapping inputs)
- [ ] Validation errors via `aria-invalid` attribute on inputs (Pico styles these red)
- [ ] Remove `.login-container`, `.login-card` custom classes

#### 2b. Dashboard / Index (`index.html`)
- [ ] Coach dashboard: use `<article>` cards in a `<div class="grid">` for Athletes/Exercises/Users nav
- [ ] Quick Start table: standard Pico `<table>` (striped)
- [ ] Non-coach unlinked message: use `<article>` with centered text
- [ ] Remove `.dashboard-nav`, `.dashboard-card` custom classes

#### 2c. Athlete Detail (`athlete_detail.html`) — largest template
- [ ] Breadcrumb: plain text with `›` separator (small muted text)
- [ ] Profile header: `<hgroup>` with name + tier badge
- [ ] Tier badge: small colored `<mark>` or `<span>` with custom CSS
  - Foundational → green
  - Intermediate → blue
  - Sport Performance → purple
  - No tier → gray
- [ ] Detail fields (tier, notes): `<dl>` definition list or Pico's grid
- [ ] Workouts section: `<article>` container with `<table>` inside
- [ ] Assigned Exercises section: `<article>` container with `<table>` inside
  - Current TM displayed inline
  - Deactivate button uses `role="button"` on a small element
- [ ] Deactivated assignments: collapsible `<details>` with reactivate buttons
- [ ] Training Maxes section: `<article>` with summary per exercise
- [ ] **Promote action**: dedicated "Promote" button with `hx-confirm` showing current → next tier
- [ ] Remove `.page-header`, `.page-actions`, `.detail-grid`, `.detail-item`, `.detail-label`, `.detail-value`, `.section`, `.section-header` custom classes

#### 2d. Workout Detail (`workout_detail.html`) — core logging flow
- [ ] Breadcrumb navigation
- [ ] Workout date + athlete name as `<hgroup>`
- [ ] Session notes: `<details>` accordion (Pico styles natively)
- [ ] Logged sets: grouped by exercise in `<article>` cards
  - Each exercise group: `<h3>` header + `<table>` of sets
  - Edit/delete buttons: Pico `<button class="outline secondary">` pattern
- [ ] **Add Set form** (critical UX):
  - Exercise picker: `<select>` with optgroups (Assigned / All Exercises)
  - Reps + weight fields: side-by-side with `<div class="grid">`
  - Large "Log Set" submit button for easy phone tapping
  - Notes field: collapsed by default in `<details>`
- [ ] Prescribed exercises reference: `<details>` accordion showing target reps + TM
- [ ] Remove `.exercise-group`, `.add-set-form`, `.form-row`, `.form-group-sm`, `.set-actions`, `.notes-toggle`, `.notes-form` custom classes

#### 2e. Athletes List (`athletes_list.html`)
- [ ] Standard Pico `<table class="striped">`
- [ ] "New Athlete" button in page header
- [ ] Tier displayed as colored badge
- [ ] Remove `.data-table` class

#### 2f. Exercises List (`exercises_list.html`)
- [ ] Tier filter: Pico `<fieldset>` with `role="group"` for pill-style filter buttons
- [ ] Standard Pico `<table class="striped">`
- [ ] Remove `.filter-bar`, `.filter-chip` custom classes

#### 2g. Workouts List (`workouts_list.html`)
- [ ] Standard Pico `<table class="striped">`
- [ ] Pagination: "Load More" button (htmx append)
- [ ] Empty state: `<article>` with centered prompt

#### 2h. Form Pages (athlete, exercise, workout, training max, user, assignment, set edit)
- [ ] All forms: Pico's label-wrapping-input pattern
- [ ] Validation errors: `aria-invalid="true"` on the field + error text below
- [ ] Form-level error: Pico `<ins>` or custom `.alert` (small custom CSS)
- [ ] Submit buttons: Pico `<button>` (full width inside form)
- [ ] Cancel links: `<a href="..." role="button" class="secondary">`
- [ ] Remove `.form-group`, `.form-card`, `.form-actions`, `.btn`, `.btn-primary`, `.btn-full`, `.alert`, `.alert-error` custom classes

#### 2i. Training Max History (`training_max_history.html`)
- [ ] `<table>` with date, weight, notes columns
- [ ] "Set New TM" button in header

#### 2j. Exercise History (`exercise_history.html`)
- [ ] Grouped by workout date
- [ ] Each day: `<article>` card with date header + sets table
- [ ] Pagination: "Load More" button

#### 2k. Exercise Detail (`exercise_detail.html`)
- [ ] Form notes in a prominent `<blockquote>` or `<article>`
- [ ] Assigned athletes: `<table>`
- [ ] Recent log history: `<table>`

#### 2l. Users List + User Form (`users_list.html`, `user_form.html`)
- [ ] Standard Pico table and form patterns
- [ ] Coach badge: `<mark>` element

#### 2m. Error Fragment (`error_fragment.html`)
- [ ] Style as Pico's `<article>` with `aria-invalid` coloring or a red-tinted `<ins>`

### Verification per page

- [ ] Visual check at 375px (phone), 768px (tablet), 1200px (desktop)
- [ ] Dark mode appearance
- [ ] htmx interactions still function (fragment swap, form submit, confirm dialogs)
- [ ] No remnant custom classes referencing deleted CSS rules

---

## Phase 3: Fitness-Specific Polish

### Tier Badges (custom CSS component)
- [ ] Small inline badge next to athlete/exercise names
- [ ] Color-coded: Foundational=green, Intermediate=blue, Sport Performance=purple, None=gray
- [ ] Works in both light and dark mode
- [ ] Implementation: `<mark>` with CSS custom properties or a `.tier-badge` class

### Promote with Confirmation
- [ ] "Promote" button visible on athlete detail when a next tier exists
- [ ] Uses `hx-confirm` with descriptive message: "Promote {Name} from {Current} to {Next}?"
- [ ] Server-side logic: compute next tier (Foundational→Intermediate→Sport Performance)
- [ ] New handler: `POST /athletes/{id}/promote`
- [ ] New model helper: `NextTier(currentTier) string`
- [ ] Button hidden if already at highest tier or no tier set

### Workout Logging UX (phone optimization)
- [ ] Exercise select: large touch target, assigned exercises listed first
- [ ] Reps field: `type="number"` with `inputmode="numeric"` for numeric keyboard
- [ ] Weight field: same numeric input, optional
- [ ] "Log Set" button: full width, prominent, easy to tap between sets
- [ ] After logging: scroll to show the new set in the table (htmx `hx-swap` with scroll behavior)

### Theme Toggle
- [ ] Small toggle in nav: sun/moon icon or text (Light / Dark / Auto)
- [ ] Persisted via `localStorage` — no server round-trip
- [ ] Sets `data-theme` attribute on `<html>` element
- [ ] Minimal JS (~10 lines) — acceptable exception to "no JS" since it's a DOM attribute toggle

### Empty States
- [ ] Consistent messaging for empty lists (no athletes, no workouts, no assignments)
- [ ] Each empty state includes a primary action CTA button
- [ ] Styled as centered text in an `<article>` card

---

## Go Code Changes

The design iteration is **almost entirely templates + CSS**. Backend changes are minimal:

### New handler (Phase 3 — Promote)
- `handlers/athletes.go`: Add `Promote` method (`POST /athletes/{id}/promote`)
- `models/athlete.go`: Add `NextTier(current string) string` helper
- `cmd/replog/main.go`: Register `POST /athletes/{id}/promote` route with `requireCoach`

### Template function (optional)
- Consider adding a `tierLabel` template function to convert `sport_performance` → `Sport Performance` for display

### No other backend changes
- No model changes
- No migration
- No middleware changes
- No route restructuring (except the one new promote route)

---

## Verification Checklist

After all phases are complete:

- [ ] All 18+ page templates updated
- [ ] `style.css` removed, replaced by `pico.min.css` + `app.css`
- [ ] `go test ./...` passes
- [ ] `go vet ./...` clean
- [ ] `go build ./cmd/replog` succeeds
- [ ] Visual smoke test on phone (375px), tablet (768px), desktop (1200px)
- [ ] Dark mode works on all pages
- [ ] Light mode works on all pages
- [ ] htmx fragment swaps work (non-boosted partial requests)
- [ ] htmx boosted navigation works
- [ ] CSRF tokens flow correctly through forms
- [ ] Login page renders standalone
- [ ] Coach sees all athletes, exercises, users
- [ ] Non-coach sees only their athlete
- [ ] Unlinked non-coach sees informative message
- [ ] Promote button appears and works for coaches
- [ ] Theme toggle persists across page loads
- [ ] Test templates in `internal/handlers/testdata/` updated to match
