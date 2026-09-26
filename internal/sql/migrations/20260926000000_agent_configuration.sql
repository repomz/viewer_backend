-- +goose Up
CREATE TABLE agent_configuration (
    agent_id INTEGER PRIMARY KEY CHECK (agent_id > 0),
    configuration JSONB NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE agent_configuration;
