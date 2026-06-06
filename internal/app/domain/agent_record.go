package domain

import (
	"time"
)

// AgentRecord is a domain agent record.
type AgentRecord struct {
	id       id
	agent_id int16
	status   string
	sentAt   time.Time
}
