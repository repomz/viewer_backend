package httpmodels

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/repomz/viewer_backend/internal/app/domain"
)

type AgentRecordRequest struct {
	AgentID string `json:"agent_id"`
	Status  string `json:"status"`
}

func (a *AgentRecordRequest) Validate() error {
	a.AgentID = strings.TrimSpace(a.AgentID)
	a.Status = strings.ToLower(strings.TrimSpace(a.Status))

	agentID, err := strconv.ParseInt(a.AgentID, 10, 32)
	if err != nil || (agentID != 1 && agentID != 2) {
		return fmt.Errorf("%w: agent_id must be 1 or 2", domain.ErrInvalidAgentID)
	}
	if a.Status != "well" && a.Status != "with_errors" {
		return fmt.Errorf("%w: status must be well or with_errors", domain.ErrInvalidStatus)
	}
	return nil
}
