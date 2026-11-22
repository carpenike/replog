# UI Design Guide

The RepLog SPA is React 19 + TypeScript + Vite, styled with Tailwind CSS v4
and shadcn/ui components (copied into `web/src/components/ui/`, built on
`@base-ui/react`). Server state goes through TanStack Query and the typed
client in `web/src/api/`. This guide covers the shared patterns every page
uses. It replaces the retired Pico CSS / htmx design (ADR 001 and ADR 004,
superseded by ADR 011).

The app is a logbook. Copy and UI never imply that the app makes coaching
decisions: suggestions are labelled as suggestions, and the coach applies
them. See `docs/COACH_VOICE.md` for wording.

## Principles

- **Phone first.** Kids and adults log at the rack on a phone. Every control
  is at least 44px tall below `md`, and the logging loop needs one hand.
- **One pattern per job.** Use the shared components below instead of
  hand-rolled headers, spinners, empty states or form rows.
- **Real elements.** Anything clickable is a `<Link>` or `<button>`, never a
  `div` or `tr` with `onClick`. Labels are associated with their controls.
- **Quiet chrome, loud content.** Navigation and action buttons use lucide
  icons. Emoji are for content: streaks, PRs, celebration and empty states.

## Tokens

Colors are CSS variables in `web/src/index.css` (OKLCH), redefined under
`.dark`. Use the Tailwind names (`bg-primary`, `text-muted-foreground`), not
raw colors.

| Token | Use |
| --- | --- |
| `--primary` | The single accent hue (blue, 262). Primary buttons, progress bars, active nav, links. |
| `--ring` | Focus rings. Same hue as primary. |
| `--destructive` | Delete, errors, required-field marker. |
| `--success` / `--warning` | Tinted backgrounds only (`bg-success/10`, `bg-warning/15`). |
| `--success-strong` / `--warning-strong` | Text and icons on light backgrounds. These pass contrast as text; the plain tokens do not in light mode. |
| `--muted-foreground` | Secondary text, metadata, hints. |
| `--app-header-offset` | Height of the fixed mobile header plus the iOS safe-area inset. Sticky elements sit at `top-(--app-header-offset)` below `md`. |
| `--toast-offset-bottom` | Lifts desktop toasts above the rest-timer bar. |

`<meta name="theme-color">` follows the active theme. The layout uses
`viewport-fit=cover` and `env(safe-area-inset-*)` on the fixed header, nav
sheet, main content and the sticky session header.

## Page anatomy

Every route renders, in order:

1. `PageHeader` (title, back link, actions).
2. Optional page-level form or filter.
3. A `QueryState` for each primary query.

```tsx
<PageHeader
  title="Body Weight"
  back={{ to: `/athletes/${athleteId}`, label: athlete?.name ?? 'Athlete' }}
  actions={<Button render={<Link to="new" />}><Plus aria-hidden="true" /> Log</Button>}
/>
<QueryState query={query} resource="body weights" empty={{ icon: '⚖️', title: 'No entries yet' }}>
  {page => <EntryList entries={page.entries} />}
</QueryState>
```

### PageHeader (`components/PageHeader.tsx`)

- Renders the page's only `<h1>`, an optional one-level-up back link
  (chevron + parent name), an optional description line, a `meta` slot for
  badges, and right-aligned `actions` that wrap under the title on phones.
- Sets `document.title` (`"<title> · RepLog"`). Pass `pageTitle` when the h1
  is not a plain string, or `null` while data loads.
- Deep pages always pass `back`. The back link points at the logical parent
  route (athlete sub-pages go to the athlete), not browser history.
- Pages that cannot use it (the live session screen, login) call
  `usePageTitle` directly and still render an `<h1>` (visually hidden if
  needed).

### QueryState (`components/QueryState.tsx`)

One component for the four states of a query:

| State | Renders |
| --- | --- |
| Loading (`data === undefined`) | `PageSkeleton` (`list`, `cards`, `detail`, `form`) or a custom node. |
| Error | `QueryError`: the message plus **Retry** (calls `refetch`). |
| 404 | A soft "No `<resource>` found" empty state, or the `notFound` node. A 404 is an expected state, not an error. |
| Empty (`isEmpty(data)`, default: empty array) | `EmptyState` with a call to action. |
| Data | `children(data)`. |

Do not use bare spinners, `"Failed to load…"` text or a plain
`<p>No items.</p>`. An `EmptyState` says what is missing and, when the
viewer can fix it, offers the action (`action` + `actionTo` or `onAction`).

For several queries, pass a merged object:
`query={{ data: b.data, error: a.error ?? b.error, refetch }}`.

In-flight mutations and background jobs (file upload, AI generation) show an
inline `role="status"` line with a spinning `Loader2` icon, not a page
spinner.

### Lists and tables

- Below `md`, wide tables (athletes, users, workouts, session logs, season
  phases, programs) render as a `CardList` of `CardListItem`s, and the table is
  wrapped in `DesktopOnly`. Narrow tables (2 to 3 columns) can stay tables.
- A clickable card or row is a real link. Cards take `to`. Table rows put a
  `RowLink` in the first cell; it stretches over the row (`relative` on the
  `TableRow`).
- Row actions are separate buttons (outside the link on cards, via
  `actions`). Icon-only buttons always carry an `aria-label` that names the
  item (`Delete check-in Sat, Nov 21`).
- Action columns get a visually hidden header (`<span className="sr-only">Actions</span>`).

## Forms

### Field (`components/Field.tsx`)

`Field` renders label, control, hint and error with the wiring done:

```tsx
<Field label="Weight" required hint="Pounds" error={errors.weight}>
  {control => <Input type="number" inputMode="decimal" required {...control} />}
</Field>
```

- The render-function child receives `id`, `aria-describedby` (hint and
  error), `aria-invalid` and `aria-required`. With a plain child, pass `id`
  to both `Field` and the control.
- Required fields show a red asterisk (hidden from screen readers) and the
  control carries `required`, so assistive tech announces it once.
- Selects: put the id on `SelectTrigger` (or `EnumSelect`'s `id` prop). The
  trigger is a button, which a `<label htmlFor>` names.
- `ExercisePicker` takes `triggerLabelId`; give the label that id.
- Groups of checkboxes or radios use `<fieldset>` + `<legend>` (or
  `role="group"` with `aria-labelledby`). A `Label` that wraps its checkbox is
  already associated.
- Number inputs set `inputMode` (`decimal` for weight, `numeric` for reps).
- Default dates use `localDateISO()`, never `toISOString()` (which flips the
  day near UTC midnight).

## Touch sizes

`Button`, `Input` and `SelectTrigger` are 44px (`h-11` / `size-11`) below `md`
and compact (`h-8`) from `md` up. Sizes `default`, `sm`, `lg`, `icon`,
`icon-sm` and `icon-lg` all follow that rule; only `xs` stays small and is for
dense desktop-only chrome. `touch` and `icon-touch` are 44px at every width,
for the logging surfaces (session screen, set rows). Custom tap targets
(chips, inline links in cards, checkbox labels) add `min-h-11 md:min-h-0`.

## Icons

- lucide-react for all UI chrome: nav items, buttons, toggles, status
  (`Check`, `X`, `AlertTriangle`), disclosure (`ChevronDown`/`ChevronRight`),
  pagination and "View all" arrows. Decorative icons get `aria-hidden="true"`.
- Emoji stay for content: the streak 🔥, PR 🎉 toasts, success ✅ cards and
  `EmptyState` icons. Mark decorative emoji `aria-hidden` and keep the meaning
  in text (`<span className="sr-only"> streak</span>`).
- Do not use text glyphs (`×`, `✓`, `→`, `▾`) as button content.

## Dates

`lib/dates.ts` formats calendar dates for headings and lists:

- `useFriendlyDate()` returns a formatter bound to the user's preferences
  (date format and time zone from `/api/preferences`).
- Output: `Today`, `Yesterday`, `Tomorrow`, otherwise `Sat, Nov 21` this year
  and `Fri, Nov 21, 2025` for other years. The numeric preference layouts keep
  their order (`Sat, 11/21`, `Sat, 21/11`, `Sat, 2026-11-21`).
- `{ absolute: true }` skips the relative words (headings, date ranges).
- `{ timestamp: true }` converts an instant (`created_at`, `revoked_at`) to
  the user's local day first. Plain dates (`workout.date`) are calendar days
  and are never shifted through `Date`.
- `formatFriendlyDate(iso, opts)` is the pure function for tests and for
  places outside React.

Never show a raw ISO date (`2026-11-21`) in a heading or list.

## Feedback

- Mutations toast on failure through the global `MutationCache` handler in
  `main.tsx`. Opt out with `meta: { skipGlobalError: true }` when the page
  shows the error inline.
- Successful saves toast (`toast.success('Training maxes saved')`) instead of
  temporary inline alerts.
- Destructive or one-way actions confirm with `useConfirm` and name the thing
  and the consequence (`Promote to Intermediate?`).
- Toasts sit top-center on phones (clear of the rest timer) and bottom-right
  on desktop, lifted while the timer runs.

## Plate setup

Each athlete has a barbell loading setup (`internal/models/plate_setup.go`):
bar, optional collars and a plate inventory, each with its own unit,
separate from the logging unit in preferences. The default is the home kg
competition setup (20 kg bar, optional 2.5 kg collars, 25 to 0.5 kg pairs),
with a standard lb preset (45 lb bar; 45/35/25/10/5/2.5 lb plates). Coaches
set it for kids, who may share a home gym.

UI rules for anything that loads a bar (plate calculator, warm-up ramps,
"round to loadable"):

- Work in the plate unit, display in the logging unit. Convert with
  1 kg = 2.20462 lb and show loaded weights to 0.1 lb.
- Show plates per side, the actual loaded weight and the difference from the
  target, with an explicit "Use 225.9 lb" action. Never change a logged or
  prescribed weight without the athlete or coach tapping it.
- Offer the nearest loadable weights above and below the target.

## Accessibility checklist

- One `<h1>` per page (via `PageHeader`); section headings step down to `h2`.
- Every control has a programmatic label; every icon-only button has an
  `aria-label`.
- Clickable things are links or buttons and are reachable by keyboard; focus
  is visible (`focus-visible:ring-3 ring-ring/50`).
- Status and errors that appear in place use `role="status"` or
  `role="alert"`.
- Text uses the `-strong` success and warning tokens on light backgrounds.
