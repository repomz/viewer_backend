-- name: CreateAgentRecord :exec
INSERT INTO agent_records (agent_id, status)
VALUES ($1, $2);

-- name: GetAgentRecordsByAgentID :many
SELECT sent_at FROM agent_records
WHERE agent_id = $1
ORDER BY sent_at DESC;

-- name: GetAgentRecordsByAgentIDandStatus :many
SELECT sent_at FROM agent_records
WHERE agent_id = $1 AND status = $2
ORDER BY sent_at DESC;

-- name: GetAgentRecordsByStatus :many
SELECT id FROM agent_records
WHERE status = $1
ORDER BY sent_at DESC;

-- name: DeleteAgentRecordsByAgentID :exec
DELETE FROM agent_records
WHERE agent_id = $1;

