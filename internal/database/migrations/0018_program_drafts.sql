-- +goose Up

-- Program drafts (phase 10). A draft is a proposal a coach reviews.
-- Existing rows stay approved, so seed programs and current assignments
-- keep working. A service token may insert a draft. Approving it, and
-- creating an already-approved template, stay browser-session actions.
--
-- lineage_id NULL is the first version of a program. A later version
-- points lineage_id at that root and increments version. The column is
-- not a self-reference: a row does not store its own id.

ALTER TABLE program_templates ADD COLUMN status TEXT NOT NULL DEFAULT 'approved'
    CHECK (status IN ('draft', 'approved'));

ALTER TABLE program_templates ADD COLUMN lineage_id INTEGER
    REFERENCES program_templates(id) ON DELETE SET NULL;

ALTER TABLE program_templates ADD COLUMN version INTEGER NOT NULL DEFAULT 1
    CHECK (version >= 1);

-- Names stay unique among approved programs. A draft may reuse a name;
-- it is not the catalog entry until a coach approves it.
DROP INDEX IF EXISTS idx_program_templates_name_global;
DROP INDEX IF EXISTS idx_program_templates_name_athlete;

CREATE UNIQUE INDEX idx_program_templates_name_global
    ON program_templates(name) WHERE athlete_id IS NULL AND status = 'approved';

CREATE UNIQUE INDEX idx_program_templates_name_athlete
    ON program_templates(athlete_id, name) WHERE athlete_id IS NOT NULL AND status = 'approved';

CREATE UNIQUE INDEX idx_program_templates_lineage_version
    ON program_templates(COALESCE(lineage_id, id), version);

CREATE INDEX idx_program_templates_status ON program_templates(status);

-- Provenance for a proposal: who wrote it, the date, and the source text.
CREATE TABLE program_sources (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    template_id INTEGER NOT NULL REFERENCES program_templates(id) ON DELETE CASCADE,
    coach_id    INTEGER REFERENCES users(id) ON DELETE SET NULL,
    source_date DATE    NOT NULL,
    source_text TEXT    NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_program_sources_template ON program_sources(template_id);

ALTER TABLE prescribed_sets ADD COLUMN prescribed_rpe REAL
    CHECK (prescribed_rpe IS NULL OR (prescribed_rpe >= 1 AND prescribed_rpe <= 10));

ALTER TABLE prescribed_sets ADD COLUMN group_label TEXT;

-- +goose Down

DROP INDEX IF EXISTS idx_program_sources_template;
DROP TABLE IF EXISTS program_sources;

ALTER TABLE prescribed_sets DROP COLUMN group_label;
ALTER TABLE prescribed_sets DROP COLUMN prescribed_rpe;

DROP INDEX IF EXISTS idx_program_templates_status;
DROP INDEX IF EXISTS idx_program_templates_lineage_version;
DROP INDEX IF EXISTS idx_program_templates_name_global;
DROP INDEX IF EXISTS idx_program_templates_name_athlete;

ALTER TABLE program_templates DROP COLUMN version;
ALTER TABLE program_templates DROP COLUMN lineage_id;
ALTER TABLE program_templates DROP COLUMN status;

CREATE UNIQUE INDEX idx_program_templates_name_global
    ON program_templates(name) WHERE athlete_id IS NULL;

CREATE UNIQUE INDEX idx_program_templates_name_athlete
    ON program_templates(athlete_id, name) WHERE athlete_id IS NOT NULL;
