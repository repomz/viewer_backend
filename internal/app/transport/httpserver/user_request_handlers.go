package httpserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/repomz/viewer_backend/internal/app/common/server"
	"github.com/repomz/viewer_backend/internal/app/domain"
	"github.com/repomz/viewer_backend/internal/app/transport/httpmodels"
)

func (h HttpServer) ListUserRequests(w http.ResponseWriter, r *http.Request) {
	agentID, err := parseAgentID(r.URL.Query().Get("agent_id"))
	if err != nil {
		server.BadRequest("invalid-agent-id", err, w, r)
		return
	}
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	limit := int32(100)
	if raw := r.URL.Query().Get("limit"); raw != "" {
		value, parseErr := strconv.Atoi(raw)
		if parseErr != nil {
			server.BadRequest("invalid-limit", parseErr, w, r)
			return
		}
		limit = int32(value)
	}
	requests, err := h.userRequestService.List(r.Context(), userID, agentID, limit)
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	response := make([]httpmodels.UserRequestResponse, 0, len(requests))
	for _, request := range requests {
		response = append(response, toUserRequestResponse(request))
	}
	server.RespondOK(response, w, r)
}

func (h HttpServer) DeleteUserRequest(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(mux.Vars(r)["request_id"])
	if err != nil {
		server.BadRequest("invalid-request-id", err, w, r)
		return
	}
	if err := h.userRequestService.Delete(
		r.Context(), id, strings.TrimSpace(r.URL.Query().Get("user_id")),
	); err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	server.RespondOK(map[string]bool{"deleted": true}, w, r)
}

func (h HttpServer) DeleteAllUserRequests(w http.ResponseWriter, r *http.Request) {
	agentID, err := parseAgentID(r.URL.Query().Get("agent_id"))
	if err != nil {
		server.BadRequest("invalid-agent-id", err, w, r)
		return
	}
	if err := h.userRequestService.DeleteAll(
		r.Context(), strings.TrimSpace(r.URL.Query().Get("user_id")), agentID,
	); err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	server.RespondOK(map[string]bool{"deleted": true}, w, r)
}

func (h HttpServer) CreateUserRequest(w http.ResponseWriter, r *http.Request) {
	var request httpmodels.UserRequestCreateRequest
	if err := decodeJSON(w, r, &request); err != nil {
		server.BadRequest("invalid-json", err, w, r)
		return
	}
	if err := request.Validate(); err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	payload, err := json.Marshal(request.Payload)
	if err != nil {
		server.BadRequest("invalid-payload", err, w, r)
		return
	}

	created, err := h.userRequestService.Create(r.Context(), domain.NewUserRequest{
		UserID:      request.UserID,
		AgentID:     request.AgentID,
		RequestType: request.RequestType,
		Command:     request.Command,
		Payload:     payload,
		MaxAttempts: request.MaxAttempts,
	})
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	server.RespondCreated(toUserRequestResponse(created), w, r)
}

func (h HttpServer) ClaimUserRequest(w http.ResponseWriter, r *http.Request) {
	agentID, err := parseAgentID(r.URL.Query().Get("agent_id"))
	if err != nil {
		server.BadRequest("invalid-agent-id", err, w, r)
		return
	}
	request, err := h.userRequestService.ClaimNext(r.Context(), agentID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			server.RespondOK(map[string]any{}, w, r)
			return
		}
		server.RespondWithError(err, w, r)
		return
	}
	server.RespondOK(toAgentCommandResponse(request), w, r)
}

func (h HttpServer) RecordUserRequestResult(w http.ResponseWriter, r *http.Request) {
	requestID, err := uuid.Parse(mux.Vars(r)["request_id"])
	if err != nil {
		server.BadRequest("invalid-request-id", err, w, r)
		return
	}
	var request httpmodels.UserRequestResultRequest
	if err := decodeJSON(w, r, &request); err != nil {
		server.BadRequest("invalid-json", err, w, r)
		return
	}
	if err := request.Validate(); err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	result, err := json.Marshal(request.Result)
	if err != nil {
		server.BadRequest("invalid-result", err, w, r)
		return
	}
	updated, err := h.userRequestService.RecordResult(r.Context(), domain.UserRequestResult{
		ID:        requestID,
		AgentID:   request.AgentID,
		OK:        request.OK,
		Retryable: request.Retryable,
		Result:    result,
		Error:     request.Errors,
	})
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	server.RespondOK(toUserRequestResponse(updated), w, r)
}

func (h HttpServer) GetUserRequest(w http.ResponseWriter, r *http.Request) {
	requestID, err := uuid.Parse(mux.Vars(r)["request_id"])
	if err != nil {
		server.BadRequest("invalid-request-id", err, w, r)
		return
	}
	request, err := h.userRequestService.GetByID(r.Context(), requestID)
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	server.RespondOK(toUserRequestResponse(request), w, r)
}

func toUserRequestResponse(request domain.UserRequest) httpmodels.UserRequestResponse {
	return httpmodels.UserRequestResponse{
		ID:             request.ID,
		CreatedAt:      request.CreatedAt,
		UpdatedAt:      request.UpdatedAt,
		AvailableAt:    request.AvailableAt,
		LeaseExpiresAt: request.LeaseExpiresAt,
		CompletedAt:    request.CompletedAt,
		Status:         request.Status,
		UserID:         request.UserID,
		AgentID:        request.AgentID,
		RequestType:    request.RequestType,
		Command:        request.Command,
		Payload:        request.Payload,
		Result:         request.Result,
		Errors:         request.ErrorLog,
		AttemptCount:   request.AttemptCount,
		MaxAttempts:    request.MaxAttempts,
	}
}

func toAgentCommandResponse(request domain.UserRequest) map[string]any {
	response := map[string]any{}
	_ = json.Unmarshal(request.Payload, &response)
	for _, protected := range []string{
		"id", "request_id", "agent_id", "command", "request_type",
		"attempt_count", "max_attempts", "response_endpoint",
	} {
		delete(response, protected)
	}
	response["id"] = request.ID.String()
	response["request_id"] = request.ID.String()
	response["agent_id"] = request.AgentID
	response["command"] = request.Command
	response["attempt_count"] = request.AttemptCount
	response["max_attempts"] = request.MaxAttempts
	response["response_endpoint"] = strings.ReplaceAll(
		"/user_requests/{request_id}/result",
		"{request_id}",
		request.ID.String(),
	)
	return response
}
