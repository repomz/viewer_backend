-- name: CreateStudy :one
INSERT INTO studies (study_id, patient, age, department, name_operation, study_type, descr_operation, time_beginning, time_duration, surgeon, dicom_link)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: GetStudies :many
SELECT * FROM studies
WHERE deleted = false
  AND (sqlc.narg('date')::timestamp IS NULL OR time_beginning::date = sqlc.narg('date')::date)
  AND (sqlc.narg('type')::text IS NULL OR study_type = sqlc.narg('type'))
  AND (sqlc.narg('surgeon')::text IS NULL OR surgeon = sqlc.narg('surgeon'))
ORDER BY time_beginning DESC, created_at DESC;

-- name: GetStudyByID :one
SELECT * FROM studies
WHERE id = $1 AND deleted = false;

-- name: GetStudyByPatient :one
SELECT * FROM studies
WHERE patient = $1 AND deleted = false;

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
