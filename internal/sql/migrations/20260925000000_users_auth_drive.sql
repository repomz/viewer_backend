-- +goose Up
CREATE TABLE app_users (
    id            BIGSERIAL PRIMARY KEY,
    display_name  TEXT NOT NULL,
    login         TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('user', 'admin')),
    quota_bytes   BIGINT NOT NULL DEFAULT 1073741824 CHECK (quota_bytes > 0),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX app_users_login_unique ON app_users (LOWER(login));

CREATE TABLE auth_sessions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     BIGINT NOT NULL REFERENCES app_users(id) ON DELETE CASCADE,
    token_hash  BYTEA NOT NULL UNIQUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at  TIMESTAMPTZ NOT NULL
);

CREATE INDEX auth_sessions_user_expiry ON auth_sessions (user_id, expires_at DESC);

CREATE TABLE login_events (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES app_users(id) ON DELETE CASCADE,
    logged_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX login_events_user_time ON login_events (user_id, logged_at DESC);

CREATE TABLE drive_files (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       BIGINT NOT NULL REFERENCES app_users(id) ON DELETE CASCADE,
    stored_name   TEXT NOT NULL UNIQUE,
    original_name TEXT NOT NULL,
    content_type  TEXT NOT NULL DEFAULT 'application/octet-stream',
    size_bytes    BIGINT NOT NULL CHECK (size_bytes >= 0),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX drive_files_user_created ON drive_files (user_id, created_at DESC);

INSERT INTO app_users (display_name, login, password_hash, role) VALUES
    ('Марат', 'marat', '$2y$12$O4/DGs2ejwLcjRhuL4xMzu2WZMFaTWI.x9iNS9/oURXr97mx7SODi', 'user'),
    ('Максим', 'maks', '$2y$12$uqu8ZjbKT0P0eD7zzhpubehoTYuI9tzVQ6Ku8orIiqTyxTUGSf3AO', 'user'),
    ('Александр', 'aleksandr', '$2y$12$uqu8ZjbKT0P0eD7zzhpubehoTYuI9tzVQ6Ku8orIiqTyxTUGSf3AO', 'user'),
    ('Артем', 'artem', '$2y$12$uqu8ZjbKT0P0eD7zzhpubehoTYuI9tzVQ6Ku8orIiqTyxTUGSf3AO', 'user'),
    ('Администратор', 'admin', '$2y$12$O4/DGs2ejwLcjRhuL4xMzu2WZMFaTWI.x9iNS9/oURXr97mx7SODi', 'admin');

-- +goose Down
DROP TABLE drive_files;
DROP TABLE login_events;
DROP TABLE auth_sessions;
DROP TABLE app_users;
