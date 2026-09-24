package services

import (
	"context"
	"time"

	"github.com/repomz/viewer_backend/internal/app/domain"
)

type AgentLogService struct {
	repo AgentLogRepository
}

func NewAgentLogService(repo AgentLogRepository) AgentLogService {
	return AgentLogService{repo: repo}
}

func (s AgentLogService) Upsert(ctx context.Context, entry domain.AgentLog) error {
	return s.repo.Upsert(ctx, entry)
}

func (s AgentLogService) List(ctx context.Context, agentID int32, from, to time.Time) ([]domain.AgentLog, error) {
	return s.repo.List(ctx, agentID, from, to)
}
