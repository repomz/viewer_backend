package httpserver

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/repomz/viewer_backend/internal/app/common/server"
	"github.com/repomz/viewer_backend/internal/app/domain"

	"github.com/repomz/viewer_backend/internal/app/transport/httpmodels"
)

func (h HttpServer) GetAgents(w http.ResponseWriter, r *http.Request) {
	ids, err := h.agentRecordsService.GetAgentIDs(r.Context())
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	server.RespondOK(ids, w, r)
}

// DeleteStudy deletes a agent record by ID
func (h HttpServer) DeleteAllAgentRecords(w http.ResponseWriter, r *http.Request) {
	// Получаем agent ID из пути
	vars := mux.Vars(r)
	agentIDstr := vars["agent_id"]
	if agentIDstr == "" {
		err := errors.New("agent ID is required")
		server.BadRequest("invalid agent ID", err, w, r)
		return
	}

	// Парсим строку в int
	agentID, err := parseAgentID(agentIDstr)
	if err != nil {
		server.BadRequest("invalid agent ID format", err, w, r)
		return
	}

	err = h.agentRecordsService.DeleteAllAgentRecords(r.Context(), agentID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			server.NotFound("record-not-found", err, w, r)
			return
		}
		server.RespondWithError(err, w, r)
		return
	}

	server.RespondOK(map[string]bool{"deleted": true}, w, r)
}

// CreateStudy creates a new agent record
func (h HttpServer) CreateAgentRecord(w http.ResponseWriter, r *http.Request) {
	// Получаем agent record запрос
	var agentRequest httpmodels.AgentRecordRequest
	if err := decodeJSON(w, r, &agentRequest); err != nil {
		server.BadRequest("invalid-json", err, w, r)
		return
	}

	// Валидируем study запрос
	if err := agentRequest.Validate(); err != nil {
		server.BadRequest("invalid-request", err, w, r)
		return
	}

	agentRecord, err := toDomainAgentRecord(agentRequest)
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}

	err = h.agentRecordsService.CreateAgentRecord(r.Context(), agentRecord)
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}

	server.RespondCreated(map[string]bool{"inserted": true}, w, r)
}

func (h HttpServer) GetAgentRecordsByAgentID(w http.ResponseWriter, r *http.Request) {
	// Получаем agent ID из пути
	query := r.URL.Query()
	agentIDstr := query.Get("agent_id")

	if agentIDstr == "" {
		err := errors.New("agent ID is required")
		server.BadRequest("invalid agent ID", err, w, r)
		return
	}

	// Парсим строку в int
	agentID, err := parseAgentID(agentIDstr)
	if err != nil {
		server.BadRequest("invalid agent ID format", err, w, r)
		return
	}

	limit := agentRecordLimit(query.Get("limit"))
	records, err := h.agentRecordsService.GetAgentRecordsByAgentID(r.Context(), agentID, limit)
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}

	response := make([]string, 0, len(records))
	for _, record := range records {
		response = append(response, record.Format(time.RFC3339))
	}

	server.RespondOK(response, w, r)
}

func (h HttpServer) GetAgentRecordsByAgentIDandStatus(w http.ResponseWriter, r *http.Request) {
	// Получаем agent ID из пути
	query := r.URL.Query()
	agentIDstr := query.Get("agent_id")

	if agentIDstr == "" {
		err := errors.New("agent ID is required")
		server.BadRequest("invalid agent ID", err, w, r)
		return
	}

	// Парсим строку в int
	agentID, err := parseAgentID(agentIDstr)
	if err != nil {
		server.BadRequest("invalid agent ID format", err, w, r)
		return
	}

	agentStatus := strings.ToLower(strings.TrimSpace(query.Get("status")))
	if agentStatus == "" {
		err := errors.New("status is required")
		server.BadRequest("invalid agent status", err, w, r)
		return
	}

	limit := agentRecordLimit(query.Get("limit"))
	records, err := h.agentRecordsService.GetAgentRecordsByAgentIDandStatus(r.Context(), agentID, agentStatus, limit)
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}

	response := make([]string, 0, len(records))
	for _, record := range records {
		response = append(response, record.Format(time.RFC3339))
	}

	server.RespondOK(response, w, r)
}

func agentRecordLimit(rawLimit string) int32 {
	const defaultLimit = 1000
	const maximumLimit = 1000
	limit, err := strconv.Atoi(rawLimit)
	if err != nil || limit < 1 {
		return defaultLimit
	}
	if limit > maximumLimit {
		limit = maximumLimit
	}
	return int32(limit)
}
