CREATE TABLE IF NOT EXISTS voice_sessions (
    id text PRIMARY KEY,
    payload jsonb NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS voice_sessions_updated_at_idx ON voice_sessions(updated_at DESC);
