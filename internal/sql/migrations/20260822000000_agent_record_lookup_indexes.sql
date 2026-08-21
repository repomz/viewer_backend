-- +goose Up
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_agent_records_agent_sent_at
    ON agent_records (agent_id, sent_at DESC);
CREATE INDEX IF NOT EXISTS idx_agent_records_agent_status_sent_at
    ON agent_records (agent_id, status, sent_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_agent_records_agent_status_sent_at;
DROP INDEX IF EXISTS idx_agent_records_agent_sent_at;
-- +goose StatementEnd
