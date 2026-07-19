package httpmodels

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/repomz/viewer_backend/internal/app/domain"
)

var allowedAgentCommands = map[string]bool{
	"send_study_to_yandex":       true,
	"send_dicom_to_mapdr":        true,
	"generate_operations_report": true,
}

type UserRequestCreateRequest struct {
	UserID      string         `json:"user_id"`
	AgentID     int32          `json:"agent_id"`
	RequestType string         `json:"request_type"`
	Command     string         `json:"command"`
	Payload     map[string]any `json:"payload"`
	MaxAttempts int32          `json:"max_attempts"`
}

func (r *UserRequestCreateRequest) Validate() error {
	r.UserID = strings.TrimSpace(r.UserID)
	r.RequestType = strings.TrimSpace(r.RequestType)
	r.Command = strings.ToLower(strings.TrimSpace(r.Command))
	if r.UserID == "" {
		return fmt.Errorf("%w: user_id", domain.ErrRequired)
	}
	if r.AgentID <= 0 {
		return fmt.Errorf("%w: agent_id", domain.ErrInvalidAgentID)
	}
	if !allowedAgentCommands[r.Command] {
		return fmt.Errorf("%w: unsupported command %q", domain.ErrInvalidCommand, r.Command)
	}
	if r.RequestType == "" {
		r.RequestType = "execute_command"
	}
	if r.Payload == nil {
		r.Payload = map[string]any{}
	}
	switch r.Command {
	case "send_study_to_yandex":
		if strings.TrimSpace(stringValue(r.Payload["study_uid"])) == "" {
			return fmt.Errorf("%w: payload.study_uid", domain.ErrRequired)
		}
	case "send_dicom_to_mapdr":
		if strings.TrimSpace(stringValue(r.Payload["dicom_path"])) == "" {
			return fmt.Errorf("%w: payload.dicom_path", domain.ErrRequired)
		}
	}
	if r.MaxAttempts == 0 {
		r.MaxAttempts = 3
	}
	if r.MaxAttempts < 1 || r.MaxAttempts > 20 {
		return fmt.Errorf("%w: max_attempts must be between 1 and 20", domain.ErrInvalidRequest)
	}
	return nil
}

type UserRequestResultRequest struct {
	AgentID   int32          `json:"agent_id"`
	OK        bool           `json:"ok"`
	Retryable bool           `json:"retryable"`
	Result    map[string]any `json:"result"`
	Error     string         `json:"error"`
}

func (r *UserRequestResultRequest) Validate() error {
	r.Error = strings.TrimSpace(r.Error)
	if r.AgentID <= 0 {
		return fmt.Errorf("%w: agent_id", domain.ErrInvalidAgentID)
	}
	if r.OK {
		r.Retryable = false
		if r.Result == nil {
			r.Result = map[string]any{}
		}
		return nil
	}
	if r.Error == "" {
		return fmt.Errorf("%w: error", domain.ErrRequired)
	}
	return nil
}

type UserRequestResponse struct {
	ID             uuid.UUID       `json:"id"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
	AvailableAt    time.Time       `json:"available_at"`
	LeaseExpiresAt *time.Time      `json:"lease_expires_at,omitempty"`
	CompletedAt    *time.Time      `json:"completed_at,omitempty"`
	Status         string          `json:"status"`
	UserID         string          `json:"user_id"`
	AgentID        int32           `json:"agent_id"`
	RequestType    string          `json:"request_type"`
	Command        string          `json:"command"`
	Payload        json.RawMessage `json:"payload"`
	Result         json.RawMessage `json:"result"`
	Error          string          `json:"error,omitempty"`
	AttemptCount   int32           `json:"attempt_count"`
	MaxAttempts    int32           `json:"max_attempts"`
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}
