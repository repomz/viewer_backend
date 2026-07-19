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

