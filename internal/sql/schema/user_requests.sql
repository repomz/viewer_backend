CREATE TABLE user_requests (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    status        TEXT NOT NULL,
    user_id       TEXT NOT NULL,
    request_type  TEXT NOT NULL,
    command       TEXT,
    study_id      TEXT,        
    xa_id         TEXT,        
    ct_id         TEXT,
    study_filter  TEXT,
    xa_filter     TEXT,
    ct_filter     TEXT,
    error_log     TEXT
);

CREATE INDEX idx_user_requests_queue ON user_requests (status, created_at);