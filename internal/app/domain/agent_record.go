package domain

import (
	"time"

	"github.com/google/uuid"
)

// AgentRecord is a domain agent record.
type AgentRecord struct {
	id       uuid.UUID
	agent_id int32
	status   string
	sentAt   time.Time
}

// These methods return the db.AgentRecord
func (a AgentRecord) AgentID() int32 {
	return a.agent_id
}

func (a AgentRecord) Status() string {
	return a.status
}

// Для перевода структуры requestAgentRecord в domain.AgentRecord
type DBAgentRecordData struct {
	AgentID int32
	Status  string
}

// Для перевода структуры requestAgentRecord в domain.AgentRecord
func RequestToDBAgentRecord(data DBAgentRecordData) AgentRecord {
	return AgentRecord{
		agent_id: data.AgentID,
		status:   data.Status,
	}
}
