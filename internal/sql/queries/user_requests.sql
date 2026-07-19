-- name: CreateUserRequest :one
INSERT INTO user_requests (
    user_id,
    agent_id,
    request_type,
    command,
    payload,
    max_attempts
)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ClaimNextUserRequest :one
WITH next_request AS (
    SELECT id
    FROM user_requests
    WHERE user_requests.agent_id = $1
      AND user_requests.attempt_count < user_requests.max_attempts
      AND (
          (status = 'pending' AND available_at <= NOW())
          OR (status = 'in_process' AND lease_expires_at <= NOW())
      )
    ORDER BY available_at ASC, created_at ASC
    FOR UPDATE SKIP LOCKED
    LIMIT 1
)
UPDATE user_requests AS request
SET status = 'in_process',
    attempt_count = request.attempt_count + 1,
    updated_at = NOW(),
    lease_expires_at = NOW() + INTERVAL '5 minutes'
FROM next_request
WHERE request.id = next_request.id
RETURNING request.*;

-- name: CompleteUserRequest :one
UPDATE user_requests
SET status = 'completed',
    result = $3,
    error_log = NULL,
    updated_at = NOW(),
    completed_at = NOW(),
    lease_expires_at = NULL
WHERE id = $1
  AND agent_id = $2
  AND status = 'in_process'
RETURNING *;

-- name: RetryUserRequest :one
UPDATE user_requests
SET status = CASE
        WHEN attempt_count < max_attempts THEN 'pending'
        ELSE 'failed'
    END,
    error_log = $3,
    updated_at = NOW(),
    available_at = NOW() + INTERVAL '30 seconds',
    lease_expires_at = NULL,
    completed_at = CASE
        WHEN attempt_count < max_attempts THEN NULL
        ELSE NOW()
    END
WHERE id = $1
  AND agent_id = $2
  AND status = 'in_process'
RETURNING *;

-- name: FailUserRequest :one
UPDATE user_requests
SET status = 'failed',
    error_log = $3,
    updated_at = NOW(),
    completed_at = NOW(),
    lease_expires_at = NULL
WHERE id = $1
  AND agent_id = $2
  AND status = 'in_process'
RETURNING *;

-- name: GetUserRequestByID :one
SELECT *
FROM user_requests
WHERE id = $1;

-- name: GetOldRequestsForArchive :many
SELECT *
FROM user_requests
WHERE status IN ('completed', 'failed')
  AND completed_at < NOW() - INTERVAL '3 days'
ORDER BY completed_at ASC;

-- name: DeleteOldRequests :exec
DELETE FROM user_requests
WHERE status IN ('completed', 'failed')
  AND completed_at < NOW() - INTERVAL '3 days';
