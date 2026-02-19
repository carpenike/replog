# Data Model

> RepLog schema — designed during bootstrap Feb 2026, updated for v1.1 enhancements.

## Design Decisions

These were resolved interactively before schema design:

1. **Tier lives on both athlete and exercise.** Exercise tier is classification (lunges are Foundational). Athlete tier is the coach's current assessment. The assignment table is the source of truth for "what does this person do today" — tier is not a hard constraint.

2. **Renamed `kids` → `athletes`.** The app tracks both kids (tier-based progression) and adults (percentage-based programs like 5/3/1). Tier is nullable — adults don't need it.

3. **Logging is per-set.** One row = one set = reps + weight. You can see set-to-set fatigue and partial completions. Easy to aggregate up, impossible to disaggregate down.

4. **A thin `workouts` table groups sets.** Without it you'd query by date and hope timestamps cluster. A workout row gives you a clean FK, a place for session-level notes, and simple history queries.

5. **Equipment is a shared catalog with per-athlete inventory.** Equipment items (barbell, squat rack, dumbbells, etc.) are defined once in a shared catalog managed by coaches. Each exercise can list required and optional equipment. Each athlete maintains an inventory of available equipment. The app can then determine whether an athlete has the required equipment for a given exercise — useful for filtering assignments and flagging compatibility issues.

6. **Training maxes are a first-class entity.** Required for percentage-based programs (5/3/1, GZCL). Multiple rows per exercise track TM progression over time. Even without a program engine, seeing "that was 85% of my TM" is useful.

7. **Assignments use an `active` flag, not hard deletes.** Deactivating preserves history. `assigned_at` / `deactivated_at` give you a timeline. Reactivation creates a **new row** (not a flag flip) — the new `assigned_at` reflects the reactivation date, preserving the full audit trail.

8. **Athletes can log any exercise, not just assigned ones.** The daily view highlights assigned exercises, but the logging UI has access to the full exercise library. This allows accessory work, one-off movements, and trying new exercises without formal assignment.

9. **Users and athletes are separate entities.** Users are login accounts (username + password hash + email). Athletes are training subjects. A user links to an athlete via `athlete_id` — coaches can manage all athletes, non-coaches can only view/log their own. The bootstrap logic auto-creates the first user (as coach) from env vars on first run.

10. **Three-tier access control: admin, coach, athlete.** Admins see and manage all athletes and users. Coaches see only athletes assigned to them (`coach_id`). Non-coach users (athletes) are linked to exactly one athlete and can only view/log/edit their own workouts. Roles overlap — an admin can also be a coach, and an athlete can also be a coach. The `is_admin` and `is_coach` flags on the users table control permissions.

11. **Program templates are separate from the logbook.** The app's core is a logbook — it records what happened. Program templates layer on a prescription engine: coaches define templates (weeks × days × prescribed sets with percentages), assign them to athletes, and the app calculates today's target weights from training maxes. Position advances by counting completed workouts since assignment start, and cycles repeat automatically.

12. **Foreign key delete behaviors are intentional.** Deleting an athlete cascades to their workouts, assignments, and training maxes. Deleting a user only unlinks their athlete profile (`SET NULL`). Deleting an exercise is restricted (`RESTRICT`) if it has been logged in any workout — prevents orphaned history.

13. **Two-level goals: long-term and per-cycle.** The `goal` column on `athletes` holds a long-term training objective ("build overall strength"). The `goal` column on `athlete_programs` holds a short-term cycle-specific goal ("increase squat TM by 10 lbs"). Both are nullable free-text fields. This separation gives future LLM-based plan generation the right context at each level.

14. **Rep type tracks per-side, timed, and distance sets.** The `rep_type` column on `prescribed_sets` and `workout_sets` uses an enum (`reps`, `each_side`, `seconds`, `distance`). This avoids encoding modifiers in notes fields — "5/ea", "30s", or "20yd" are first-class data. The `reps` column continues to hold the numeric value; `rep_type` determines how to display it.

15. **Progression rules are suggestions, not automation.** The `progression_rules` table stores per-exercise TM increment amounts for each program template (e.g. +5 lbs for upper body, +10 lbs for lower body in 5/3/1). At cycle boundaries, the app surfaces suggested TM bumps alongside AMRAP results from the completed cycle. The coach decides whether to apply, edit, or skip each suggestion. The app never auto-applies TM changes — this preserves the "logbook, not coach" principle while removing the friction of manually remembering increment rules.

16. **Prescribed sets support both percentage-based and fixed-weight programs.** Percentage-based programs (5/3/1, GZCL) use `percentage` to derive target weight from training maxes. Fixed-weight programs (Yessis 1×20, accessories) use `absolute_weight` to prescribe a specific load in pounds/kg. When both are set, percentage takes priority. Coach-controlled `sort_order` determines exercise display order within a day — critical for Yessis methodology where exercise sequence matters (compound → isolation → specialized). The `is_loop` flag on templates marks indefinite cycling programs (Yessis foundational phases) that repeat until the coach decides to advance the athlete.

17. **Journal is a read-only timeline, not a separate data store.** The journal view (`/athletes/{id}/journal`) aggregates dated events from existing tables — workouts, body weights, training max changes, goal changes, tier changes, program starts, and reviews — into a unified chronological feed via `UNION ALL`. The only new write paths are `athlete_notes` (coach free-text notes) and `tier_history` (automatic tier change recording). No denormalized journal table exists.

18. **Coach notes have public/private visibility.** The `is_private` flag on `athlete_notes` controls whether non-coach athletes can see a note. Private notes (`is_private = 1`) are coach-only; public notes (`is_private = 0`) appear on the athlete's journal view. This lets coaches keep internal observations (e.g., "watch for overtraining signs") separate from athlete-facing notes (e.g., "great progress on squat form").

## Entity Relationship Diagram

```mermaid
erDiagram
    users {
        INTEGER id PK
        TEXT username UK "COLLATE NOCASE"
        TEXT name "nullable"
        TEXT email UK "COLLATE NOCASE, nullable"
        TEXT password_hash "nullable"
        INTEGER athlete_id FK "nullable"
        INTEGER is_coach "0 or 1"
        INTEGER is_admin "0 or 1"
        TEXT avatar_path "nullable"
        DATETIME created_at
        DATETIME updated_at
    }

    user_preferences {
        INTEGER id PK
        INTEGER user_id FK "UNIQUE"
        TEXT weight_unit "lbs or kg"
        TEXT timezone "IANA timezone"
        TEXT date_format "Go format string"
        DATETIME created_at
        DATETIME updated_at
    }

    athletes {
        INTEGER id PK
        TEXT name "COLLATE NOCASE"
        TEXT tier "nullable"
        TEXT notes "nullable"
        TEXT goal "nullable"
        INTEGER coach_id FK "nullable"
        INTEGER track_body_weight "0 or 1, default 1"
        DATETIME created_at
        DATETIME updated_at
    }

    exercises {
        INTEGER id PK
        TEXT name UK "COLLATE NOCASE"
        TEXT tier "nullable"
        TEXT form_notes "nullable"
        TEXT demo_url "nullable"
        INTEGER rest_seconds "nullable"
        INTEGER featured "0 or 1, default 0"
        DATETIME created_at
        DATETIME updated_at
    }

    athlete_exercises {
        INTEGER id PK
        INTEGER athlete_id FK
        INTEGER exercise_id FK
        INTEGER target_reps "nullable"
        INTEGER active "0 or 1"
        DATETIME assigned_at
        DATETIME deactivated_at "nullable"
    }

    training_maxes {
        INTEGER id PK
        INTEGER athlete_id FK
        INTEGER exercise_id FK
        REAL weight
        DATE effective_date
        TEXT notes "nullable"
        DATETIME created_at
    }

    workouts {
        INTEGER id PK
        INTEGER athlete_id FK
        DATE date
        TEXT notes "nullable"
        DATETIME created_at
        DATETIME updated_at
    }

    workout_sets {
        INTEGER id PK
        INTEGER workout_id FK
        INTEGER exercise_id FK
        INTEGER set_number
        INTEGER reps
        TEXT rep_type "reps, each_side, seconds, or distance"
        REAL weight "nullable"
        REAL rpe "nullable, CHECK 1-10"
        TEXT notes "nullable"
        DATETIME created_at
        DATETIME updated_at
    }

    body_weights {
        INTEGER id PK
        INTEGER athlete_id FK
        DATE date
        REAL weight
        TEXT notes "nullable"
        DATETIME created_at
    }

    users ||--o| athletes : "linked profile"
    users ||--o{ athletes : "coaches"
    users ||--o| user_preferences : "has preferences"
    athletes ||--o{ athlete_exercises : "has"
    exercises ||--o{ athlete_exercises : "assigned via"
    athletes ||--o{ training_maxes : "has"
    exercises ||--o{ training_maxes : "for"
    athletes ||--o{ workouts : "logs"
    workouts ||--o{ workout_sets : "contains"
    exercises ||--o{ workout_sets : "performed"
    athletes ||--o{ body_weights : "tracks"
    athletes ||--o{ goal_history : "goal changes"
    users ||--o{ goal_history : "set by"
    athletes ||--o{ tier_history : "tier changes"
    users ||--o{ tier_history : "set by"
    athletes ||--o{ athlete_notes : "notes"
    users ||--o{ athlete_notes : "authored by"
    workouts ||--o| workout_reviews : "reviewed via"
    users ||--o{ workout_reviews : "reviews"
    program_templates ||--o{ prescribed_sets : "defines"
    exercises ||--o{ prescribed_sets : "used in"
    athletes ||--o{ athlete_programs : "follows"
    program_templates ||--o{ athlete_programs : "assigned via"
    program_templates ||--o{ progression_rules : "has rules"
    exercises ||--o{ progression_rules : "incremented by"
    users ||--o{ login_tokens : "has"
    users ||--o{ webauthn_credentials : "has"
    equipment ||--o{ exercise_equipment : "required by"
    exercises ||--o{ exercise_equipment : "requires"
    equipment ||--o{ athlete_equipment : "owned by"
    athletes ||--o{ athlete_equipment : "has"

    login_tokens {
        INTEGER id PK
        INTEGER user_id FK
        TEXT token UK
        TEXT label "nullable"
        DATETIME expires_at "nullable"
        DATETIME created_at
    }

    webauthn_credentials {
        INTEGER id PK
        INTEGER user_id FK
        BLOB credential_id UK
        BLOB public_key
        TEXT attestation_type
        TEXT transport "nullable"
        INTEGER sign_count
        INTEGER clone_warning "0 or 1"
        TEXT attachment
        BLOB aaguid "nullable"
        INTEGER flags_user_present "0 or 1"
        INTEGER flags_user_verified "0 or 1"
        INTEGER flags_backup_eligible "0 or 1"
        INTEGER flags_backup_state "0 or 1"
        TEXT label "nullable"
        DATETIME created_at
    }

    goal_history {
        INTEGER id PK
        INTEGER athlete_id FK
        TEXT goal
        TEXT previous_goal "nullable"
        INTEGER set_by FK "nullable"
        DATE effective_date
        TEXT notes "nullable"
        DATETIME created_at
    }

    tier_history {
        INTEGER id PK
        INTEGER athlete_id FK
        TEXT tier
        TEXT previous_tier "nullable"
        INTEGER set_by FK "nullable"
        DATE effective_date
        TEXT notes "nullable"
        DATETIME created_at
    }

    athlete_notes {
        INTEGER id PK
        INTEGER athlete_id FK
        INTEGER author_id FK "nullable"
        DATE date
        TEXT content
        INTEGER is_private "0 or 1, default 0"
        INTEGER pinned "0 or 1, default 0"
        DATETIME created_at
        DATETIME updated_at
    }

    sessions {
        TEXT token PK
        BLOB data
        REAL expiry
    }

    equipment {
        INTEGER id PK
        TEXT name UK "COLLATE NOCASE"
        TEXT description "nullable"
        DATETIME created_at
        DATETIME updated_at
    }

    exercise_equipment {
        INTEGER id PK
        INTEGER exercise_id FK
        INTEGER equipment_id FK
        INTEGER optional "0 or 1"
    }

    athlete_equipment {
        INTEGER id PK
        INTEGER athlete_id FK
        INTEGER equipment_id FK
    }

    workout_reviews {
        INTEGER id PK
        INTEGER workout_id FK "UNIQUE"
        INTEGER coach_id FK "nullable"
        TEXT status "approved or needs_work"
        TEXT notes "nullable"
        DATETIME created_at
        DATETIME updated_at
    }

    program_templates {
        INTEGER id PK
        TEXT name UK "COLLATE NOCASE"
        TEXT description "nullable"
        INTEGER num_weeks
        INTEGER num_days
        INTEGER is_loop "0 or 1, default 0"
        DATETIME created_at
        DATETIME updated_at
    }

    prescribed_sets {
        INTEGER id PK
        INTEGER template_id FK
        INTEGER exercise_id FK
        INTEGER week
        INTEGER day
        INTEGER set_number
        INTEGER reps "nullable, NULL = AMRAP"
        TEXT rep_type "reps, each_side, seconds, or distance"
        REAL percentage "nullable"
        REAL absolute_weight "nullable, fixed weight"
        INTEGER sort_order "display order within day"
        TEXT notes "nullable"
    }

    athlete_programs {
        INTEGER id PK
        INTEGER athlete_id FK
        INTEGER template_id FK
        DATE start_date
        INTEGER active "0 or 1"
        TEXT notes "nullable"
        TEXT goal "nullable"
        DATETIME created_at
        DATETIME updated_at
    }

    progression_rules {
        INTEGER id PK
        INTEGER template_id FK
        INTEGER exercise_id FK
        REAL increment "TM bump amount"
    }
```

## Schema

### `users`

| Column          | Type         | Constraints                          |
|----------------|-------------|--------------------------------------|
| `id`           | INTEGER      | PRIMARY KEY AUTOINCREMENT            |
| `username`     | TEXT         | NOT NULL UNIQUE COLLATE NOCASE       |
| `name`         | TEXT         | NULL                                 |
| `email`        | TEXT         | NULL UNIQUE COLLATE NOCASE            |
| `password_hash`| TEXT         | NULL                                 |
| `athlete_id`   | INTEGER      | NULL, FK → athletes(id)              |
| `is_coach`     | INTEGER      | NOT NULL DEFAULT 0, CHECK(is_coach IN (0, 1)) |
| `is_admin`     | INTEGER      | NOT NULL DEFAULT 0, CHECK(is_admin IN (0, 1)) |
| `avatar_path`  | TEXT         | NULL                                 |
| `created_at`   | DATETIME     | NOT NULL DEFAULT CURRENT_TIMESTAMP   |
| `updated_at`   | DATETIME     | NOT NULL DEFAULT CURRENT_TIMESTAMP   |

- Login accounts, not training subjects. Separate from athletes.
- `email` for password reset or notifications in the future. Required for coaches, optional for kids.
- `athlete_id` links the user to "their" athlete profile. NULL for coach-only accounts without a personal training profile.
- `is_coach = 1` → full access to all athletes. `is_coach = 0` → can only view/log/edit workouts for their linked athlete.
- `avatar_path` stores the relative path to the user's uploaded avatar image. NULL if no avatar has been uploaded.
- `COLLATE NOCASE` prevents "Admin" and "admin" or duplicate emails.
- Bootstrap: if `COUNT(*) = 0` on startup, insert from `REPLOG_ADMIN_USER` / `REPLOG_ADMIN_PASS` / `REPLOG_ADMIN_EMAIL` env vars with `is_coach = 1`.

### `user_preferences`

| Column        | Type         | Constraints                          |
|--------------|-------------|--------------------------------------|
| `id`         | INTEGER      | PRIMARY KEY AUTOINCREMENT            |
| `user_id`    | INTEGER      | NOT NULL UNIQUE, FK → users(id) ON DELETE CASCADE |
| `weight_unit`| TEXT         | NOT NULL DEFAULT 'lbs', CHECK(weight_unit IN ('lbs', 'kg')) |
| `timezone`   | TEXT         | NOT NULL DEFAULT 'America/New_York'  |
| `date_format`| TEXT         | NOT NULL DEFAULT 'Jan 2, 2006'       |
| `created_at` | DATETIME     | NOT NULL DEFAULT CURRENT_TIMESTAMP   |
| `updated_at` | DATETIME     | NOT NULL DEFAULT CURRENT_TIMESTAMP   |

- One row per user — stores display and locale preferences.
- `weight_unit` controls how weights are labeled throughout the UI ('lbs' or 'kg'). Weights are stored in the user's chosen unit — no automatic conversion.
- `timezone` is an IANA timezone identifier (e.g. 'America/New_York', 'Europe/London'). Used for displaying dates in the user's local time.
- `date_format` is a Go `time.Format` string (e.g. 'Jan 2, 2006', '2006-01-02', '01/02/2006').
- Default preferences are seeded on login if no row exists.
- Deleting a user cascades to their preferences.

### `athletes`

| Column              | Type         | Constraints                          |
|--------------------|-------------|--------------------------------------|
| `id`               | INTEGER      | PRIMARY KEY AUTOINCREMENT            |
| `name`             | TEXT         | NOT NULL COLLATE NOCASE               |
| `tier`             | TEXT         | NULL, CHECK(tier IN ('foundational','intermediate','sport_performance')) |
| `notes`            | TEXT         | NULL                                 |
| `goal`             | TEXT         | NULL                                 |
| `coach_id`         | INTEGER      | NULL, FK → users(id)                 |
| `track_body_weight`| INTEGER      | NOT NULL DEFAULT 1, CHECK(track_body_weight IN (0, 1)) |
| `created_at`       | DATETIME     | NOT NULL DEFAULT CURRENT_TIMESTAMP   |
| `updated_at`       | DATETIME     | NOT NULL DEFAULT CURRENT_TIMESTAMP   |

- `tier` is nullable — adults running their own programs don't use the tier system.
- `notes` holds free-form coaching observations ("ready to try intermediate bench").
- `goal` holds a long-term training objective ("build overall strength", "prepare for football season"). Nullable.
- `track_body_weight` controls whether body weight tracking UI is visible for this athlete. Defaults to enabled.

### `exercises`

| Column       | Type         | Constraints                          |
|-------------|-------------|--------------------------------------|
| `id`        | INTEGER      | PRIMARY KEY AUTOINCREMENT            |
| `name`      | TEXT         | NOT NULL UNIQUE COLLATE NOCASE        |
| `tier`      | TEXT         | NULL, CHECK(tier IN ('foundational','intermediate','sport_performance')) |
| `form_notes`| TEXT         | NULL                                 |
| `demo_url`  | TEXT         | NULL                                 |
| `rest_seconds`| INTEGER    | NULL                                 |
| `featured`  | INTEGER      | NOT NULL DEFAULT 0, CHECK(featured IN (0, 1)) |
| `created_at`| DATETIME     | NOT NULL DEFAULT CURRENT_TIMESTAMP   |
| `updated_at`| DATETIME     | NOT NULL DEFAULT CURRENT_TIMESTAMP   |

- `tier` is nullable — general lifts (squat, bench, deadlift) exist independent of the kids' tier system.
- `form_notes` holds static coaching cues ("keep elbows tucked").
- `rest_seconds` is the recommended rest between sets in seconds. NULL means use the app default (90s). Passed to the client-side rest timer after logging a set.
- `demo_url` links to a video demonstrating proper form.
- `featured` marks exercises that appear on the featured lifts dashboard. Defaults to not featured.

### `athlete_exercises`

| Column          | Type         | Constraints                          |
|----------------|-------------|--------------------------------------|
| `id`           | INTEGER      | PRIMARY KEY AUTOINCREMENT            |
| `athlete_id`   | INTEGER      | NOT NULL, FK → athletes(id)          |
| `exercise_id`  | INTEGER      | NOT NULL, FK → exercises(id)         |
| `target_reps`  | INTEGER      | NULL                                 |
| `active`       | INTEGER      | NOT NULL DEFAULT 1, CHECK(active IN (0, 1)) |
| `assigned_at`  | DATETIME     | NOT NULL DEFAULT CURRENT_TIMESTAMP   |
| `deactivated_at`| DATETIME    | NULL                                 |

- `target_reps` is the per-assignment prescription — rep targets vary by athlete even for the same exercise.
- Partial unique index ensures only one active assignment per athlete+exercise at a time.
- Deactivation sets `active = 0` and populates `deactivated_at`.
- Reactivation creates a new row (preserves audit trail with fresh `assigned_at`).
- History is preserved; query `WHERE active = 1` for current assignments.

### `training_maxes`

| Column          | Type         | Constraints                          |
|----------------|-------------|--------------------------------------|
| `id`           | INTEGER      | PRIMARY KEY AUTOINCREMENT            |
| `athlete_id`   | INTEGER      | NOT NULL, FK → athletes(id)          |
| `exercise_id`  | INTEGER      | NOT NULL, FK → exercises(id)         |
| `weight`       | REAL         | NOT NULL                             |
| `effective_date`| DATE        | NOT NULL                             |
| `notes`        | TEXT         | NULL                                 |
| `created_at`   | DATETIME     | NOT NULL DEFAULT CURRENT_TIMESTAMP   |

- Multiple rows per athlete+exercise track TM progression over time.
- `effective_date` allows backdating or planning ahead.
- Current TM = most recent row by `effective_date` for a given athlete+exercise.

### `workouts`

| Column       | Type         | Constraints                          |
|-------------|-------------|--------------------------------------|
| `id`        | INTEGER      | PRIMARY KEY AUTOINCREMENT            |
| `athlete_id`| INTEGER      | NOT NULL, FK → athletes(id)          |
| `date`      | DATE         | NOT NULL                             |
| `notes`     | TEXT         | NULL                                 |
| `created_at`| DATETIME     | NOT NULL DEFAULT CURRENT_TIMESTAMP   |
| `updated_at`| DATETIME     | NOT NULL DEFAULT CURRENT_TIMESTAMP   |

- One row per training session.
- `notes` holds session-level observations ("knee was bothering her today").
- UNIQUE(athlete_id, date) — one workout per athlete per day for v1.

### `workout_sets`

| Column       | Type         | Constraints                          |
|-------------|-------------|--------------------------------------|
| `id`        | INTEGER      | PRIMARY KEY AUTOINCREMENT            |
| `workout_id`| INTEGER      | NOT NULL, FK → workouts(id)          |
| `exercise_id`| INTEGER     | NOT NULL, FK → exercises(id)         |
| `set_number`| INTEGER      | NOT NULL                             |
| `reps`      | INTEGER      | NOT NULL                             |
| `weight`    | REAL         | NULL                                 |
| `rep_type`  | TEXT         | NOT NULL DEFAULT 'reps', CHECK(rep_type IN ('reps', 'each_side', 'seconds', 'distance')) |
| `rpe`       | REAL         | NULL, CHECK(rpe >= 1 AND rpe <= 10)  |
| `notes`     | TEXT         | NULL                                 |
| `created_at`| DATETIME     | NOT NULL DEFAULT CURRENT_TIMESTAMP   |
| `updated_at`| DATETIME     | NOT NULL DEFAULT CURRENT_TIMESTAMP   |

- One row = one set.
- `weight` is nullable — bodyweight exercises (push-ups, bear crawls) don't need it.
- `rep_type` determines how `reps` is displayed: `reps` → "5", `each_side` → "5/ea", `seconds` → "30s", `distance` → "20yd".
- `rpe` is rate of perceived exertion (1–10 scale, half-steps allowed). Nullable — only logged when the athlete reports it.
- `set_number` preserves ordering within exercise within workout.
- `notes` holds per-set observations ("form broke down on rep 18").

### `body_weights`

| Column       | Type         | Constraints                          |
|-------------|-------------|--------------------------------------|
| `id`        | INTEGER      | PRIMARY KEY AUTOINCREMENT            |
| `athlete_id`| INTEGER      | NOT NULL, FK → athletes(id)          |
| `date`      | DATE         | NOT NULL                             |
| `weight`    | REAL         | NOT NULL                             |
| `notes`     | TEXT         | NULL                                 |
| `created_at`| DATETIME     | NOT NULL DEFAULT CURRENT_TIMESTAMP   |

- One weigh-in per athlete per day (`UNIQUE(athlete_id, date)`).
- `weight` stored in the athlete's preferred unit (lb or kg) — unit convention is per-deployment, not per-row.
- Deleting an athlete cascades to their body weight history.

### `goal_history`

| Column          | Type         | Constraints                          |
|----------------|-------------|--------------------------------------|
| `id`           | INTEGER      | PRIMARY KEY AUTOINCREMENT            |
| `athlete_id`   | INTEGER      | NOT NULL, FK → athletes(id) ON DELETE CASCADE |
| `goal`         | TEXT         | NOT NULL                             |
| `previous_goal`| TEXT         | NULL                                 |
| `set_by`       | INTEGER      | NULL, FK → users(id) ON DELETE SET NULL |
| `effective_date`| DATE        | NOT NULL DEFAULT (date('now'))       |
| `notes`        | TEXT         | NULL                                 |
| `created_at`   | DATETIME     | NOT NULL DEFAULT CURRENT_TIMESTAMP   |

- Append-only history of **athlete-level** goal changes (the long-term objective on `athletes.goal`).
- Per-cycle goals live on `athlete_programs.goal` and are inherently historized — each cycle is a separate row, so no additional history table is needed. See design note 13 above for the two-level goal distinction.
- `goal` is the new goal text. `previous_goal` is the prior goal (NULL if this is the first goal set).
- `set_by` records which user (coach/admin) made the change. SET NULL on user deletion preserves the history entry.
- `effective_date` defaults to today. Allows backdating if needed.
- `notes` holds optional context for the change ("Shifting focus after knee recovery").
- Current goal is still read from `athletes.goal` for quick access — this table provides the historical timeline.
- Deleting an athlete cascades to their goal history.

### `tier_history`

| Column          | Type         | Constraints                          |
|----------------|-------------|--------------------------------------|
| `id`           | INTEGER      | PRIMARY KEY AUTOINCREMENT            |
| `athlete_id`   | INTEGER      | NOT NULL, FK → athletes(id) ON DELETE CASCADE |
| `tier`         | TEXT         | NOT NULL, CHECK(tier IN ('foundational', 'intermediate', 'sport_performance')) |
| `previous_tier`| TEXT         | NULL, CHECK(previous_tier IN ('foundational', 'intermediate', 'sport_performance')) |
| `set_by`       | INTEGER      | NULL, FK → users(id) ON DELETE SET NULL |
| `effective_date`| DATE        | NOT NULL DEFAULT (date('now'))       |
| `notes`        | TEXT         | NULL                                 |
| `created_at`   | DATETIME     | NOT NULL DEFAULT CURRENT_TIMESTAMP   |

- Append-only history of tier transitions (e.g., foundational → intermediate).
- Same pattern as `goal_history` — the current tier is still read from `athletes.tier` for quick access.
- `previous_tier` is NULL when the tier is first set on a new athlete.
- Automatically recorded when a coach edits or promotes an athlete's tier.
- Deleting an athlete cascades to their tier history.

### `athlete_notes`

| Column       | Type         | Constraints                          |
|-------------|-------------|--------------------------------------|
| `id`        | INTEGER      | PRIMARY KEY AUTOINCREMENT            |
| `athlete_id`| INTEGER      | NOT NULL, FK → athletes(id) ON DELETE CASCADE |
| `author_id` | INTEGER      | NULL, FK → users(id) ON DELETE SET NULL |
| `date`      | DATE         | NOT NULL DEFAULT (date('now'))       |
| `content`   | TEXT         | NOT NULL                             |
| `is_private`| INTEGER      | NOT NULL DEFAULT 0, CHECK(is_private IN (0, 1)) |
| `pinned`    | INTEGER      | NOT NULL DEFAULT 0, CHECK(pinned IN (0, 1)) |
| `created_at`| DATETIME     | NOT NULL DEFAULT CURRENT_TIMESTAMP   |
| `updated_at`| DATETIME     | NOT NULL DEFAULT CURRENT_TIMESTAMP   |

- Free-form coach notes attached to an athlete, shown on the journal timeline.
- `is_private = 1` means only coaches/admins can see the note; `is_private = 0` means the athlete can see it too.
- `pinned` notes appear at the top of the journal regardless of date.
- `author_id` records who wrote the note. SET NULL on user deletion preserves the note.
- `date` defaults to today but can be set to any date (e.g., backdating a note from a conversation).
- Deleting an athlete cascades to their notes.

### `workout_reviews`

| Column       | Type         | Constraints                          |
|-------------|-------------|--------------------------------------|
| `id`        | INTEGER      | PRIMARY KEY AUTOINCREMENT            |
| `workout_id`| INTEGER      | NOT NULL UNIQUE, FK → workouts(id) ON DELETE CASCADE |
| `coach_id`  | INTEGER      | NULL, FK → users(id) ON DELETE SET NULL |
| `status`    | TEXT         | NOT NULL, CHECK(status IN ('approved', 'needs_work')) |
| `notes`     | TEXT         | NULL                                 |
| `created_at`| DATETIME     | NOT NULL DEFAULT CURRENT_TIMESTAMP   |
| `updated_at`| DATETIME     | NOT NULL DEFAULT CURRENT_TIMESTAMP   |

- One review per workout (`UNIQUE(workout_id)`) — coaches can update their review but there is only one.
- `status` is either `approved` (coach is satisfied) or `needs_work` (coach wants the athlete to address feedback).
- `notes` holds coach feedback ("Great form on the deadlifts! Try to go deeper on squats next time.").
- `coach_id` records which coach submitted the review.
- Deleting a workout cascades to its review. Deleting the reviewing coach sets `coach_id` to NULL, preserving the review.

### `program_templates`

| Column       | Type         | Constraints                          |
|-------------|-------------|--------------------------------------|
| `id`        | INTEGER      | PRIMARY KEY AUTOINCREMENT            |
| `name`      | TEXT         | NOT NULL UNIQUE COLLATE NOCASE        |
| `description`| TEXT        | NULL                                 |
| `num_weeks` | INTEGER      | NOT NULL                             |
| `num_days`  | INTEGER      | NOT NULL                             |
| `is_loop`   | INTEGER      | NOT NULL DEFAULT 0, CHECK(0 or 1)    |
| `created_at`| DATETIME     | NOT NULL DEFAULT CURRENT_TIMESTAMP   |
| `updated_at`| DATETIME     | NOT NULL DEFAULT CURRENT_TIMESTAMP   |

- Defines a reusable training program structure (e.g. "5/3/1 BBB", "GZCL T1/T2/T3").
- `num_weeks` and `num_days` define the cycle length — e.g. 4 weeks × 4 days for 5/3/1.
- `is_loop = 1` marks indefinite cycling programs (e.g. Yessis 1×20 foundational) that repeat until the coach advances the athlete. `is_loop = 0` (default) for standard programs that still cycle but show completion progress.
- Templates are shared across athletes; assignment is tracked via `athlete_programs`.

### `prescribed_sets`

| Column       | Type         | Constraints                          |
|-------------|-------------|--------------------------------------|
| `id`        | INTEGER      | PRIMARY KEY AUTOINCREMENT            |
| `template_id`| INTEGER     | NOT NULL, FK → program_templates(id) |
| `exercise_id`| INTEGER     | NOT NULL, FK → exercises(id)         |
| `week`      | INTEGER      | NOT NULL                             |
| `day`       | INTEGER      | NOT NULL                             |
| `set_number`| INTEGER      | NOT NULL                             |
| `reps`      | INTEGER      | NULL (NULL = AMRAP)                  |
| `rep_type`  | TEXT         | NOT NULL DEFAULT 'reps', CHECK(rep_type IN ('reps', 'each_side', 'seconds', 'distance')) |
| `percentage`| REAL         | NULL (% of training max)             |
| `absolute_weight`| REAL    | NULL (fixed weight in lbs/kg)        |
| `sort_order`| INTEGER      | NOT NULL DEFAULT 0                   |
| `notes`     | TEXT         | NULL                                 |

- Each row is one prescribed set within a template's week/day.
- `reps = NULL` indicates an AMRAP (as many reps as possible) set.
- `rep_type` determines how `reps` is displayed: `reps` → "5", `each_side` → "5/ea", `seconds` → "30s", `distance` → "20yd".
- `percentage` is a decimal (e.g. 65.0 for 65%) used to calculate target weight from the athlete's training max.
- `absolute_weight` is a fixed weight for programs that don't use percentage-of-TM (e.g. Yessis foundational, accessories). When both `percentage` and `absolute_weight` are set, percentage takes priority.
- `sort_order` controls exercise display order within a day. All sets for the same exercise share the same sort_order. Lower values appear first. Critical for methodologies where exercise sequence matters.
- `UNIQUE(template_id, week, day, exercise_id, set_number)` prevents duplicate sets.

### `athlete_programs`

| Column       | Type         | Constraints                          |
|-------------|-------------|--------------------------------------|
| `id`        | INTEGER      | PRIMARY KEY AUTOINCREMENT            |
| `athlete_id`| INTEGER      | NOT NULL, FK → athletes(id)          |
| `template_id`| INTEGER     | NOT NULL, FK → program_templates(id) |
| `start_date`| DATE         | NOT NULL                             |
| `active`    | INTEGER      | NOT NULL DEFAULT 1, CHECK(0 or 1)    |
| `notes`     | TEXT         | NULL                                 |
| `goal`      | TEXT         | NULL                                 |
| `created_at`| DATETIME     | NOT NULL DEFAULT CURRENT_TIMESTAMP   |
| `updated_at`| DATETIME     | NOT NULL DEFAULT CURRENT_TIMESTAMP   |

- Links an athlete to a program template.
- Partial unique index enforces one active program per athlete.
- Deactivation sets `active = 0`; reassignment creates a new row.
- `start_date` is the reference point for calculating program position — position advances by counting completed workouts since start.
- `goal` holds a cycle-specific training goal ("increase squat TM by 10 lbs"). Nullable.
- Program cycles repeat automatically when all weeks × days are exhausted.

### `progression_rules`

| Column        | Type         | Constraints                          |
|--------------|-------------|--------------------------------------|
| `id`         | INTEGER      | PRIMARY KEY AUTOINCREMENT            |
| `template_id`| INTEGER      | NOT NULL, FK → program_templates(id) ON DELETE CASCADE |
| `exercise_id`| INTEGER      | NOT NULL, FK → exercises(id) ON DELETE CASCADE |
| `increment`  | REAL         | NOT NULL                             |

- Per-exercise training max increment rule within a program template.
- `increment` is the suggested TM bump amount (e.g. 5.0 or 10.0 lbs) after a successful cycle.
- `UNIQUE(template_id, exercise_id)` — one rule per exercise per template.
- Cascades on delete from both template and exercise sides.
- Used by the cycle review screen to suggest TM updates — the coach still decides whether to apply, edit, or skip.

### `login_tokens`

| Column       | Type         | Constraints                          |
|-------------|-------------|--------------------------------------|
| `id`        | INTEGER      | PRIMARY KEY AUTOINCREMENT            |
| `user_id`   | INTEGER      | NOT NULL, FK → users(id) ON DELETE CASCADE |
| `token`     | TEXT         | NOT NULL UNIQUE                      |
| `label`     | TEXT         | NULL                                 |
| `expires_at`| DATETIME     | NULL                                 |
| `created_at`| DATETIME     | NOT NULL DEFAULT CURRENT_TIMESTAMP   |

- Passwordless login tokens — generated by coaches/admins and given to athletes for first-time device enrollment.
- `token` is a unique random string used as the login credential.
- `label` is an optional human-readable name for the token (e.g. "Caydan's iPad").
- `expires_at` is optional — NULL means the token never expires.
- Deleting a user cascades to their login tokens.

### `webauthn_credentials`

| Column                | Type         | Constraints                          |
|----------------------|-------------|--------------------------------------|
| `id`                 | INTEGER      | PRIMARY KEY AUTOINCREMENT            |
| `user_id`            | INTEGER      | NOT NULL, FK → users(id) ON DELETE CASCADE |
| `credential_id`      | BLOB         | NOT NULL UNIQUE                      |
| `public_key`         | BLOB         | NOT NULL                             |
| `attestation_type`   | TEXT         | NOT NULL DEFAULT ''                  |
| `transport`          | TEXT         | NULL                                 |
| `sign_count`         | INTEGER      | NOT NULL DEFAULT 0                   |
| `clone_warning`      | INTEGER      | NOT NULL DEFAULT 0, CHECK(0 or 1)    |
| `attachment`         | TEXT         | NOT NULL DEFAULT ''                  |
| `aaguid`             | BLOB         | NULL                                 |
| `flags_user_present` | INTEGER      | NOT NULL DEFAULT 0, CHECK(0 or 1)    |
| `flags_user_verified`| INTEGER      | NOT NULL DEFAULT 0, CHECK(0 or 1)    |
| `flags_backup_eligible`| INTEGER    | NOT NULL DEFAULT 0, CHECK(0 or 1)    |
| `flags_backup_state` | INTEGER      | NOT NULL DEFAULT 0, CHECK(0 or 1)    |
| `label`              | TEXT         | NULL                                 |
| `created_at`         | DATETIME     | NOT NULL DEFAULT CURRENT_TIMESTAMP   |

- WebAuthn/passkey credentials for passwordless authentication.
- Each user can register multiple passkeys (one per device).
- `credential_id` and `public_key` are the core WebAuthn credential data.
- `sign_count` tracks authentication counter for clone detection.
- `flags_*` columns store the WebAuthn authenticator flags.
- `label` is an optional human-readable name for the passkey (e.g. "iPhone", "YubiKey").
- Deleting a user cascades to their credentials.

### `sessions`

| Column  | Type  | Constraints     |
|---------|-------|-----------------|
| `token` | TEXT  | PRIMARY KEY     |
| `data`  | BLOB  | NOT NULL        |
| `expiry`| REAL  | NOT NULL        |

- Session store for `alexedwards/scs` session manager.
- Managed entirely by the scs library — not accessed directly by application code.
- `token` is the session ID sent to the client as a cookie.
- `expiry` is a Unix timestamp used by scs for automatic cleanup.

### `app_settings`

| Column  | Type | Constraints          |
|---------|------|----------------------|
| `key`   | TEXT | PRIMARY KEY NOT NULL |
| `value` | TEXT | NOT NULL             |

- Key-value store for runtime configuration (LLM provider, model, API key, etc.).
- Resolution chain: environment variable → `app_settings` row → built-in default.
- Sensitive values (API keys) are stored encrypted with AES-256-GCM, prefixed with `enc:`.
- Managed via the admin Settings page.

## SQLite DDL

```sql
PRAGMA journal_mode = WAL;
PRAGMA busy_timeout = 5000;
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS athletes (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT    NOT NULL COLLATE NOCASE,
    tier        TEXT    CHECK(tier IN ('foundational', 'intermediate', 'sport_performance')),
    notes       TEXT,
    goal        TEXT,
    coach_id    INTEGER REFERENCES users(id) ON DELETE SET NULL,
    track_body_weight INTEGER NOT NULL DEFAULT 1 CHECK(track_body_weight IN (0, 1)),
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS users (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    username        TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    name            TEXT,
    email           TEXT    UNIQUE COLLATE NOCASE,
    password_hash   TEXT,
    athlete_id      INTEGER REFERENCES athletes(id) ON DELETE SET NULL,
    is_coach        INTEGER NOT NULL DEFAULT 0 CHECK(is_coach IN (0, 1)),
    is_admin        INTEGER NOT NULL DEFAULT 0 CHECK(is_admin IN (0, 1)),
    avatar_path     TEXT,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS user_preferences (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     INTEGER NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    weight_unit TEXT    NOT NULL DEFAULT 'lbs' CHECK(weight_unit IN ('lbs', 'kg')),
    timezone    TEXT    NOT NULL DEFAULT 'America/New_York',
    date_format TEXT    NOT NULL DEFAULT 'Jan 2, 2006',
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS exercises (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    name         TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    tier         TEXT    CHECK(tier IN ('foundational', 'intermediate', 'sport_performance')),
    form_notes   TEXT,
    demo_url     TEXT,
    rest_seconds INTEGER,
    featured     INTEGER NOT NULL DEFAULT 0 CHECK(featured IN (0, 1)),
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS athlete_exercises (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    athlete_id      INTEGER NOT NULL REFERENCES athletes(id) ON DELETE CASCADE,
    exercise_id     INTEGER NOT NULL REFERENCES exercises(id) ON DELETE CASCADE,
    target_reps     INTEGER,
    active          INTEGER NOT NULL DEFAULT 1 CHECK(active IN (0, 1)),
    assigned_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deactivated_at  DATETIME
);

CREATE TABLE IF NOT EXISTS training_maxes (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    athlete_id      INTEGER NOT NULL REFERENCES athletes(id) ON DELETE CASCADE,
    exercise_id     INTEGER NOT NULL REFERENCES exercises(id) ON DELETE CASCADE,
    weight          REAL    NOT NULL,
    effective_date  DATE    NOT NULL,
    notes           TEXT,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(athlete_id, exercise_id, effective_date)
);

CREATE TABLE IF NOT EXISTS workouts (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    athlete_id  INTEGER NOT NULL REFERENCES athletes(id) ON DELETE CASCADE,
    date        DATE    NOT NULL,
    notes       TEXT,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(athlete_id, date)
);

CREATE TABLE IF NOT EXISTS workout_sets (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    workout_id  INTEGER NOT NULL REFERENCES workouts(id) ON DELETE CASCADE,
    exercise_id INTEGER NOT NULL REFERENCES exercises(id) ON DELETE RESTRICT,
    set_number  INTEGER NOT NULL,
    reps        INTEGER NOT NULL,
    rep_type    TEXT    NOT NULL DEFAULT 'reps' CHECK(rep_type IN ('reps', 'each_side', 'seconds', 'distance')),
    weight      REAL,
    rpe         REAL    CHECK(rpe >= 1 AND rpe <= 10),
    notes       TEXT,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(workout_id, exercise_id, set_number)
);

-- Indexes for common query patterns
CREATE UNIQUE INDEX IF NOT EXISTS idx_athlete_exercises_unique_active
    ON athlete_exercises(athlete_id, exercise_id) WHERE active = 1;

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_unique_athlete_id
    ON users(athlete_id) WHERE athlete_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_user_preferences_user_id
    ON user_preferences(user_id);

CREATE INDEX IF NOT EXISTS idx_athletes_coach_id
    ON athletes(coach_id);

CREATE INDEX IF NOT EXISTS idx_athlete_exercises_athlete_id
    ON athlete_exercises(athlete_id);

CREATE INDEX IF NOT EXISTS idx_workout_sets_workout
    ON workout_sets(workout_id);

CREATE TABLE IF NOT EXISTS body_weights (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    athlete_id  INTEGER NOT NULL REFERENCES athletes(id) ON DELETE CASCADE,
    date        DATE    NOT NULL,
    weight      REAL    NOT NULL,
    notes       TEXT,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(athlete_id, date)
);

CREATE INDEX IF NOT EXISTS idx_body_weights_athlete_date
    ON body_weights(athlete_id, date DESC);

CREATE TABLE IF NOT EXISTS goal_history (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    athlete_id      INTEGER NOT NULL REFERENCES athletes(id) ON DELETE CASCADE,
    goal            TEXT    NOT NULL,
    previous_goal   TEXT,
    set_by          INTEGER REFERENCES users(id) ON DELETE SET NULL,
    effective_date  DATE    NOT NULL DEFAULT (date('now')),
    notes           TEXT,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_goal_history_athlete_date
    ON goal_history(athlete_id, effective_date DESC, created_at DESC);

-- Triggers for updated_at timestamps
-- WHEN guard prevents infinite recursion (trigger fires UPDATE, which would fire trigger again)
CREATE TRIGGER IF NOT EXISTS trigger_users_updated_at
AFTER UPDATE ON users FOR EACH ROW
WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE users SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

CREATE TRIGGER IF NOT EXISTS trigger_user_preferences_updated_at
AFTER UPDATE ON user_preferences FOR EACH ROW
WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE user_preferences SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

CREATE TRIGGER IF NOT EXISTS trigger_athletes_updated_at
AFTER UPDATE ON athletes FOR EACH ROW
WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE athletes SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

CREATE TRIGGER IF NOT EXISTS trigger_exercises_updated_at
AFTER UPDATE ON exercises FOR EACH ROW
WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE exercises SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

CREATE TRIGGER IF NOT EXISTS trigger_workouts_updated_at
AFTER UPDATE ON workouts FOR EACH ROW
WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE workouts SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

CREATE TRIGGER IF NOT EXISTS trigger_workout_sets_updated_at
AFTER UPDATE ON workout_sets FOR EACH ROW
WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE workout_sets SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

CREATE TABLE IF NOT EXISTS workout_reviews (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    workout_id  INTEGER NOT NULL UNIQUE REFERENCES workouts(id) ON DELETE CASCADE,
    coach_id    INTEGER REFERENCES users(id) ON DELETE SET NULL,
    status      TEXT    NOT NULL CHECK(status IN ('approved', 'needs_work')),
    notes       TEXT,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_workout_reviews_workout_id
    ON workout_reviews(workout_id);

CREATE INDEX IF NOT EXISTS idx_workout_reviews_status
    ON workout_reviews(status);

CREATE TRIGGER IF NOT EXISTS trigger_workout_reviews_updated_at
AFTER UPDATE ON workout_reviews FOR EACH ROW
WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE workout_reviews SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

CREATE TABLE IF NOT EXISTS program_templates (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    description TEXT,
    num_weeks   INTEGER NOT NULL DEFAULT 1,
    num_days    INTEGER NOT NULL DEFAULT 1,
    is_loop     INTEGER NOT NULL DEFAULT 0 CHECK(is_loop IN (0, 1)),
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS prescribed_sets (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    template_id     INTEGER NOT NULL REFERENCES program_templates(id) ON DELETE CASCADE,
    exercise_id     INTEGER NOT NULL REFERENCES exercises(id) ON DELETE RESTRICT,
    week            INTEGER NOT NULL,
    day             INTEGER NOT NULL,
    set_number      INTEGER NOT NULL,
    reps            INTEGER,
    rep_type        TEXT    NOT NULL DEFAULT 'reps' CHECK(rep_type IN ('reps', 'each_side', 'seconds', 'distance')),
    percentage      REAL,
    absolute_weight REAL,
    sort_order      INTEGER NOT NULL DEFAULT 0,
    notes           TEXT,
    UNIQUE(template_id, week, day, exercise_id, set_number)
);

CREATE INDEX IF NOT EXISTS idx_prescribed_sets_template
    ON prescribed_sets(template_id, week, day);

CREATE TABLE IF NOT EXISTS athlete_programs (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    athlete_id   INTEGER NOT NULL REFERENCES athletes(id) ON DELETE CASCADE,
    template_id  INTEGER NOT NULL REFERENCES program_templates(id) ON DELETE RESTRICT,
    start_date   DATE    NOT NULL,
    active       INTEGER NOT NULL DEFAULT 1 CHECK(active IN (0, 1)),
    notes        TEXT,
    goal         TEXT,
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_athlete_programs_active
    ON athlete_programs(athlete_id) WHERE active = 1;

CREATE TABLE IF NOT EXISTS progression_rules (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    template_id  INTEGER NOT NULL REFERENCES program_templates(id) ON DELETE CASCADE,
    exercise_id  INTEGER NOT NULL REFERENCES exercises(id) ON DELETE CASCADE,
    increment    REAL    NOT NULL,
    UNIQUE(template_id, exercise_id)
);

CREATE INDEX IF NOT EXISTS idx_progression_rules_template
    ON progression_rules(template_id);

CREATE TRIGGER IF NOT EXISTS trigger_program_templates_updated_at
AFTER UPDATE ON program_templates FOR EACH ROW
WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE program_templates SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

CREATE TRIGGER IF NOT EXISTS trigger_athlete_programs_updated_at
AFTER UPDATE ON athlete_programs FOR EACH ROW
WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE athlete_programs SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

CREATE TABLE IF NOT EXISTS login_tokens (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token       TEXT    NOT NULL UNIQUE,
    label       TEXT,
    expires_at  DATETIME,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_login_tokens_token ON login_tokens(token);
CREATE INDEX IF NOT EXISTS idx_login_tokens_user_id ON login_tokens(user_id);

CREATE TABLE IF NOT EXISTS webauthn_credentials (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id         INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    credential_id   BLOB    NOT NULL UNIQUE,
    public_key      BLOB    NOT NULL,
    attestation_type TEXT   NOT NULL DEFAULT '',
    transport       TEXT,
    sign_count      INTEGER NOT NULL DEFAULT 0,
    clone_warning   INTEGER NOT NULL DEFAULT 0 CHECK(clone_warning IN (0, 1)),
    attachment      TEXT    NOT NULL DEFAULT '',
    aaguid          BLOB,
    flags_user_present    INTEGER NOT NULL DEFAULT 0 CHECK(flags_user_present IN (0, 1)),
    flags_user_verified   INTEGER NOT NULL DEFAULT 0 CHECK(flags_user_verified IN (0, 1)),
    flags_backup_eligible INTEGER NOT NULL DEFAULT 0 CHECK(flags_backup_eligible IN (0, 1)),
    flags_backup_state    INTEGER NOT NULL DEFAULT 0 CHECK(flags_backup_state IN (0, 1)),
    label           TEXT,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_webauthn_credentials_user_id ON webauthn_credentials(user_id);

CREATE TABLE IF NOT EXISTS equipment (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    description TEXT,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TRIGGER IF NOT EXISTS trigger_equipment_updated_at
AFTER UPDATE ON equipment FOR EACH ROW
WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE equipment SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

CREATE TABLE IF NOT EXISTS exercise_equipment (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    exercise_id  INTEGER NOT NULL REFERENCES exercises(id) ON DELETE CASCADE,
    equipment_id INTEGER NOT NULL REFERENCES equipment(id) ON DELETE CASCADE,
    optional     INTEGER NOT NULL DEFAULT 0 CHECK(optional IN (0, 1)),
    UNIQUE(exercise_id, equipment_id)
);

CREATE INDEX IF NOT EXISTS idx_exercise_equipment_exercise
    ON exercise_equipment(exercise_id);

CREATE INDEX IF NOT EXISTS idx_exercise_equipment_equipment
    ON exercise_equipment(equipment_id);

CREATE TABLE IF NOT EXISTS athlete_equipment (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    athlete_id   INTEGER NOT NULL REFERENCES athletes(id) ON DELETE CASCADE,
    equipment_id INTEGER NOT NULL REFERENCES equipment(id) ON DELETE CASCADE,
    UNIQUE(athlete_id, equipment_id)
);

CREATE INDEX IF NOT EXISTS idx_athlete_equipment_athlete
    ON athlete_equipment(athlete_id);

CREATE INDEX IF NOT EXISTS idx_athlete_equipment_equipment
    ON athlete_equipment(equipment_id);

-- Session store for alexedwards/scs
CREATE TABLE IF NOT EXISTS sessions (
    token  TEXT PRIMARY KEY,
    data   BLOB NOT NULL,
    expiry REAL NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sessions_expiry ON sessions(expiry);

-- Application settings — key-value store for runtime configuration.
CREATE TABLE IF NOT EXISTS app_settings (
    key   TEXT PRIMARY KEY NOT NULL,
    value TEXT NOT NULL
);
```

## Seed Data (Development)

```sql
-- Tier exercises
INSERT INTO exercises (name, tier, form_notes) VALUES
    ('Lunges', 'foundational', 'Keep front knee over ankle, torso upright'),
    ('Push-ups', 'foundational', 'Full range of motion, elbows at 45 degrees'),
    ('Goblet Squats', 'foundational', 'Hold weight at chest, sit back into heels'),
    ('Bear Crawls', 'foundational', 'Keep hips low, opposite hand-foot movement'),
    ('Bench Press', 'intermediate', 'Training bar. Feet flat, arch back slightly, control the descent'),
    ('Dumbbell Snatch', 'intermediate', 'Start from hang position, explosive hip drive'),
    ('Cleans', 'sport_performance', 'Full clean from floor, catch in front rack'),
    ('Deadlifts', 'sport_performance', 'Traditional stance, neutral spine throughout');

-- General lifts (no tier)
INSERT INTO exercises (name, tier, form_notes) VALUES
    ('Back Squat', NULL, 'Break parallel, drive knees out'),
    ('Overhead Press', NULL, 'Strict press, no leg drive');
```

## Operational Notes

- **Connection pooling:** Go's `database/sql` must be configured with `db.SetMaxOpenConns(1)` for SQLite's single-writer model.
- **Busy timeout:** `PRAGMA busy_timeout = 5000` is set in the DDL — concurrent reads will wait up to 5s during writes instead of failing immediately.
- **Backups:** Do NOT use `cp` on a live WAL-mode database. Use `sqlite3 replog.db ".backup backup.db"` or the SQLite backup API, which correctly handles the WAL file.
- **WAL mode:** Set once; persists across connections. Provides concurrent reads with single-writer without blocking.

### `equipment`

| Column        | Type         | Constraints                          |
|--------------|-------------|--------------------------------------|
| `id`         | INTEGER      | PRIMARY KEY AUTOINCREMENT            |
| `name`       | TEXT         | NOT NULL UNIQUE COLLATE NOCASE        |
| `description`| TEXT         | NULL                                 |
| `created_at` | DATETIME     | NOT NULL DEFAULT CURRENT_TIMESTAMP   |
| `updated_at` | DATETIME     | NOT NULL DEFAULT CURRENT_TIMESTAMP   |

- Shared catalog of equipment types (e.g. "Barbell", "Squat Rack", "Dumbbells", "Pull-up Bar").
- Managed by coaches — athletes select from the catalog.
- `COLLATE NOCASE` prevents "Barbell" and "barbell" duplicates.

### `exercise_equipment`

| Column        | Type         | Constraints                          |
|--------------|-------------|--------------------------------------|
| `id`         | INTEGER      | PRIMARY KEY AUTOINCREMENT            |
| `exercise_id`| INTEGER      | NOT NULL, FK → exercises(id) ON DELETE CASCADE |
| `equipment_id`| INTEGER     | NOT NULL, FK → equipment(id) ON DELETE CASCADE |
| `optional`   | INTEGER      | NOT NULL DEFAULT 0, CHECK(optional IN (0, 1)) |

- Many-to-many: which equipment is needed for an exercise.
- `optional = 0` means required; `optional = 1` means nice-to-have.
- `UNIQUE(exercise_id, equipment_id)` prevents duplicate links.
- Deleting an exercise or equipment item cascades to remove the link.

### `athlete_equipment`

| Column        | Type         | Constraints                          |
|--------------|-------------|--------------------------------------|
| `id`         | INTEGER      | PRIMARY KEY AUTOINCREMENT            |
| `athlete_id` | INTEGER      | NOT NULL, FK → athletes(id) ON DELETE CASCADE |
| `equipment_id`| INTEGER     | NOT NULL, FK → equipment(id) ON DELETE CASCADE |

- Many-to-many: which equipment an athlete has available.
- `UNIQUE(athlete_id, equipment_id)` prevents duplicate entries.
- Deleting an athlete or equipment item cascades to remove the link.

## Future Considerations (v2+)

- **Exercise categories/tags**: Muscle group, movement pattern (push/pull/hinge/squat/carry).
- **Program template sharing/import**: JSON export/import of templates between deployments.
