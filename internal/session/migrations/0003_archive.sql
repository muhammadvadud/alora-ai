CREATE TABLE IF NOT EXISTS agent.archived_messages (
    id BIGSERIAL PRIMARY KEY,
    session_id TEXT NOT NULL,
    user_id TEXT NOT NULL DEFAULT '',
    tenant_id TEXT NOT NULL DEFAULT '',
    messages JSONB NOT NULL,
    state JSONB NOT NULL,
    archived_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    reason TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS archived_messages_time_idx ON agent.archived_messages (archived_at);
