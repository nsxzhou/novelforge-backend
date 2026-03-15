CREATE TABLE IF NOT EXISTS project_prompt_overrides (
    project_id  UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    capability  TEXT NOT NULL,
    system_tmpl TEXT NOT NULL,
    user_tmpl   TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (project_id, capability)
);
