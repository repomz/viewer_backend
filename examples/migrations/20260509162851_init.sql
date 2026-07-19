-- +goose Up
-- +goose StatementBegin
CREATE TABLE studies (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    study_id        TEXT NOT NULL,
    patient         TEXT NOT NULL,
    age             INTEGER,
    department      TEXT NOT NULL,
    name_operation  TEXT NOT NULL,        
    study_type      TEXT NOT NULL,        
    descr_operation TEXT NOT NULL,
    time_beginning  TIMESTAMP,
    time_duration   INTEGER,
    surgeon         TEXT NOT NULL,
    dicom_link      TEXT,
    deleted         BOOLEAN NOT NULL DEFAULT FALSE
);

-- Индексы для быстрого поиска
CREATE INDEX idx_studies_time_beginning ON studies (time_beginning) WHERE NOT deleted;
CREATE INDEX idx_studies_surgeon ON studies (surgeon) WHERE NOT deleted;
CREATE INDEX idx_studies_study_type ON studies (study_type) WHERE NOT deleted;
CREATE INDEX idx_studies_time_surgeon ON studies (time_beginning, surgeon) WHERE NOT deleted;
CREATE INDEX idx_studies_time_type ON studies (time_beginning, study_type) WHERE NOT deleted;
CREATE INDEX idx_studies_surgeon_type ON studies (surgeon, study_type) WHERE NOT deleted;
CREATE INDEX idx_studies_time_surgeon_type ON studies (time_beginning, surgeon, study_type) WHERE NOT deleted;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE studies;
-- +goose StatementEnd
