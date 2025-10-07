-- 001_initial_schema.sql
-- Initial trigger table schema

CREATE TABLE triggers
(
    internal_id BIGSERIAL PRIMARY KEY,
    id          UUID  NOT NULL UNIQUE,
    name        TEXT  NOT NULL,
    type        TEXT  NOT NULL,
    workflow_id UUID  NOT NULL,
    data        JSONB NOT NULL,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_triggers_id ON triggers (id);
CREATE INDEX idx_triggers_workflow_id ON triggers (workflow_id);
