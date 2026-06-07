package httpmodels

import (
	"fmt"

	"github.com/repomz/viewer_backend/internal/app/domain"
)

type AgentRecordRequest struct {
	AgentID int32  `json:"agent_id"`
	Status  string `json:"status"`
}

func (a *AgentRecordRequest) Validate() error {
	if a.AgentID != 1 && a.AgentID != 2 {
		return fmt.Errorf("%w: agent_id", domain.ErrRequired)
	}
	if a.Status == "" {
		return fmt.Errorf("%w: status", domain.ErrRequired)
	}
	return nil
}
