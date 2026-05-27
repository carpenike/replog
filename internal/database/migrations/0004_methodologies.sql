-- +goose Up

-- First-class training methodology (ADR 016, Phase 1).
--
-- A Methodology is a stored, coach-selectable program-design philosophy +
-- prescription block (e.g. Yessis 1×20, 5/3/1, Sarge circuit) that will
-- drive Phase-2 generation. Phase 1 is data-only — nothing in
-- buildSystemPrompt / BuildAthleteContext / buildExerciseCatalog reads
-- these tables yet; the seeded definitions exist alongside the existing
-- hardcoded prompt blocks. Additive only (ADR 002 pre-prod policy).
--
-- IMPORTANT (per ADR 016 Decision #4): `methodologies.definition` carries
-- ONLY the methodology-specific per-tier block. The shared youth-rules
-- preamble and the youth-safety floors (NEVER 1RM testing for youth,
-- NEVER percentage loading unless TM-set AND sport_performance, etc.)
-- STAY IN CODE, emitted for every youth athlete regardless of which
-- methodology is selected.

CREATE TABLE IF NOT EXISTS methodologies (
    id               INTEGER  PRIMARY KEY AUTOINCREMENT,
    key              TEXT     NOT NULL UNIQUE COLLATE NOCASE,
    name             TEXT     NOT NULL,
    audience         TEXT     CHECK(audience IN ('youth', 'adult') OR audience IS NULL),
    applicable_tiers TEXT,
    philosophy       TEXT,
    definition       TEXT     NOT NULL,
    created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_methodologies_audience
    ON methodologies(audience);

-- Exemplar reference programs for a methodology — the program_templates
-- the LLM should treat as primary structural examples when generating
-- against this methodology.
CREATE TABLE IF NOT EXISTS methodology_reference_programs (
    methodology_id INTEGER NOT NULL REFERENCES methodologies(id)    ON DELETE CASCADE,
    template_id    INTEGER NOT NULL REFERENCES program_templates(id) ON DELETE CASCADE,
    PRIMARY KEY (methodology_id, template_id)
);

CREATE INDEX IF NOT EXISTS idx_methodology_reference_programs_template
    ON methodology_reference_programs(template_id);

-- Allow-list: equipment this methodology is allowed to draw from. The
-- generation catalog is filtered to this allow-list (intersected with the
-- athlete's available equipment) before the LLM sees it — Phase 2.
CREATE TABLE IF NOT EXISTS methodology_allowed_equipment (
    methodology_id INTEGER NOT NULL REFERENCES methodologies(id) ON DELETE CASCADE,
    equipment_id   INTEGER NOT NULL REFERENCES equipment(id)     ON DELETE CASCADE,
    PRIMARY KEY (methodology_id, equipment_id)
);

CREATE INDEX IF NOT EXISTS idx_methodology_allowed_equipment_equipment
    ON methodology_allowed_equipment(equipment_id);

-- Movement-pattern tags on exercises (Dan John taxonomy). Same tag set
-- powers both the methodology pattern allow-list AND the joint-action /
-- movement-coverage checks Yessis already requires.
CREATE TABLE IF NOT EXISTS exercise_movement_patterns (
    exercise_id INTEGER NOT NULL REFERENCES exercises(id) ON DELETE CASCADE,
    pattern     TEXT    NOT NULL CHECK(pattern IN ('push', 'pull', 'hinge', 'squat', 'carry', 'ground')),
    PRIMARY KEY (exercise_id, pattern)
);

CREATE INDEX IF NOT EXISTS idx_exercise_movement_patterns_pattern
    ON exercise_movement_patterns(pattern);

-- A methodology's allowed exercise scope, by pattern (broad rule).
-- e.g. Yessis 1×20 = push/pull/hinge/squat/ground (no carry).
CREATE TABLE IF NOT EXISTS methodology_allowed_patterns (
    methodology_id INTEGER NOT NULL REFERENCES methodologies(id) ON DELETE CASCADE,
    pattern        TEXT    NOT NULL CHECK(pattern IN ('push', 'pull', 'hinge', 'squat', 'carry', 'ground')),
    PRIMARY KEY (methodology_id, pattern)
);

-- An explicit exercise allow-list override on top of the pattern scope
-- (e.g. 5/3/1 barbell mains, the Sarge bespoke list). Both surfaces ship
-- in Phase 1; allow-by-pattern + override-by-list semantics are settled
-- at Phase-2 prompt-composition time.
CREATE TABLE IF NOT EXISTS methodology_allowed_exercises (
    methodology_id INTEGER NOT NULL REFERENCES methodologies(id) ON DELETE CASCADE,
    exercise_id    INTEGER NOT NULL REFERENCES exercises(id)     ON DELETE CASCADE,
    PRIMARY KEY (methodology_id, exercise_id)
);

CREATE INDEX IF NOT EXISTS idx_methodology_allowed_exercises_exercise
    ON methodology_allowed_exercises(exercise_id);

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS trigger_methodologies_updated_at
AFTER UPDATE ON methodologies FOR EACH ROW
WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE methodologies SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;
-- +goose StatementEnd

-- +goose Down

DROP TRIGGER IF EXISTS trigger_methodologies_updated_at;
DROP TABLE IF EXISTS methodology_allowed_exercises;
DROP TABLE IF EXISTS methodology_allowed_patterns;
DROP TABLE IF EXISTS exercise_movement_patterns;
DROP TABLE IF EXISTS methodology_allowed_equipment;
DROP TABLE IF EXISTS methodology_reference_programs;
DROP TABLE IF EXISTS methodologies;
