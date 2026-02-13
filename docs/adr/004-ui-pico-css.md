# ADR 004: UI Framework — Pico CSS v2

**Status:** Accepted
**Date:** 2026-02-12

## Context

RepLog's current UI is ~500 lines of hand-rolled CSS with CSS custom properties. It works but looks generic — plain borders, no depth, no dark mode, and minimal visual hierarchy. The app is primarily used on phones at the gym, so mobile experience matters.

We want a modern, clean design without introducing a JavaScript build step or heavy framework. The solution must:
- Ship as a static CSS file embedded in the Go binary (no CDN dependency)
- Work seamlessly with htmx fragment swaps (no client-side rendering assumptions)
- Look good on mobile without extensive custom media queries
- Support dark mode (common gym lighting preference)
- Minimize template complexity — fewer CSS classes, not more

## Decision

Adopt **[Pico CSS v2](https://picocss.com/)** as the CSS foundation, with a slim app-specific override stylesheet (`app.css`) for fitness-domain components.

### What we use

- `pico.min.css` (~11kb gzipped) — embedded in binary via `static/css/`
- `app.css` (~100-150 lines) — fitness-specific overrides and components
- Pico's semantic styling (elements auto-style without classes)
- Pico's `.container` and `.grid` for layout
- Pico's `<article>` card styling for section containers
- Pico's native dark mode via `<meta name="color-scheme" content="light dark">`
- Pico's form styling (labels wrap inputs, validation via `aria-invalid`)

### What we don't use

- Pico's SASS source (no build step — use the prebuilt CSS)
- Pico's classless version (we need `.container`, `.grid`, and occasional classes)
- CDN — the CSS file is vendored into `static/css/` and embedded in the binary

## Rationale

### Why Pico CSS

| Factor | Pico CSS | Tailwind CDN | Custom CSS (status quo) |
|---|---|---|---|
| File size | 11kb gzipped | 500kb+ (no purge without build) | ~5kb |
| Build step | None | Needs CDN or npm | None |
| Dark mode | Automatic | Manual per-element | Not implemented |
| htmx fragment compat | Excellent — semantic elements | Fine but class-heavy | Current |
| Template verbosity | Lower — fewer classes | Much higher | Moderate |
| Mobile-first | Built-in | Built-in | Manual |

**Key advantages for RepLog:**

1. **Our HTML is already semantic.** Pico styles `<table>`, `<form>`, `<button>`, `<article>`, `<nav>`, `<details>` directly. Most custom classes (`data-table`, `form-group`, `btn`) become unnecessary.

2. **htmx fragments auto-style.** When htmx swaps a `<table>` fragment into the DOM, Pico styles it immediately — no class injection needed. This is a direct advantage over utility-class frameworks.

3. **Dark mode is free.** `<meta name="color-scheme" content="light dark">` enables automatic OS-preference matching. Add `data-theme="dark"` to force a specific mode.

4. **No build step.** Download the minified CSS, put it in `static/css/`, done. This preserves our single-binary, zero-toolchain philosophy from ADR 001.

5. **Small custom layer.** App-specific styles (tier badges, workout logging form, promote confirmation) layer cleanly on top of Pico's CSS variables.

### Why not Tailwind

Tailwind is excellent for teams with build toolchains. Without one, the CDN version ships ~500kb of CSS (no tree-shaking). It also makes templates significantly more verbose — every element needs utility classes — which adds noise to Go templates. We'd be fighting the `html/template` ergonomics.

### Why not keep custom CSS

The current CSS works but doesn't provide dark mode, has inconsistent spacing on mobile, and requires manual work for every new component. Pico gives us a professional baseline for free and lets us focus custom CSS on fitness-domain components only.

## Consequences

- **Positive:** Professional look with minimal effort; dark mode; better mobile defaults
- **Positive:** Templates become simpler — many CSS classes are removed, not added
- **Positive:** New pages/features auto-style from semantic HTML — less design debt
- **Negative:** Pico has opinions about spacing, typography, and colors — customization requires CSS variable overrides, not class changes
- **Negative:** Advanced components (complex grids, custom progress rings) still need manual CSS on top of Pico
- **Migration:** All 18 page templates + base layout need updating; existing custom classes are systematically replaced with semantic elements or Pico equivalents
