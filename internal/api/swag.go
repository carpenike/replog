// Package api provides the JSON REST API and its OpenAPI documentation.
//
// The OpenAPI spec is generated from swag annotations on each handler and the
// general info below. Run `just openapi` to regenerate after changing any
// handler signature or adding/removing a route.
package api

// @title           RepLog API
// @version         1.0.0
// @description     REST API for the RepLog workout tracking application.
// @description
// @description     Authentication uses session cookies issued by `POST /api/login`
// @description     (or the WebAuthn / token-login endpoints). Cookies are
// @description     `HttpOnly`, `SameSite=Lax`, and (in production) `Secure`.
// @description     The browser sends them automatically on subsequent requests.
//
// @BasePath  /api
//
// @tag.name Auth
// @tag.description Login, logout, current user, magic-link token, OIDC login
//
// @tag.name Dashboard
// @tag.description Aggregated home-page stats
//
// @tag.name Athletes
// @tag.description Athlete CRUD, journals, prescriptions, programs, accessories
//
// @tag.name Workouts
// @tag.description Workout and set logging
//
// @tag.name Exercises
// @tag.description Exercise catalog and per-athlete history
//
// @tag.name Programs
// @tag.description Program template CRUD, prescribed sets, progression rules
//
// @tag.name TrainingMaxes
// @tag.description Training max history per athlete + exercise
//
// @tag.name Equipment
// @tag.description Equipment catalog and per-athlete / per-exercise links
//
// @tag.name Reviews
// @tag.description Coach review of logged workouts
//
// @tag.name Notifications
// @tag.description User notifications and preferences
//
// @tag.name Preferences
// @tag.description Per-user preferences (theme, units, etc.)
//
// @tag.name Avatars
// @tag.description User avatar upload
//
// @tag.name Users
// @tag.description User management (admin only)
//
// @tag.name Admin
// @tag.description Admin-only impersonation, settings, and catalog import/export
