package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/repomz/viewer_backend/internal/app/domain"
)

type UserRequestService struct {
	repo UserRequestRepository
}

func NewUserRequestService(repo UserRequestRepository) UserRequestService {
	return UserRequestService{repo: repo}
}

func (s UserRequestService) Create(ctx context.Context, request domain.NewUserRequest) (domain.UserRequest, error) {
	if request.AgentID <= 0 {
		return domain.UserRequest{}, fmt.Errorf("%w: agent_id", domain.ErrInvalidAgentID)
	}
	if request.UserID == "" {
		return domain.UserRequest{}, fmt.Errorf("%w: user_id", domain.ErrRequired)
	}
	if request.Command == "" {
		return domain.UserRequest{}, fmt.Errorf("%w: command", domain.ErrRequired)
	}
	return s.repo.Create(ctx, request)
}

func (s UserRequestService) ClaimNext(ctx context.Context, agentID int32) (domain.UserRequest, error) {
	if agentID <= 0 {
		return domain.UserRequest{}, fmt.Errorf("%w: agent_id", domain.ErrInvalidAgentID)
	}
	return s.repo.ClaimNext(ctx, agentID)
}

func (s UserRequestService) RecordResult(ctx context.Context, result domain.UserRequestResult) (domain.UserRequest, error) {
	if result.ID == uuid.Nil {
		return domain.UserRequest{}, fmt.Errorf("%w: request_id", domain.ErrRequired)
	}
	if result.AgentID <= 0 {
		return domain.UserRequest{}, fmt.Errorf("%w: agent_id", domain.ErrInvalidAgentID)
	}
	return s.repo.RecordResult(ctx, result)
}

func (s UserRequestService) GetByID(ctx context.Context, id uuid.UUID) (domain.UserRequest, error) {
	if id == uuid.Nil {
		return domain.UserRequest{}, fmt.Errorf("%w: request_id", domain.ErrRequired)
	}
	return s.repo.GetByID(ctx, id)
}
