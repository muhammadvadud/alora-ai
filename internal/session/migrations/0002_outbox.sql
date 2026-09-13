CREATE TABLE IF NOT EXISTS agent.outbox (
    id BIGSERIAL PRIMARY KEY,
    kind TEXT NOT NULL,
    payload JSONB NOT NULL,
    attempts INT NOT NULL DEFAULT 0,
    next_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    dead BOOLEAN NOT NULL DEFAULT false
);

CREATE INDEX IF NOT EXISTS outbox_pending_idx ON agent.outbox (dead, next_at);
