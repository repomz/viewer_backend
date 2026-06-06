package services

import (
	"context"

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

func (s AgentRecordsService) GetAgentRecordsByAgentID(ctx context.Context, id int16) (domain.AgentRecord, error) {
	return s.repo.GetAgentRecordsByAgentID(ctx, id)
}

func (s AgentRecordsService) GetAgentRecordsByAgentIDandStatust(ctx context.Context, id int16, status string) (domain.AgentRecord, error) {
	return s.repo.GetAgentRecordsByAgentIDandStatus(ctx, id, status)
}

func (s AgentRecordsService) CreateAgentRecord(ctx context.Context, record domain.AgentRecord) error {
	return s.repo.CreateAgentRecord(ctx, record)
}

func (s AgentRecordsService) DeleteAllAgentRecords(ctx context.Context) error {
	return s.repo.DeleteAllAgentRecords(ctx)
}
