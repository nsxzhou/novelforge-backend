CREATE TABLE IF NOT EXISTS projects (
    id UUID PRIMARY KEY,
    title TEXT NOT NULL,
    summary TEXT NOT NULL,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_projects_status_created_at ON projects (status, created_at, id);

CREATE TABLE IF NOT EXISTS assets (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    type TEXT NOT NULL,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_assets_project_id_created_at ON assets (project_id, created_at, id);
CREATE INDEX IF NOT EXISTS idx_assets_project_id_type_created_at ON assets (project_id, type, created_at, id);

CREATE TABLE IF NOT EXISTS chapters (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    ordinal INTEGER NOT NULL,
    status TEXT NOT NULL,
    content TEXT NOT NULL,
    current_draft_id UUID NULL,
    current_draft_confirmed_at TIMESTAMPTZ NULL,
    current_draft_confirmed_by UUID NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_chapters_project_id_ordinal ON chapters (project_id, ordinal, created_at, id);

CREATE TABLE IF NOT EXISTS conversations (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    target_type TEXT NOT NULL,
    target_id UUID NOT NULL,
    messages JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_conversations_project_id_created_at ON conversations (project_id, created_at, id);
CREATE INDEX IF NOT EXISTS idx_conversations_target_lookup ON conversations (project_id, target_type, target_id, created_at, id);

CREATE TABLE IF NOT EXISTS generation_records (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    chapter_id UUID NULL,
    conversation_id UUID NULL,
    kind TEXT NOT NULL,
    status TEXT NOT NULL,
    input_snapshot_ref TEXT NOT NULL,
    output_ref TEXT NOT NULL,
    token_usage INTEGER NOT NULL,
    duration_millis BIGINT NOT NULL,
    error_message TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_generation_records_project_id_created_at ON generation_records (project_id, created_at, id);
CREATE INDEX IF NOT EXISTS idx_generation_records_project_id_status_created_at ON generation_records (project_id, status, created_at, id);
CREATE INDEX IF NOT EXISTS idx_generation_records_chapter_id_created_at ON generation_records (chapter_id, created_at, id);
CREATE INDEX IF NOT EXISTS idx_generation_records_chapter_id_status_created_at ON generation_records (chapter_id, status, created_at, id);

CREATE TABLE IF NOT EXISTS metric_events (
    id UUID PRIMARY KEY,
    event_name TEXT NOT NULL,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    chapter_id UUID NULL,
    labels JSONB NOT NULL DEFAULT '{}'::jsonb,
    stats JSONB NOT NULL DEFAULT '{}'::jsonb,
    occurred_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_metric_events_project_id_occurred_at ON metric_events (project_id, occurred_at, id);
CREATE INDEX IF NOT EXISTS idx_metric_events_project_id_event_name_occurred_at ON metric_events (project_id, event_name, occurred_at, id);
