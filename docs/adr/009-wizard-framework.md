# ADR 009: Wizard Framework for Setup Flows

**Status:** Accepted
**Date:** 2026-02-21

## Context

RepLog uses magic links (login tokens) as a first-login mechanism for family members — a coach generates a link, the kid taps it on their device, and they're in. However, once authenticated, there's nothing nudging users toward stronger auth (passkeys). Magic links are inherently shareable, visible in browser history, and weaker than device-bound credentials.

Additionally, RepLog has several multi-step workflows (data import, AI generation, program assignment) that each implement their own ad-hoc step sequencing. As the app grows, more onboarding and setup flows are likely:
- Post-login passkey enrollment
- New athlete onboarding (create → assign exercises → set training maxes)
- Program quick-start (select program → assign to athlete → set TMs → review)
- First-run admin setup

## Decision

### Wizard Layout

Introduce a dedicated **wizard layout** (`templates/layouts/wizard.html`) for focused, distraction-free multi-step flows:

- **No sidebar or navigation chrome** — users stay focused on the task
- **Step indicator** — numbered progress dots (done / active / upcoming)
- **Skip option** — respects user choice, doesn't nag
- **Centered card** — mobile-first, similar to login page layout

### Convention: `setup_*.html`

Page templates prefixed with `setup_` are automatically parsed with the wizard layout instead of the base layout. This is a zero-configuration convention — just name your template `setup_foo.html` and it gets the wizard chrome.

### `WizardStep` Type

A generic `WizardStep` struct (defined in `handlers/setup.go`) provides step metadata:

```go
type WizardStep struct {
    Number int
    Label  string
}
```

Templates receive `WizardSteps` (slice) and `WizardCurrentStep` (int) in their data to render the progress indicator.

### Passkey Enrollment as First Consumer

The first wizard flow is **passkey enrollment after magic link login**:

1. Coach generates magic link → sends to kid
2. Kid taps link → `TokenLogin` authenticates and checks passkey count
3. If zero passkeys → redirect to `/setup/passkey` (step 2 of 3)
4. User registers passkey via Face ID / Touch ID → redirect to success (step 3 of 3)
5. "Skip for now" stores a session flag — no nagging until next session

Magic link tokens now default to **7-day expiry** (down from 30 days) since they're an enrollment mechanism, not persistent auth.

### Session-Based Skip

The skip flag (`passkey_setup_skipped`) is session-scoped, not persisted to the database. This means:
- Users aren't nagged during a single session after skipping
- Next login, they'll see the setup prompt again (gentle nudge)
- No schema changes needed

### Preferences Page Nudge

For users who skip or arrive via password login, the preferences page shows a highlighted banner when zero passkeys are registered, encouraging registration.

## Consequences

### Positive
- Magic link → passkey enrollment creates a smooth onboarding path
- Wizard layout is reusable for future multi-step flows
- Convention-based (`setup_*`) so new wizards require minimal wiring
- No JavaScript framework needed — still server-rendered HTML + htmx

### Negative
- Wizard pages don't have the sidebar, so users must complete or skip to navigate
- Each wizard flow needs its own handler — no generic "wizard engine" (intentional: keeps it simple)

### Future Considerations
- Consider a `WizardFlow` interface if wizard patterns become repetitive:
  ```go
  type WizardFlow interface {
      Steps() []WizardStep
      CurrentStep(r *http.Request) int
      Render(w http.ResponseWriter, r *http.Request) error
  }
  ```
- Athlete onboarding wizard could chain: create athlete → assign exercises → set TMs
- Program quick-start could chain: select → assign → configure TMs → review prescription
