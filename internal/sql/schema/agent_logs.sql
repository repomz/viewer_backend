CREATE TABLE agent_logs (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id     INTEGER NOT NULL CHECK (agent_id > 0),
    period_start TIMESTAMPTZ NOT NULL,
    period_end   TIMESTAMPTZ NOT NULL,
    content      TEXT NOT NULL,
    received_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT agent_logs_period_valid CHECK (period_end > period_start),
    CONSTRAINT agent_logs_agent_period_unique UNIQUE (agent_id, period_start)
);

CREATE INDEX idx_agent_logs_agent_period
    ON agent_logs (agent_id, period_start DESC);
