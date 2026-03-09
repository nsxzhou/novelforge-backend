ALTER TABLE conversations
    ADD COLUMN IF NOT EXISTS pending_suggestion JSONB NOT NULL DEFAULT 'null'::jsonb;
