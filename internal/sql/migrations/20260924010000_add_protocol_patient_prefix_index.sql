-- +goose Up
CREATE INDEX idx_studies_protocol_patient_prefix
    ON studies (
        lower(replace(split_part(btrim(patient), ' ', 1), 'ё', 'е')) text_pattern_ops,
        time_beginning DESC
    )
    WHERE deleted = false AND lower(study_type) NOT IN ('xa', 'ct');

-- +goose Down
DROP INDEX IF EXISTS idx_studies_protocol_patient_prefix;
