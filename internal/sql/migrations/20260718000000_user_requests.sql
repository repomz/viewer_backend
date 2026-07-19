-- +goose Up
-- +goose StatementBegin
CREATE TABLE user_requests (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    available_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    lease_expires_at TIMESTAMPTZ,
    completed_at     TIMESTAMPTZ,
    status           TEXT NOT NULL DEFAULT 'pending'
                     CHECK (status IN ('pending', 'in_process', 'completed', 'failed')),
    user_id          TEXT NOT NULL,
    agent_id         INTEGER NOT NULL CHECK (agent_id > 0),
    request_type     TEXT NOT NULL DEFAULT 'execute_command',
    command          TEXT NOT NULL,
    payload          JSONB NOT NULL DEFAULT '{}'::jsonb
                     CHECK (jsonb_typeof(payload) = 'object'),
    result           JSONB NOT NULL DEFAULT '{}'::jsonb
                     CHECK (jsonb_typeof(result) = 'object'),
    error_log        TEXT,
    attempt_count    INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    max_attempts     INTEGER NOT NULL DEFAULT 3 CHECK (max_attempts BETWEEN 1 AND 20)
);

CREATE INDEX idx_user_requests_queue
    ON user_requests (agent_id, status, available_at, created_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE user_requests;
-- +goose StatementEnd
