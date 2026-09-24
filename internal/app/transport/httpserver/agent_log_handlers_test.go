package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/repomz/viewer_backend/internal/app/domain"
)

type agentLogServiceStub struct {
	stored domain.AgentLog
	listed []domain.AgentLog
}

func (s *agentLogServiceStub) Upsert(_ context.Context, entry domain.AgentLog) error {
	s.stored = entry
	return nil
}

func (s *agentLogServiceStub) List(_ context.Context, _ int32, _, _ time.Time) ([]domain.AgentLog, error) {
	return s.listed, nil
}

func TestCreateAgentLogStoresHourlySlice(t *testing.T) {
	service := &agentLogServiceStub{}
	handler := HttpServer{}
	handler.SetAgentLogService(service)
	body := bytes.NewBufferString(`{
		"agent_id":2,
		"period_start":"2026-09-23T09:00:00+07:00",
		"period_end":"2026-09-23T10:00:00+07:00",
		"content":"first line\nsecond line"
	}`)
	request := httptest.NewRequest(http.MethodPost, "/agent_logs", body)
	recorder := httptest.NewRecorder()

	handler.CreateAgentLog(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusCreated, recorder.Body)
	}
	if service.stored.AgentID != 2 || service.stored.Content != "first line\nsecond line" {
		t.Fatalf("stored log = %#v", service.stored)
	}
}

func TestGetAgentLogsReturnsSelectedDay(t *testing.T) {
	location, err := time.LoadLocation("Asia/Tomsk")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().In(location)
	day := time.Date(now.Year(), now.Month(), now.Day(), 9, 0, 0, 0, location)
	service := &agentLogServiceStub{listed: []domain.AgentLog{{
		AgentID: 1, PeriodStart: day, PeriodEnd: day.Add(time.Hour), Content: "ready",
	}}}
	handler := HttpServer{}
	handler.SetAgentLogService(service)
	request := httptest.NewRequest(
		http.MethodGet,
		"/agent_logs?agent_id=1&date="+day.Format("2006-01-02"),
		nil,
	)
	recorder := httptest.NewRecorder()

	handler.GetAgentLogs(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body)
	}
	var response []domain.AgentLog
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response) != 1 || response[0].Content != "ready" {
		t.Fatalf("response = %#v", response)
	}
}
