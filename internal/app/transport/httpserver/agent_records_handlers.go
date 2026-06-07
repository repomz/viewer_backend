package httpserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/repomz/viewer_backend/internal/app/common/server"
	"github.com/repomz/viewer_backend/internal/app/domain"

	"github.com/repomz/viewer_backend/internal/app/transport/httpmodels"
)

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
	agentID, err := strconv.Atoi(agentIDstr)
	if err != nil {
		server.BadRequest("invalid agent ID format", err, w, r)
		return
	}

	err = h.agentRecordsService.DeleteAllAgentRecords(r.Context(), int32(agentID))
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
	if err := json.NewDecoder(r.Body).Decode(&agentRequest); err != nil {
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

	server.RespondOK(map[string]bool{"agent record inserted": true}, w, r)
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
	agentID, err := strconv.Atoi(agentIDstr)
	if err != nil {
		server.BadRequest("invalid agent ID format", err, w, r)
		return
	}

	records, err := h.agentRecordsService.GetAgentRecordsByAgentID(r.Context(), int32(agentID))
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}

	response := make([]string, 0, len(records))
	for _, record := range records {
		response = append(response, record.Format("15:04"))
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
	agentID, err := strconv.Atoi(agentIDstr)
	if err != nil {
		server.BadRequest("invalid agent ID format", err, w, r)
		return
	}

	agentStatus := query.Get("status")
	if agentStatus == "" {
		err := errors.New("status is required")
		server.BadRequest("invalid agent status", err, w, r)
		return
	}

	records, err := h.agentRecordsService.GetAgentRecordsByAgentIDandStatus(r.Context(), int32(agentID), agentStatus)
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}

	response := make([]string, 0, len(records))
	for _, record := range records {
		response = append(response, record.Format("15:04"))
	}

	server.RespondOK(response, w, r)
}
