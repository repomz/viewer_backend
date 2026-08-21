CREATE TABLE agent_records (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id        INTEGER NOT NULL,
    status          TEXT NOT NULL,
    sent_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_agent_records_agent_sent_at
    ON agent_records (agent_id, sent_at DESC);
CREATE INDEX idx_agent_records_agent_status_sent_at
    ON agent_records (agent_id, status, sent_at DESC);
