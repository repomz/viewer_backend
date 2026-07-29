package pgrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/repomz/viewer_backend/internal/app/db"
	"github.com/repomz/viewer_backend/internal/app/domain"
)

type UserRequestRepo struct {
	query *db.Queries
}

func NewUserRequestRepo(query *db.Queries) *UserRequestRepo {
	return &UserRequestRepo{query: query}
}

func (r UserRequestRepo) Create(ctx context.Context, request domain.NewUserRequest) (domain.UserRequest, error) {
	created, err := r.query.CreateUserRequest(ctx, db.CreateUserRequestParams{
		UserID:      request.UserID,
		AgentID:     request.AgentID,
		RequestType: request.RequestType,
		Command:     request.Command,
		Payload:     request.Payload,
		MaxAttempts: request.MaxAttempts,
	})
	if err != nil {
		return domain.UserRequest{}, fmt.Errorf("create user request: %w", err)
	}
	return dbUserRequestToDomain(created), nil
}

func (r UserRequestRepo) ClaimNext(ctx context.Context, agentID int32) (domain.UserRequest, error) {
	request, err := r.query.ClaimNextUserRequest(ctx, agentID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.UserRequest{}, domain.ErrNotFound
		}
		return domain.UserRequest{}, fmt.Errorf("claim next user request: %w", err)
	}
	return dbUserRequestToDomain(request), nil
}

func (r UserRequestRepo) RecordResult(ctx context.Context, result domain.UserRequestResult) (domain.UserRequest, error) {
	var (
		request db.UserRequest
		err     error
	)
	errorLog := sql.NullString{String: result.Error, Valid: result.Error != ""}

	switch {
	case result.OK:
		response := result.Result
		if len(response) == 0 {
			response = json.RawMessage(`{}`)
		}
		request, err = r.query.CompleteUserRequest(ctx, db.CompleteUserRequestParams{
			ID:      result.ID,
			AgentID: result.AgentID,
			Result:  response,
		})
	case result.Retryable:
		request, err = r.query.RetryUserRequest(ctx, db.RetryUserRequestParams{
			ID:       result.ID,
			AgentID:  result.AgentID,
			ErrorLog: errorLog,
		})
	default:
		request, err = r.query.FailUserRequest(ctx, db.FailUserRequestParams{
			ID:       result.ID,
			AgentID:  result.AgentID,
			ErrorLog: errorLog,
		})
	}
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			existing, getErr := r.query.GetUserRequestByID(ctx, result.ID)
			if getErr != nil {
				if errors.Is(getErr, sql.ErrNoRows) {
					return domain.UserRequest{}, domain.ErrNotFound
				}
				return domain.UserRequest{}, fmt.Errorf("get user request after result conflict: %w", getErr)
			}
			if existing.AgentID != result.AgentID {
				return domain.UserRequest{}, domain.ErrNotFound
			}
			resultWasRecorded := (result.OK && existing.Status == domain.UserRequestCompleted) ||
				(!result.OK && !result.Retryable && existing.Status == domain.UserRequestError) ||
				(!result.OK && result.Retryable &&
					(existing.Status == domain.UserRequestPending || existing.Status == domain.UserRequestError))
			if resultWasRecorded {
				return dbUserRequestToDomain(existing), nil
			}
			return domain.UserRequest{}, domain.ErrConflict
		}
		return domain.UserRequest{}, fmt.Errorf("record user request result: %w", err)
	}
	return dbUserRequestToDomain(request), nil
}

func (r UserRequestRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.UserRequest, error) {
	request, err := r.query.GetUserRequestByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.UserRequest{}, domain.ErrNotFound
		}
		return domain.UserRequest{}, fmt.Errorf("get user request: %w", err)
	}
	return dbUserRequestToDomain(request), nil
}

func (r UserRequestRepo) List(
	ctx context.Context,
	userID string,
	agentID int32,
	limit int32,
) ([]domain.UserRequest, error) {
	rows, err := r.query.ListUserRequests(ctx, db.ListUserRequestsParams{
		UserID: userID, AgentID: agentID, Limit: limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list user requests: %w", err)
	}
	result := make([]domain.UserRequest, 0, len(rows))
	for _, row := range rows {
		result = append(result, dbUserRequestToDomain(row))
	}
	return result, nil
}

func (r UserRequestRepo) Delete(ctx context.Context, id uuid.UUID, userID string) error {
	count, err := r.query.DeleteUserRequest(ctx, db.DeleteUserRequestParams{
		ID: id, UserID: userID,
	})
	if err != nil {
		return fmt.Errorf("delete user request: %w", err)
	}
	if count == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r UserRequestRepo) DeleteAll(ctx context.Context, userID string, agentID int32) error {
	_, err := r.query.DeleteAllUserRequests(ctx, db.DeleteAllUserRequestsParams{
		UserID: userID, AgentID: agentID,
	})
	if err != nil {
		return fmt.Errorf("delete all user requests: %w", err)
	}
	return nil
}

func dbUserRequestToDomain(request db.UserRequest) domain.UserRequest {
	var leaseExpiresAt, completedAt *time.Time
	if request.LeaseExpiresAt.Valid {
		value := request.LeaseExpiresAt.Time
		leaseExpiresAt = &value
	}
	if request.CompletedAt.Valid {
		value := request.CompletedAt.Time
		completedAt = &value
	}
	return domain.UserRequest{
		ID:             request.ID,
		CreatedAt:      request.CreatedAt,
		UpdatedAt:      request.UpdatedAt,
		AvailableAt:    request.AvailableAt,
		LeaseExpiresAt: leaseExpiresAt,
		CompletedAt:    completedAt,
		Status:         request.Status,
		UserID:         request.UserID,
		AgentID:        request.AgentID,
		RequestType:    request.RequestType,
		Command:        request.Command,
		Payload:        request.Payload,
		Result:         request.Result,
		ErrorLog:       request.ErrorLog.String,
		AttemptCount:   request.AttemptCount,
		MaxAttempts:    request.MaxAttempts,
	}
}
