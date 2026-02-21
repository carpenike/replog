# ADR 008: Notification System

**Status:** Accepted
**Date:** 2026-02-20

## Context

RepLog needs a way to notify users about events: workout reviews, program assignments, training max updates, magic link delivery, and other coaching actions. Notifications must work as in-app toast messages and optionally dispatch to external channels (email, push services, webhooks) without requiring separate infrastructure.

The existing "pending reviews" feature (`/reviews/pending`) is effectively a manual notification inbox. Formalizing notifications into a first-class system provides a consistent UX and enables external delivery.

## Decision

### Architecture: Channel-Agnostic Dispatch

Notification creation is decoupled from delivery. A single `notify.Send()` call resolves the target user, checks per-type preferences, and dispatches to enabled channels:

- **In-app:** Insert into `notifications` table → displayed as toast popups and a notification list page.
- **External:** Send via [Shoutrrr](https://github.com/containrrr/shoutrrr) (Go library, compiled into binary) → supports 30+ services (ntfy, Pushover, Gotify, Discord, Slack, Telegram, SMTP, webhooks, etc.).

Adding a new event type requires one `notify.Send()` call in the relevant handler. Adding a new channel requires updating the dispatch layer — zero handler changes.

### Shoutrrr Over Alternatives

| Option | Fit |
|--------|-----|
| Apprise | Python — requires sidecar container, breaks single-binary |
| Custom SMTP | Only one channel, limited utility |
| Webhook-only | Simple but no rich service support |
| **Shoutrrr** | **Go native, compiles into binary, URL-based config, 30+ services** |

Shoutrrr URLs are stored in `app_settings` (encrypted via the existing secret key system). Admins configure them through the existing Settings UI.

### In-App Delivery: Toast Notifications

In-app notifications are displayed as **toast popups** using htmx polling:

1. The base layout polls `GET /notifications/toast` every 30 seconds.
2. If new unread notifications exist, the endpoint returns an HTML toast fragment.
3. Toasts auto-dismiss after 5 seconds or on click.
4. A notification bell icon with unread badge count is added to the sidebar and mobile topbar.
5. A dedicated notifications list page (`/notifications`) shows full history with mark-as-read.

No WebSockets — htmx polling is sufficient for a family-scale app.

### Schema

Two new tables consolidated into the initial migration (`0001_initial_schema.sql`):

- `notifications` — stores in-app notifications (user_id, type, title, message, link, read, athlete_id, created_at).
- `notification_preferences` — per-user, per-type opt-in/out for in-app and external channels.

### Notification Types (Initial Set)

| Type | Recipient | Trigger |
|------|-----------|---------|
| `review_submitted` | Athlete | Coach reviews a workout |
| `program_assigned` | Athlete | Coach assigns a program |
| `tm_updated` | Athlete | Training max changed |
| `note_added` | Athlete | Coach adds a public note |
| `magic_link_sent` | User (via external) | Admin generates a login token |
| `workout_logged` | Coach | Athlete completes a workout |

### External Configuration

Shoutrrr URLs are stored as an `app_settings` key (`notify.urls`) — comma-separated, encrypted. Example:

```
smtp://user:pass@host:587/?to=coach@example.com
ntfy://mytopic
pushover://token@user
```

## Consequences

- **Single binary preserved** — Shoutrrr compiles in, no sidecar needed.
- **No JavaScript framework** — toasts use htmx + CSS animations, consistent with ADR 001.
- **Graceful degradation** — if no Shoutrrr URLs are configured, only in-app notifications fire. If a send fails, it's logged but doesn't block the triggering action.
- **New Go dependency** — `github.com/containrrr/shoutrrr` added to `go.mod`.
- **Polling overhead** — one lightweight query per 30 seconds per connected session. Negligible for family-scale.
