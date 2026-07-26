package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/repomz/viewer_backend/internal/app/domain"
)

type userRequestServiceStub struct {
	created      domain.NewUserRequest
	claimedAgent int32
	result       domain.UserRequestResult
	request      domain.UserRequest
}

func (s *userRequestServiceStub) Create(_ context.Context, request domain.NewUserRequest) (domain.UserRequest, error) {
	s.created = request
	return s.request, nil
}

func (s *userRequestServiceStub) ClaimNext(_ context.Context, agentID int32) (domain.UserRequest, error) {
	s.claimedAgent = agentID
	return s.request, nil
}

func (s *userRequestServiceStub) RecordResult(_ context.Context, result domain.UserRequestResult) (domain.UserRequest, error) {
	s.result = result
	return s.request, nil
}

func (s *userRequestServiceStub) GetByID(context.Context, uuid.UUID) (domain.UserRequest, error) {
	return s.request, nil
}

func queuedUserRequest() domain.UserRequest {
	return domain.UserRequest{
		ID:           uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		AvailableAt:  time.Now(),
		Status:       domain.UserRequestInProgress,
		UserID:       "operator-1",
		AgentID:      2,
		RequestType:  "execute_command",
		Command:      "get_xa",
		Payload:      json.RawMessage(`{"study_uid":"1.2.3"}`),
		Result:       json.RawMessage(`{}`),
		AttemptCount: 1,
		MaxAttempts:  3,
	}
}

func TestCreateUserRequest(t *testing.T) {
	service := &userRequestServiceStub{request: queuedUserRequest()}
	handler := NewHttpServer(nil, nil, service)
	body := bytes.NewBufferString(`{
		"user_id":"operator-1",
		"agent_id":2,
		"command":"get_xa",
		"payload":{"study_uid":"1.2.3"}
	}`)
	request := httptest.NewRequest(http.MethodPost, "/user_requests", body)
	recorder := httptest.NewRecorder()

	handler.CreateUserRequest(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusCreated, recorder.Body)
	}
	if service.created.AgentID != 2 || service.created.Command != "get_xa" {
		t.Fatalf("created request = %#v", service.created)
	}
}

func TestClaimUserRequestBuildsAgentContract(t *testing.T) {
	service := &userRequestServiceStub{request: queuedUserRequest()}
	handler := NewHttpServer(nil, nil, service)
	request := httptest.NewRequest(http.MethodGet, "/user_requests?agent_id=2", nil)
	recorder := httptest.NewRecorder()

	handler.ClaimUserRequest(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response["study_uid"] != "1.2.3" {
		t.Fatalf("study_uid = %#v", response["study_uid"])
	}
	for _, alias := range []string{"action", "type", "request_type"} {
		if _, exists := response[alias]; exists {
			t.Fatalf("agent response unexpectedly contains %q", alias)
		}
	}
	if response["response_endpoint"] != "/user_requests/11111111-1111-1111-1111-111111111111/result" {
		t.Fatalf("response_endpoint = %#v", response["response_endpoint"])
	}
}

func TestRecordUserRequestResult(t *testing.T) {
	service := &userRequestServiceStub{request: queuedUserRequest()}
	handler := NewHttpServer(nil, nil, service)
	request := httptest.NewRequest(
		http.MethodPost,
		"/user_requests/11111111-1111-1111-1111-111111111111/result",
		bytes.NewBufferString(`{"agent_id":2,"ok":true,"result":{"uploaded":3}}`),
	)
	request = mux.SetURLVars(request, map[string]string{
		"request_id": "11111111-1111-1111-1111-111111111111",
	})
	recorder := httptest.NewRecorder()

	handler.RecordUserRequestResult(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body)
	}
	if !service.result.OK || service.result.AgentID != 2 {
		t.Fatalf("result = %#v", service.result)
	}
}
