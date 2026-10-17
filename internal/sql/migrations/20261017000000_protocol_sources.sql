-- +goose Up
CREATE TABLE protocol_sources (
    agent_id INTEGER NOT NULL CHECK (agent_id > 0),
    file_id UUID NOT NULL,
    study_id UUID NOT NULL REFERENCES studies(id),
    root TEXT NOT NULL,
    path TEXT NOT NULL,
    missing_since TIMESTAMPTZ,
    last_seen TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (agent_id, file_id)
);
CREATE INDEX protocol_sources_study ON protocol_sources(study_id);
CREATE INDEX protocol_sources_root ON protocol_sources(agent_id, root);
CREATE TABLE protocol_source_scans (
    agent_id INTEGER NOT NULL,
    root TEXT NOT NULL,
    last_scan UUID NOT NULL,
    PRIMARY KEY (agent_id, root)
);

-- +goose Down
DROP TABLE protocol_source_scans;
DROP TABLE protocol_sources;
