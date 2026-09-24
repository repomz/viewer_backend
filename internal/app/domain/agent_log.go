package domain

import "time"

// AgentLog is one completed hourly slice of a hospital agent log file.
type AgentLog struct {
	AgentID     int32     `json:"agent_id"`
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
	Content     string    `json:"content"`
	ReceivedAt  time.Time `json:"received_at"`
}
