-- +goose Up
-- Keep the first stored copy of an operation and softly retire later copies.
-- Imaging studies are excluded because XA/CT lifecycle has its own identity.
WITH ranked_protocols AS (
    SELECT
        id,
        ROW_NUMBER() OVER (
            PARTITION BY
                lower(btrim(patient)),
                time_beginning,
                lower(btrim(name_operation))
            ORDER BY created_at ASC, id ASC
        ) AS duplicate_number
    FROM studies
    WHERE deleted = false
      AND lower(btrim(study_type)) NOT IN ('xa', 'ct')
      AND time_beginning IS NOT NULL
)
UPDATE studies AS study
SET deleted = true,
    updated_at = NOW()
FROM ranked_protocols AS ranked
WHERE study.id = ranked.id
  AND ranked.duplicate_number > 1;

CREATE UNIQUE INDEX uq_studies_active_protocol_identity
    ON studies (
        lower(btrim(patient)),
        time_beginning,
        lower(btrim(name_operation))
    )
    WHERE deleted = false
      AND lower(btrim(study_type)) NOT IN ('xa', 'ct')
      AND time_beginning IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS uq_studies_active_protocol_identity;
