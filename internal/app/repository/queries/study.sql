-- name: UpdateStudyDicomLink :one
UPDATE studies
SET dicom_link = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;
