-- name: CreateStudy :one
INSERT INTO studies (study_id, patient, age, department, name_operation, study_type, descr_operation, recommendation, time_beginning, time_duration, surgeon, dicom_link)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
ON CONFLICT (lower(btrim(patient)), time_beginning, lower(btrim(name_operation)))
WHERE deleted = false
  AND lower(btrim(study_type)) NOT IN ('xa', 'ct')
  AND time_beginning IS NOT NULL
DO UPDATE SET
    study_id = EXCLUDED.study_id,
    patient = EXCLUDED.patient,
    age = EXCLUDED.age,
    department = EXCLUDED.department,
    name_operation = EXCLUDED.name_operation,
    study_type = EXCLUDED.study_type,
    descr_operation = EXCLUDED.descr_operation,
    recommendation = EXCLUDED.recommendation,
    time_beginning = EXCLUDED.time_beginning,
    time_duration = EXCLUDED.time_duration,
    surgeon = EXCLUDED.surgeon,
    dicom_link = COALESCE(NULLIF(EXCLUDED.dicom_link, ''), studies.dicom_link),
    updated_at = NOW()
RETURNING *;

-- name: GetStudies :many
SELECT * FROM studies
WHERE deleted = false
ORDER BY time_beginning DESC NULLS LAST, created_at DESC, id DESC
LIMIT $1 OFFSET $2;

-- name: GetProtocolStudiesSince :many
SELECT * FROM studies
WHERE deleted = false
  AND time_beginning >= $1
  AND lower(study_type) NOT IN ('xa', 'ct')
ORDER BY time_beginning DESC NULLS LAST, created_at DESC, id DESC
LIMIT $2 OFFSET $3;

-- name: GetProtocolStudyCandidates :many
SELECT * FROM studies
WHERE deleted = false
  AND lower(study_type) NOT IN ('xa', 'ct')
  AND time_beginning >= sqlc.arg(from_time)
  AND time_beginning < sqlc.arg(to_time)
  AND lower(replace(split_part(btrim(patient), ' ', 1), 'ё', 'е'))
      LIKE sqlc.arg(patient_prefix) || '%'
ORDER BY time_beginning DESC, created_at DESC, id DESC
LIMIT sqlc.arg(row_limit);

-- name: GetStudyByID :one
SELECT * FROM studies
WHERE id = $1 AND deleted = false;

-- name: GetStudyByStudyIDAndType :one
SELECT * FROM studies
WHERE study_id = $1 AND study_type = $2 AND deleted = false
ORDER BY created_at DESC
LIMIT 1;

-- name: GetStudyByPatient :one
SELECT * FROM studies
WHERE patient = $1 AND deleted = false
ORDER BY time_beginning DESC NULLS LAST, created_at DESC
LIMIT 1;

-- name: SoftDeleteStudy :exec
UPDATE studies SET deleted = true, updated_at = NOW()
WHERE id = $1;

-- name: SoftDeleteAllStudies :exec
UPDATE studies SET deleted = true, updated_at = NOW()
WHERE deleted = false;

-- name: UpdateStudyDicomLink :one
UPDATE studies
SET dicom_link = $2, updated_at = NOW()
WHERE id = $1 AND deleted = false
RETURNING *;

-- name: GetStudiesByDate :many
SELECT * FROM studies
WHERE time_beginning::date = $1 AND deleted = false
ORDER BY time_beginning DESC, created_at DESC;

-- name: GetStudiesBySurgeon :many
SELECT * FROM studies
WHERE surgeon = $1 AND deleted = false
ORDER BY time_beginning DESC, created_at DESC;

-- name: GetStudiesByStudyType :many
SELECT * FROM studies
WHERE study_type = $1 AND deleted = false
ORDER BY time_beginning DESC, created_at DESC;

-- name: GetStudiesByDateAndSurgeon :many
SELECT * FROM studies
WHERE time_beginning::date = $1 AND surgeon = $2 AND deleted = false
ORDER BY time_beginning DESC, created_at DESC;

-- name: GetStudiesByDateAndStudyType :many
SELECT * FROM studies
WHERE time_beginning::date = $1 AND study_type = $2 AND deleted = false
ORDER BY time_beginning DESC, created_at DESC;

-- name: GetStudiesBySurgeonAndStudyType :many
SELECT * FROM studies
WHERE surgeon = $1 AND study_type = $2 AND deleted = false
ORDER BY time_beginning DESC, created_at DESC;

-- name: GetStudiesByDateSurgeonStudyType :many
SELECT * FROM studies
WHERE time_beginning::date = $1 AND surgeon = $2 AND study_type = $3 AND deleted = false
ORDER BY time_beginning DESC, created_at DESC;

-- -- name: CountStudiesByDate :one
-- SELECT COUNT(*) FROM studies WHERE time_beginning::date = $1 AND deleted = false;

-- -- name: CountStudiesBySurgeon :one
-- SELECT COUNT(*) FROM studies WHERE surgeon = $1 AND deleted = false;

-- -- name: CountStudiesByStudyType :one
-- SELECT COUNT(*) FROM studies WHERE study_type = $1 AND deleted = false;

-- -- name: CountStudiesByDateAndSurgeon :one
-- SELECT COUNT(*) FROM studies WHERE time_beginning::date = $1 AND surgeon = $2 AND deleted = false;

-- -- name: CountStudiesByDateAndStudyType :one
-- SELECT COUNT(*) FROM studies WHERE time_beginning::date = $1 AND study_type = $2 AND deleted = false;

-- -- name: CountStudiesBySurgeonAndStudyType :one
-- SELECT COUNT(*) FROM studies WHERE surgeon = $1 AND study_type = $2 AND deleted = false;

-- -- name: CountStudiesByDateSurgeonStudyType :one
-- SELECT COUNT(*) FROM studies WHERE time_beginning::date = $1 AND surgeon = $2 AND study_type = $3 AND deleted = false;
