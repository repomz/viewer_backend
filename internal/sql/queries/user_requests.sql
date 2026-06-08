-- name: CreateUserRequest :one
INSERT INTO user_requests (
    status, user_id, request_type, command, 
    study_id, xa_id, ct_id, 
    study_filter, xa_filter, ct_filter,
    error_log
)
VALUES (
    'pending', $1, $2, $3, 
    $4, $5, $6, 
    $7, $8, $9,
    NULL
)
RETURNING id; 

-- name: GetAndProcessNextUserRequest :one
UPDATE user_requests
SET 
    status = 'in_process',
    updated_at = NOW()
WHERE id = (
    SELECT id 
    FROM user_requests 
    WHERE 
        status = 'pending'
        OR (
            status = 'in_process' 
            AND updated_at < NOW() - CASE 
                WHEN request_type IN ('find_study', 'find_xa', 'find_ct', 'execute_command') THEN INTERVAL '5 minutes'
                WHEN request_type IN ('get_xa', 'get_ct') THEN INTERVAL '15 minutes'
                ELSE INTERVAL '30 minutes' -- Дефолтный таймаут на случай появления новых типов
            END
        )
    ORDER BY created_at ASC 
    LIMIT 1
)
RETURNING *;

-- name: CompleteUserRequest :one
UPDATE user_requests
SET 
    status = 'completed',
    updated_at = NOW()
WHERE id = $1 AND status = 'in_process'
RETURNING *;

-- name: FailUserRequest :one
UPDATE user_requests
SET 
    status = 'failed',
    updated_at = NOW(),
    error_log = $2
WHERE id = $1 AND status = 'in_process'
RETURNING *;

-- name: GetOldRequestsForArchive :many
SELECT * 
FROM user_requests
WHERE status IN ('completed', 'failed')
  AND updated_at < NOW() - INTERVAL '3 days'
ORDER BY updated_at ASC;

-- name: DeleteOldRequests :exec
DELETE FROM user_requests
WHERE status IN ('completed', 'failed')
  AND updated_at < NOW() - INTERVAL '3 days';