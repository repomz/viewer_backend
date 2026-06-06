-- name: CreateAgentRecord :one
INSERT INTO agent_records (agent_id, status, sent_at)
VALUES ($1, $2, $3);

-- name: GetAgentRecordByAgentID :many
SELECT sent_at FROM agent_records
WHERE agent_id = $1
ORDER BY created_at ASC;

-- name: GetAgentRecordByAgentIDandStatus :many
SELECT sent_at FROM agent_records
WHERE agent_id = $1 AND status = $2
ORDER BY sent_at DESC;

-- name: GetAgentRecordByStatus :many
SELECT id FROM agent_records
WHERE status = $1
ORDER BY sent_at DESC;

-- name: DeleteAgentRecords :many
DELETE * FROM agent_records
WHERE agent_id = $1
