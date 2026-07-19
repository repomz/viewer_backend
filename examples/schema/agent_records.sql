CREATE TABLE agent_records (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id        INTEGER NOT NULL,
    status          TEXT NOT NULL,
    sent_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);