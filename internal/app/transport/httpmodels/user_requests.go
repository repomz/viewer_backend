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
	"get_report":      true,
	"get_plan":        true,
	"find_study":      true,
	"import_study":    true,
	"find_xa":         true,
	"find_ct":         true,
	"get_xa":          true,
	"get_ct":          true,
	"send_xa_to_pacs": true,
	"send_ct_to_pacs": true,
	"xa_polling_on":   true,
	"xa_polling_off":  true,
	"ct_polling_on":   true,
	"ct_polling_off":  true,
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
	case "get_xa", "get_ct", "send_xa_to_pacs", "send_ct_to_pacs":
		if strings.TrimSpace(stringValue(r.Payload["study_uid"])) == "" {
			return fmt.Errorf("%w: payload.study_uid", domain.ErrRequired)
		}
	case "import_study":
		if strings.TrimSpace(stringValue(r.Payload["protocol_ref"])) == "" {
			return fmt.Errorf("%w: payload.protocol_ref", domain.ErrRequired)
		}
	case "find_xa", "find_ct", "find_study":
		if strings.TrimSpace(stringValue(r.Payload["patient"])) == "" &&
			strings.TrimSpace(stringValue(r.Payload["patient_name"])) == "" {
			return fmt.Errorf("%w: payload.patient", domain.ErrRequired)
		}
	case "get_report":
		if value, exists := r.Payload["period"]; exists {
			period, valid := integerValue(value)
			if !valid || period < 1 || period > 4 {
				return fmt.Errorf(
					"%w: payload.period must be an integer between 1 and 4",
					domain.ErrInvalidRequest,
				)
			}
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
	Errors    string         `json:"errors"`
}

func (r *UserRequestResultRequest) Validate() error {
	r.Errors = strings.TrimSpace(r.Errors)
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
	if r.Errors == "" {
		return fmt.Errorf("%w: errors", domain.ErrRequired)
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
	Errors         string          `json:"errors,omitempty"`
	AttemptCount   int32           `json:"attempt_count"`
	MaxAttempts    int32           `json:"max_attempts"`
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func integerValue(value any) (int64, bool) {
	switch typed := value.(type) {
	case float64:
		converted := int64(typed)
		return converted, float64(converted) == typed
	case int:
		return int64(typed), true
	case int32:
		return int64(typed), true
	case int64:
		return typed, true
	case json.Number:
		converted, err := typed.Int64()
		return converted, err == nil
	default:
		return 0, false
	}
}
