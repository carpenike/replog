# ADR 006 — Workout Import / Export

> Status: **Accepted** · Date: 2026-02-17

## Context

Users asked for the ability to import historical workout data from other apps (Strong, Hevy, FitNotes, StrongLifts, etc.) and to export RepLog data for backup, analysis, or migration.

### Industry Landscape

**There is no standard format for strength training data exchange.** Unlike cardio/GPS activities which have mature standards (TCX, GPX, FIT/ANT+), the strength training ecosystem relies entirely on app-specific CSV exports. Every app rolls its own schema.

#### Common Export Formats Observed

| App | Format | Key Columns |
|-----|--------|-------------|
| **Strong** | CSV | `Date`, `Workout Name`, `Duration`, `Exercise Name`, `Set Order`, `Weight`, `Reps`, `Distance`, `Seconds`, `Notes`, `Workout Notes`, `RPE` |
| **Hevy** | CSV | `title`, `start_time`, `end_time`, `description`, `exercise_title`, `superset_id`, `exercise_notes`, `set_index`, `set_type`, `weight_lbs`, `reps`, `distance_miles`, `duration_seconds`, `rpe` |
| **FitNotes (iOS)** | CSV | `Date`, `Exercise`, `Category`, `Weight (kg)`, `Weight (lbs)`, `Reps`, `Distance`, `Distance Unit`, `Time`, `Notes`, `Kind` |
| **StrongLifts** | CSV | Workout date, number, name, program, body weight, exercise, sets/reps, top set, e1RM, volume, duration, notes, per-set breakdown |

#### Common Patterns Across Apps

Despite different schemas, all formats share these concepts:
- **One row = one set** (per-set logging is universal)
- **Exercise identified by name** (no standard exercise IDs)
- **Date/time of workout** (format varies wildly)
- **Weight + reps** as the core data pair
- **Optional: RPE, notes, duration, distance**

#### What Doesn't Exist
- No open standard or RFC for strength training data
- No equivalent of GPX/TCX for resistance training
- No community-driven schema standardization effort
- Most apps offer export but **not** re-import of their own format (Strong explicitly says "exported files cannot be imported back")

#### What Other Apps Don't Cover

External CSVs contain **only workout log data** (sets, reps, weight). None of them include:
- Equipment catalogs or exercise-equipment dependencies
- Training maxes or progression history
- Exercise assignments or athlete profiles
- Program templates or prescribed sets
- Body weight history
- Workout reviews

These are all RepLog-specific concepts that the **RepLog Native JSON** format must capture in full.

## Decision

### Export

RepLog will export data in **two formats**:

1. **RepLog Native JSON** — full-fidelity export of all data for an athlete, suitable for backup and re-import into RepLog. This is a **complete snapshot** including: athlete profile, exercises (with equipment dependencies), equipment catalog, athlete equipment inventory, exercise assignments, training maxes, body weights, workouts (with sets and reviews), and program assignments.

2. **Strong-compatible CSV** — the de facto interchange format. Strong's CSV schema is the most widely supported import target (Hevy, FitNotes, Ryot, Intervals.icu, and many others can import it). This maximizes portability at the cost of losing RepLog-specific data (equipment, assignments, training maxes, rep_type, etc.).

### Import

RepLog will support importing from **three sources**, in priority order:

1. **RepLog Native JSON** — restore from backup, migrate between instances (full-fidelity)
2. **Strong CSV** — the most common export format from other apps (workout log only)
3. **Hevy CSV** — second most popular strength training app (workout log only)

### What Gets Exported / Imported

| Data Domain | RepLog JSON Export | RepLog JSON Import | Strong/Hevy CSV Import |
|---|---|---|---|
| Athlete profile | ✅ name, tier, notes, goal | ✅ | ❌ (target athlete already exists) |
| Equipment catalog | ✅ full catalog w/ descriptions | ✅ with mapping | ❌ |
| Exercise-equipment dependencies | ✅ required + optional links | ✅ with mapping | ❌ |
| Athlete equipment inventory | ✅ what they have | ✅ with mapping | ❌ |
| Exercises | ✅ all referenced exercises | ✅ with mapping | ✅ with mapping |
| Exercise assignments | ✅ active + historical | ✅ | ❌ |
| Training maxes | ✅ full history | ✅ | ❌ |
| Body weights | ✅ full history | ✅ | ❌ |
| Workouts + sets | ✅ all fields | ✅ | ✅ with mapping |
| Workout reviews | ✅ status + notes | ✅ | ❌ |
| Program templates | ✅ if athlete has assignment | ✅ with mapping | ❌ |
| Progression rules | ✅ per template | ✅ | ❌ |

### RepLog Native JSON Schema

```json
{
  "version": "1.0",
  "exported_at": "2026-02-17T15:04:05Z",
  "weight_unit": "lbs",

  "athlete": {
    "name": "Caydan",
    "tier": "foundational",
    "notes": "Ready to try intermediate bench",
    "goal": "Build overall strength",
    "track_body_weight": true
  },

  "equipment": [
    {
      "name": "Barbell",
      "description": "Standard Olympic barbell, 45 lbs"
    },
    {
      "name": "Flat Bench",
      "description": null
    },
    {
      "name": "Squat Rack",
      "description": "Full power rack with safety pins"
    },
    {
      "name": "Dumbbells",
      "description": "Adjustable 5-75 lbs"
    }
  ],

  "athlete_equipment": ["Barbell", "Flat Bench", "Squat Rack", "Dumbbells"],

  "exercises": [
    {
      "name": "Bench Press",
      "tier": "foundational",
      "form_notes": "Keep elbows tucked, feet flat on floor",
      "demo_url": "https://example.com/bench-press",
      "rest_seconds": 120,
      "equipment": [
        { "name": "Barbell", "optional": false },
        { "name": "Flat Bench", "optional": false },
        { "name": "Squat Rack", "optional": true }
      ]
    },
    {
      "name": "Dumbbell Bench Press",
      "tier": null,
      "form_notes": null,
      "demo_url": null,
      "rest_seconds": 90,
      "equipment": [
        { "name": "Dumbbells", "optional": false },
        { "name": "Flat Bench", "optional": false }
      ]
    }
  ],

  "assignments": [
    {
      "exercise": "Bench Press",
      "target_reps": 5,
      "active": true,
      "assigned_at": "2026-01-01T00:00:00Z",
      "deactivated_at": null
    }
  ],

  "training_maxes": [
    {
      "exercise": "Bench Press",
      "weight": 135.0,
      "effective_date": "2026-01-15",
      "notes": "Initial TM at 85% of tested 1RM"
    }
  ],

  "body_weights": [
    {
      "date": "2026-02-01",
      "weight": 155.5,
      "notes": null
    }
  ],

  "workouts": [
    {
      "date": "2026-02-15",
      "notes": "Felt strong today",
      "review": {
        "status": "approved",
        "notes": "Great form on the bench. Push harder on accessories next time."
      },
      "sets": [
        {
          "exercise": "Bench Press",
          "set_number": 1,
          "reps": 5,
          "rep_type": "reps",
          "weight": 115.0,
          "rpe": 7.5,
          "notes": null
        },
        {
          "exercise": "Bench Press",
          "set_number": 2,
          "reps": 5,
          "rep_type": "reps",
          "weight": 115.0,
          "rpe": 8.0,
          "notes": "Slight grind on rep 5"
        }
      ]
    }
  ],

  "programs": [
    {
      "template": {
        "name": "5/3/1 BBB",
        "description": "Wendler 5/3/1 Boring But Big",
        "num_weeks": 4,
        "num_days": 4,
        "prescribed_sets": [
          {
            "exercise": "Bench Press",
            "week": 1,
            "day": 1,
            "set_number": 1,
            "reps": 5,
            "rep_type": "reps",
            "percentage": 65.0,
            "notes": null
          }
        ],
        "progression_rules": [
          {
            "exercise": "Bench Press",
            "increment": 5.0
          }
        ]
      },
      "start_date": "2026-01-01",
      "active": true,
      "notes": "First cycle",
      "goal": "Increase bench TM by 10 lbs"
    }
  ]
}
```

### Strong CSV Mapping (Export)

RepLog data maps to Strong CSV columns as follows:

| Strong Column | RepLog Source |
|---------------|-------------|
| `Date` | `workouts.date` formatted as `YYYY-MM-DD HH:MM:SS` |
| `Workout Name` | `athlete.name + " — " + workouts.date` |
| `Duration` | empty (RepLog doesn't track duration) |
| `Exercise Name` | `exercises.name` |
| `Set Order` | `workout_sets.set_number` |
| `Weight` | `workout_sets.weight` |
| `Reps` | `workout_sets.reps` (for `rep_type = "reps"` or `"each_side"`) |
| `Distance` | empty |
| `Seconds` | `workout_sets.reps` (when `rep_type = "seconds"`) |
| `Notes` | `workout_sets.notes` |
| `Workout Notes` | `workouts.notes` |
| `RPE` | `workout_sets.rpe` |

### Strong CSV Field Mapping (Import)

| Strong Column | RepLog Target |
|---------------|--------------|
| `Date` | `workouts.date` (parsed, one workout per unique date) |
| `Workout Name` | ignored (RepLog uses athlete+date) |
| `Duration` | ignored |
| `Exercise Name` | → **mapping step** (see below) |
| `Set Order` | `workout_sets.set_number` |
| `Weight` | `workout_sets.weight` |
| `Reps` | `workout_sets.reps` with `rep_type = "reps"` |
| `Distance` | ignored |
| `Seconds` | if `Reps` is empty and `Seconds` > 0 → `workout_sets.reps` with `rep_type = "seconds"` |
| `Notes` | `workout_sets.notes` |
| `Workout Notes` | `workouts.notes` (first occurrence per workout wins) |
| `RPE` | `workout_sets.rpe` |

### Hevy CSV Field Mapping (Import)

| Hevy Column | RepLog Target |
|-------------|--------------|
| `start_time` | `workouts.date` (date portion only) |
| `title` | ignored |
| `exercise_title` | → **mapping step** (see below) |
| `set_index` | `workout_sets.set_number` (offset by +1 since Hevy is 0-indexed) |
| `set_type` | `"warmup"` → skip or annotate in notes; `"normal"` → normal set |
| `weight_lbs` | `workout_sets.weight` |
| `reps` | `workout_sets.reps` |
| `duration_seconds` | if reps is empty → `workout_sets.reps` with `rep_type = "seconds"` |
| `exercise_notes` | `workout_sets.notes` |
| `description` | `workouts.notes` |
| `rpe` | `workout_sets.rpe` |

## Architecture

### Scope

**Export** is per-athlete — coaches select an athlete and export their complete profile. The RepLog JSON export includes all related catalog data (exercises, equipment, programs) referenced by that athlete's records.

**Import** operates at two levels:
- **RepLog JSON** — imports a complete athlete profile including catalog data (equipment, exercises, programs). Entities are matched to existing records or created via the mapping step.
- **Strong/Hevy CSV** — imports workout log data into an existing athlete. Exercises are mapped via the mapping step.

### Access Control

- **Export**: Admin and coach can export any athlete's data. Non-coach users can export their own linked athlete's data.
- **Import**: Admin and coach only. Importing creates exercises, equipment, and workout data, which are coaching decisions.

### Routes

```
GET  /athletes/{id}/export          → export options page
GET  /athletes/{id}/export/json     → download RepLog JSON
GET  /athletes/{id}/export/csv      → download Strong-compatible CSV

GET  /athletes/{id}/import          → import upload page (file select + format)
POST /athletes/{id}/import/upload   → parse file, redirect to mapping step
GET  /athletes/{id}/import/map      → mapping UI (htmx-driven)
POST /athletes/{id}/import/preview  → dry-run summary after mapping confirmed
POST /athletes/{id}/import/execute  → apply import in transaction
```

### The Mapping Step

**All imports go through a mandatory mapping step.** This is the core UX differentiation from other apps that blindly auto-create entities by string matching.

After the file is parsed, the app extracts all unique entity names and presents a mapping UI for each entity type:

#### Exercise Mapping

For each unique exercise name found in the import file:

```
Import File              →  RepLog Exercise
─────────────────────────────────────────────
"Bench Press"            →  [Bench Press (id: 3)     ▼]  ✅ matched
"Barbell Bench Press"    →  [Bench Press (id: 3)     ▼]  🔀 manual map
"DB Rows"                →  [Dumbbell Row (id: 12)   ▼]  🔀 manual map
"Cable Face Pull"        →  [+ Create New Exercise   ▼]  ➕ new
"Leg Press"              →  [+ Create New Exercise   ▼]  ➕ new
```

Each row shows:
- The name as it appears in the import file
- A dropdown listing all existing RepLog exercises (sorted by similarity), plus a "Create New Exercise" option
- A status icon indicating: exact match (auto-mapped), manual map (user chose a different exercise), or new (will be created)

The app **pre-populates** mappings using case-insensitive exact match. Any exercise name that doesn't match exactly is left unmapped — the user must explicitly map it or confirm creation. **No exercise is auto-created without user confirmation.**

#### Equipment Mapping (RepLog JSON Import Only)

Same pattern for equipment when importing RepLog JSON files:

```
Import File              →  RepLog Equipment
─────────────────────────────────────────────
"Barbell"                →  [Barbell (id: 1)         ▼]  ✅ matched
"Olympic Barbell"        →  [Barbell (id: 1)         ▼]  🔀 manual map
"Flat Bench"             →  [Flat Bench (id: 2)      ▼]  ✅ matched
"Resistance Bands"       →  [+ Create New Equipment  ▼]  ➕ new
```

Equipment mapping is critical because exercise-equipment dependencies link by ID, not by name. If "Bench Press" requires equipment "Barbell" in the import file, and the user maps "Barbell" → existing equipment ID 1, then the dependency is wired correctly.

#### Program Template Mapping (RepLog JSON Import Only)

```
Import File              →  RepLog Program Template
─────────────────────────────────────────────────────
"5/3/1 BBB"              →  [5/3/1 BBB (id: 1)      ▼]  ✅ matched
"Custom GZCL"            →  [+ Create New Template   ▼]  ➕ new
```

If a template maps to an existing one, the prescribed sets and progression rules from the import are **skipped** (the existing template's definition is authoritative). Only the `athlete_programs` assignment is created.

If a template is created new, all its prescribed sets and progression rules are imported.

#### Mapping Persistence

The mapping state is stored in the user's session between the upload and execute steps. This allows the user to navigate back and adjust mappings without re-uploading. Session data is cleared after successful import or after 1 hour (whichever comes first).

### Import Workflow (UX)

#### Strong / Hevy CSV Import

1. Coach navigates to athlete → "Import Data"
2. Selects file + format (auto-detected from CSV headers when possible)
3. Selects weight unit (lbs/kg) — applies to all weight values in the file
4. **Mapping step**: app presents exercise mapping UI
   - Exact name matches are pre-selected
   - Unmatched exercises default to "Create New Exercise"
   - User reviews every mapping, adjusts as needed, confirms
5. **Preview step**: after mapping confirmed, app shows:
   - Number of workouts to import (with date range)
   - Number of exercises mapped → existing vs. new
   - Conflicts — dates that already have workouts logged
   - Estimated sets to import
6. Coach confirms
7. Import executes in a single transaction
8. Success page with summary (X workouts, Y exercises created, Z sets imported)

#### RepLog JSON Import

1. Coach navigates to athlete → "Import Data"
2. Uploads RepLog JSON file
3. **Mapping step**: multi-tab mapping UI
   - **Equipment tab**: map imported equipment → existing or create new
   - **Exercises tab**: map imported exercises → existing or create new (shows equipment dependencies for each)
   - **Programs tab**: map imported templates → existing or create new
4. **Preview step**: comprehensive summary
   - Athlete profile fields that will be updated (diff view)
   - Equipment: matched, new to create
   - Exercises: matched, new to create (with equipment dependencies)
   - Assignments to import
   - Training maxes to import
   - Body weights to import (with conflict count)
   - Workouts to import (with conflict count)
   - Reviews to import
   - Programs to assign
5. Coach confirms
6. Import executes in a single transaction
7. Success page with summary

### Conflict Resolution

- **Existing workout on same date**: skip by default, with option to merge (append sets to existing workout)
- **Existing body weight on same date**: skip (existing data wins)
- **Existing training max on same date+exercise**: skip (existing data wins)
- **Duplicate assignment**: skip if an active assignment already exists for the same exercise
- **Unit declaration**: user selects weight unit (lbs/kg) before import — applies to all weight values
- **Athlete profile**: on RepLog JSON import, optionally update athlete's tier/notes/goal from the import file (checkbox, off by default — existing profile wins)

### Implementation Layers

```
internal/
  models/
    import_export.go    # Data structures for import/export payloads
  handlers/
    import_export.go    # HTTP handlers for upload, mapping, preview, download
  importers/
    replog.go           # RepLog JSON parser + mapper
    strong.go           # Strong CSV parser
    hevy.go             # Hevy CSV parser
    common.go           # Shared types: ParsedWorkout, ParsedSet, EntityMapping, etc.
    mapper.go           # Entity matching logic (exact, case-insensitive, similarity scoring)
```

### File Size Limits

- Max upload: **10 MB** (a year of daily training in Strong CSV is ~500 KB)
- Parse in memory — no streaming needed at this scale

## Consequences

- RepLog can accept data from the two most popular strength training apps
- The Strong-compatible export makes RepLog data portable to most other apps
- The native JSON format provides lossless backup/restore of all athlete data including equipment, assignments, programs, and reviews
- The mandatory mapping step prevents accidental entity duplication — "DB Bench" doesn't silently create a new exercise when "Dumbbell Bench Press" already exists
- Equipment dependencies are preserved through ID-based mapping — the import never relies on string equality for FK relationships
- The mapping UI adds an extra step compared to "upload and go" but prevents data quality issues that are harder to fix after the fact
- No automated unit conversion — the user must declare which unit the import file uses
- Warm-up sets from Hevy (`set_type = "warmup"`) are imported as regular sets with a note annotation, since RepLog doesn't distinguish set types
- Exercise matching pre-population uses case-insensitive exact match only — no fuzzy matching in v1 (keeps logic simple and predictable)

## Future Considerations

- **FitNotes CSV import**: third priority, straightforward to add with the `importers/` pattern
- **Bulk export of all athletes**: admin-level backup, exports one JSON per athlete (or a ZIP)
- **Full instance backup**: admin-level export of all data (athletes, users, catalog) — useful for migration between RepLog instances
- **Exercise synonym table**: improves auto-matching in the mapping step ("Bench Press" ↔ "Barbell Bench Press" ↔ "Flat Bench")
- **Fuzzy matching in mapping UI**: Levenshtein distance or token overlap scoring to suggest likely matches for unmatched exercises — but always as suggestions, never auto-applied
- **Equipment seed catalog**: a baseline set of common equipment (Barbell, Dumbbells, Squat Rack, Pull-up Bar, etc.) that can be imported into a fresh instance — not part of the import/export system per se, but could ship as a bundled RepLog JSON fragment
- **Apple Health / Google Fit integration**: out of scope — these platforms have poor support for per-set strength data
- **If an open standard emerges**: adopt it as a fourth export format — the architecture supports pluggable importers/exporters
