-- name: CreateAgentRecord :exec
INSERT INTO agent_records (agent_id, status)
VALUES ($1, $2);

-- name: GetAgentRecordsByAgentID :many
SELECT sent_at FROM agent_records
WHERE agent_id = sqlc.arg(agent_id)
ORDER BY sent_at DESC
LIMIT sqlc.arg(result_limit);

-- name: GetAgentRecordsByAgentIDandStatus :many
SELECT sent_at FROM agent_records
WHERE agent_id = sqlc.arg(agent_id) AND status = sqlc.arg(status)
ORDER BY sent_at DESC
LIMIT sqlc.arg(result_limit);

-- name: GetAgentIDs :many
SELECT DISTINCT agent_id FROM agent_records
ORDER BY agent_id;

-- name: GetAgentRecordsByStatus :many
SELECT id FROM agent_records
WHERE status = $1
ORDER BY sent_at DESC;

-- name: DeleteAgentRecordsByAgentID :exec
DELETE FROM agent_records
WHERE agent_id = $1;
