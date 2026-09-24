package httpserver

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/repomz/viewer_backend/internal/app/common/server"
	"github.com/repomz/viewer_backend/internal/app/domain"
)

const maxAgentLogBytes = 8 << 20

type agentLogRequest struct {
	AgentID     int32     `json:"agent_id"`
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
	Content     string    `json:"content"`
}

func (h HttpServer) CreateAgentLog(w http.ResponseWriter, r *http.Request) {
	if h.agentLogService == nil {
		server.RespondWithError(errors.New("agent log service is unavailable"), w, r)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxAgentLogBytes)
	var request agentLogRequest
	if err := decodeJSON(w, r, &request); err != nil {
		server.BadRequest("invalid-json", err, w, r)
		return
	}
	request.Content = strings.TrimSpace(request.Content)
	if request.AgentID <= 0 || request.PeriodStart.IsZero() || request.PeriodEnd.IsZero() ||
		!request.PeriodEnd.After(request.PeriodStart) || request.Content == "" {
		server.BadRequest("invalid-request", errors.New("agent_id, period and content are required"), w, r)
		return
	}
	if request.PeriodEnd.Sub(request.PeriodStart) > 2*time.Hour {
		server.BadRequest("invalid-period", errors.New("log period is too large"), w, r)
		return
	}
	if err := h.agentLogService.Upsert(r.Context(), domain.AgentLog{
		AgentID: request.AgentID, PeriodStart: request.PeriodStart,
		PeriodEnd: request.PeriodEnd, Content: request.Content,
	}); err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	server.RespondCreated(map[string]bool{"stored": true}, w, r)
}

func (h HttpServer) GetAgentLogs(w http.ResponseWriter, r *http.Request) {
	if h.agentLogService == nil {
		server.RespondWithError(errors.New("agent log service is unavailable"), w, r)
		return
	}
	agentID64, err := strconv.ParseInt(r.URL.Query().Get("agent_id"), 10, 32)
	if err != nil || agentID64 <= 0 {
		server.BadRequest("invalid-agent-id", errors.New("positive agent_id is required"), w, r)
		return
	}
	location, err := time.LoadLocation("Asia/Tomsk")
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	day, err := time.ParseInLocation("2006-01-02", r.URL.Query().Get("date"), location)
	if err != nil {
		server.BadRequest("invalid-date", errors.New("date must use YYYY-MM-DD format"), w, r)
		return
	}
	now := time.Now().In(location)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	if day.Before(today.AddDate(0, 0, -6)) || day.After(today) {
		server.BadRequest("date-out-of-range", errors.New("only the last 7 days are available"), w, r)
		return
	}
	entries, err := h.agentLogService.List(r.Context(), int32(agentID64), day, day.AddDate(0, 0, 1))
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	server.RespondOK(entries, w, r)
}
