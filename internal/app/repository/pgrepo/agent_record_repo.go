package pgrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/repomz/viewer_backend/internal/app/db"
	"github.com/repomz/viewer_backend/internal/app/domain"
)

type AgentRecordRepo struct {
	query *db.Queries
}

func NewAgentRecordRepo(qr *db.Queries) *AgentRecordRepo {
	return &AgentRecordRepo{
		query: qr,
	}
}

func (a AgentRecordRepo) GetAgentIDs(ctx context.Context) ([]int32, error) {
	ids, err := a.query.GetAgentIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list agent IDs: %w", err)
	}
	return ids, nil
}

func (a AgentRecordRepo) DeleteAllAgentRecords(ctx context.Context, agent_id int32) error {
	if agent_id <= 0 {
		return fmt.Errorf("%w: agent_id", domain.ErrInvalidAgentID)
	}

	err := a.query.DeleteAgentRecordsByAgentID(ctx, agent_id)
	if err != nil {
		return fmt.Errorf("failed to delete agent_id %v records: %w", agent_id, err)
	}

	return nil
}

func (a AgentRecordRepo) CreateAgentRecord(ctx context.Context, agentRecord domain.AgentRecord) error {
	if agentRecord.AgentID() <= 0 {
		return fmt.Errorf("%w: agent_id", domain.ErrInvalidAgentID)
	}
	if agentRecord.Status() != "well" && agentRecord.Status() != "with_errors" {
		return fmt.Errorf("%w: status", domain.ErrInvalidStatus)
	}

	agentRecordParams := domainToDBagentRecordParams(agentRecord)

	err := a.query.CreateAgentRecord(ctx, agentRecordParams)
	if err != nil {
		return fmt.Errorf("failed to insert a agent record: %w", err)
	}

	return nil

}

func (a AgentRecordRepo) GetAgentRecordsByAgentIDandStatus(ctx context.Context, agent_id int32, status string) ([]time.Time, error) {

	if agent_id <= 0 {
		return []time.Time{}, fmt.Errorf("%w: agent_id", domain.ErrInvalidAgentID)
	}

	if status != "well" && status != "with_errors" {
		return []time.Time{}, fmt.Errorf("%w: status", domain.ErrInvalidStatus)
	}

	arg := db.GetAgentRecordsByAgentIDandStatusParams{
		AgentID: agent_id,
		Status:  status,
	}

	times, err := a.query.GetAgentRecordsByAgentIDandStatus(ctx, arg)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []time.Time{}, domain.ErrNotFound
		}
		return []time.Time{}, fmt.Errorf("failed to get a agent time record: %w", err)
	}

	return times, nil
}
func (a AgentRecordRepo) GetAgentRecordsByAgentID(ctx context.Context, agent_id int32) ([]time.Time, error) {

	if agent_id <= 0 {
		return []time.Time{}, fmt.Errorf("%w: agent_id", domain.ErrInvalidAgentID)
	}
	times, err := a.query.GetAgentRecordsByAgentID(ctx, agent_id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []time.Time{}, domain.ErrNotFound
		}
		return []time.Time{}, fmt.Errorf("failed to get a agent time record: %w", err)
	}

	return times, nil
}
