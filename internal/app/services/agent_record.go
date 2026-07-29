package services

import (
	"context"
	"time"

	"github.com/repomz/viewer_backend/internal/app/domain"
)

// AgentRecordsService is a AgentRecords service
type AgentRecordsService struct {
	repo AgentRecordsRepository
}

// NewAgentRecordsService creates a new AgentRecords service
func NewAgentRecordsService(repo AgentRecordsRepository) AgentRecordsService {
	return AgentRecordsService{
		repo: repo,
	}
}

func (a AgentRecordsService) GetAgentIDs(ctx context.Context) ([]int32, error) {
	return a.repo.GetAgentIDs(ctx)
}

func (a AgentRecordsService) GetAgentRecordsByAgentID(ctx context.Context, id int32) ([]time.Time, error) {
	return a.repo.GetAgentRecordsByAgentID(ctx, id)
}

func (a AgentRecordsService) GetAgentRecordsByAgentIDandStatus(ctx context.Context, id int32, status string) ([]time.Time, error) {
	return a.repo.GetAgentRecordsByAgentIDandStatus(ctx, id, status)
}

func (a AgentRecordsService) CreateAgentRecord(ctx context.Context, record domain.AgentRecord) error {
	return a.repo.CreateAgentRecord(ctx, record)
}

func (a AgentRecordsService) DeleteAllAgentRecords(ctx context.Context, agent_id int32) error {
	return a.repo.DeleteAllAgentRecords(ctx, agent_id)
}
