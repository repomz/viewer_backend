package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const (
	UserRequestPending   = "pending"
	UserRequestInProcess = "in_process"
	UserRequestCompleted = "completed"
	UserRequestFailed    = "failed"
)

type UserRequest struct {
	ID             uuid.UUID
	CreatedAt      time.Time
	UpdatedAt      time.Time
	AvailableAt    time.Time
	LeaseExpiresAt *time.Time
	CompletedAt    *time.Time
	Status         string
	UserID         string
	AgentID        int32
	RequestType    string
	Command        string
	Payload        json.RawMessage
	Result         json.RawMessage
	ErrorLog       string
	AttemptCount   int32
	MaxAttempts    int32
}

type NewUserRequest struct {
	UserID      string
	AgentID     int32
	RequestType string
	Command     string
	Payload     json.RawMessage
	MaxAttempts int32
}

type UserRequestResult struct {
	ID        uuid.UUID
	AgentID   int32
	OK        bool
	Retryable bool
	Result    json.RawMessage
	Error     string
}
