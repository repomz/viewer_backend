-- name: CreateStudy :one
INSERT INTO studies (study_id, patient, age, department, name_operation, study_type, descr_operation, time_beginning, time_duration, surgeon, dicom_link)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: GetStudies :many
SELECT * FROM studies
WHERE deleted = false
ORDER BY created_at ASC;

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