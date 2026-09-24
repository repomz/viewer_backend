package pgrepo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/repomz/viewer_backend/internal/app/domain"
)

// AgentLogRepo stores complete hourly log slices without duplicating them on retry.
type AgentLogRepo struct {
	db *sql.DB
}

func NewAgentLogRepo(database *sql.DB) *AgentLogRepo {
	return &AgentLogRepo{db: database}
}

func (r AgentLogRepo) Upsert(ctx context.Context, entry domain.AgentLog) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO agent_logs (agent_id, period_start, period_end, content)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (agent_id, period_start) DO UPDATE SET
			period_end = EXCLUDED.period_end,
			content = EXCLUDED.content,
			received_at = NOW()
	`, entry.AgentID, entry.PeriodStart, entry.PeriodEnd, entry.Content)
	if err != nil {
		return fmt.Errorf("upsert agent log: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `DELETE FROM agent_logs WHERE period_start < NOW() - INTERVAL '7 days'`)
	if err != nil {
		return fmt.Errorf("prune agent logs: %w", err)
	}
	return nil
}

func (r AgentLogRepo) List(ctx context.Context, agentID int32, from, to time.Time) ([]domain.AgentLog, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT agent_id, period_start, period_end, content, received_at
		FROM agent_logs
		WHERE agent_id = $1 AND period_start >= $2 AND period_start < $3
		ORDER BY period_start ASC
	`, agentID, from, to)
	if err != nil {
		return nil, fmt.Errorf("list agent logs: %w", err)
	}
	defer rows.Close()

	entries := make([]domain.AgentLog, 0)
	for rows.Next() {
		var entry domain.AgentLog
		if err := rows.Scan(
			&entry.AgentID,
			&entry.PeriodStart,
			&entry.PeriodEnd,
			&entry.Content,
			&entry.ReceivedAt,
		); err != nil {
			return nil, fmt.Errorf("scan agent log: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate agent logs: %w", err)
	}
	return entries, nil
}
