package httpserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/repomz/viewer_backend/internal/app/common/server"
	"github.com/repomz/viewer_backend/internal/app/domain"

	"github.com/repomz/viewer_backend/internal/app/transport/httpmodels"
)

func (h HttpServer) GetAllStudies(w http.ResponseWriter, r *http.Request) {
	// filter by category IDs
	queryCategoryIDs := r.URL.Query()["category_id"]
	var categoryIDs []int
	for _, id := range queryCategoryIDs {
		categoryID, err := strconv.Atoi(id)
		if err != nil {
			server.BadRequest("invalid-category-id", err, w, r)
			return
		}
		categoryIDs = append(categoryIDs, categoryID)
	}
	// page
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil {
		page = 1
	}
	var limit, offset int
	if page > 0 {
		limit = 10
		offset = (page - 1) * limit
	}

	studies, err := h.studyService.GetAllStudies(r.Context(), categoryIDs, limit, offset)
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}

	response := make([]httpmodels.StudyResponse, 0, len(studies))
	for _, study := range studies {
		response = append(response, toResponseStudy(study))
	}

	server.RespondOK(response, w, r)
}

func (h HttpServer) GetStudiesByFilter(w http.ResponseWriter, r *http.Request) {
	// filter by category IDs
	query := r.URL.Query()
	filterRequest := httpmodels.StudyFilter{
		Surgeon:   query.Get("surgeon"),
		StudyType: query.Get("study_type"),
	}

	// Парсинг даты (с обработкой ошибки формата)
	if dateStr := query.Get("study_date"); dateStr != "" {
		// Ожидаем формат ISO (например, 2026-05-19)
		parsedDate, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			server.BadRequest("invalid study_date format, use YYYY-MM-DD", err, w, r)
			return
		}
		filterRequest.StudyDate = &parsedDate
	}

	// Нормализуем данные перед валидацией
	filterRequest.Normalize()

	// Валидируем фильтр исследований
	if err := filterRequest.Validate(); err != nil {
		server.BadRequest("validation error", err, w, r)
		return
	}

	filter := toDomainStudyFilter(filterRequest)

	studies, err := h.studyService.GetStudiesByFilter(r.Context(), filter)
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}

	response := make([]httpmodels.StudyResponse, 0, len(studies))
	for _, study := range studies {
		response = append(response, toResponseStudy(study))
	}

	server.RespondOK(response, w, r)
}

// GetStudy returns a Study by ID
func (h HttpServer) GetStudyByID(w http.ResponseWriter, r *http.Request) {
	// Получаем study ID из пути
	vars := mux.Vars(r)
	studyIDstr := vars["study_id"]
	if studyIDstr == "" {
		err := errors.New("study ID is required")
		server.BadRequest("invalid study ID", err, w, r)
		return
	}

	// Парсим строку в UUID
	studyID, err := uuid.Parse(studyIDstr)
	if err != nil {
		server.BadRequest("invalid study ID format", err, w, r)
		return
	}

	study, err := h.studyService.GetStudyByID(r.Context(), studyID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			server.NotFound("Study-not-found", err, w, r)
			return
		}
		server.RespondWithError(err, w, r)
		return
	}

	response := toResponseStudy(study)

	server.RespondOK(response, w, r)
}

// GetStudy returns a Study by patient filter
func (h HttpServer) GetStudyByPatient(w http.ResponseWriter, r *http.Request) {
	// Получаем study ID из пути
	vars := mux.Vars(r)
	rawPatient := vars["patient"]
	// rawPatient := r.URL.Query().Get("patient")
	patientRequest := httpmodels.PatientFilter{
		Patient: rawPatient,
	}
	// Инициализируем фильтр и нормализуем его
	// patientRequest := patientFilter.Normalize()

	// Валидируем
	if err := patientRequest.Validate(); err != nil {
		server.BadRequest("invalid patient request", err, w, r)
		return
	}

	patientFilter := toDomainPatientFilter(patientRequest) // !!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!

	study, err := h.studyService.GetStudyByPatient(r.Context(), patientFilter)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			server.NotFound("Study-not-found", err, w, r)
			return
		}
		server.RespondWithError(err, w, r)
		return
	}

	response := toResponseStudy(study)

	server.RespondOK(response, w, r)
}

// CreateStudy creates a new Study
func (h HttpServer) CreateStudy(w http.ResponseWriter, r *http.Request) {
	// Получаем study запрос
	var studyRequest httpmodels.StudyRequest
	if err := json.NewDecoder(r.Body).Decode(&studyRequest); err != nil {
		server.BadRequest("invalid-json", err, w, r)
		return
	}

	// Валидируем study запрос
	if err := studyRequest.Validate(); err != nil {
		server.BadRequest("invalid-request", err, w, r)
		return
	}

	study := toDomainStudy(studyRequest)
	// !!!!!!!!!!!!!!!!!!!!!!!!!! может нужна валидация маппинга
	// if err != nil {
	// 	server.RespondWithError(err, w, r)
	// 	return
	// }

	insertedStudy, err := h.studyService.CreateStudy(r.Context(), study)
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}

	response := toResponseStudy(insertedStudy)

	server.RespondOK(response, w, r)
}

// UpdateStudy updates a Study by ID
func (h HttpServer) UpdateStudy(w http.ResponseWriter, r *http.Request) {
	// Получаем study ID из пути
	vars := mux.Vars(r)
	studyIDstr := vars["study_id"]
	if studyIDstr == "" {
		err := errors.New("study ID is required")
		server.BadRequest("invalid study ID", err, w, r)
		return
	}

	// Парсим строку в UUID
	studyID, err := uuid.Parse(studyIDstr)
	if err != nil {
		server.BadRequest("invalid study ID format", err, w, r)
		return
	}

	// Получаем study запрос
	var studyRequest httpmodels.StudyDicomLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&studyRequest); err != nil {
		server.BadRequest("invalid-json", err, w, r)
		return
	}

	// Валидируем study запрос
	if err := studyRequest.Validate(); err != nil {
		server.BadRequest("invalid-request", err, w, r)
		return
	}

	// Проверяем наличие study с таким id в базе данных
	_, err = h.studyService.GetStudyByID(r.Context(), studyID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			server.NotFound("Study-not-found", err, w, r)
			return
		}
		server.RespondWithError(err, w, r)
		return
	}

	study := domain.ResponseToDBStudy(domain.DBStudyData{
		ID:        studyID,
		DicomLink: studyRequest.DicomLink,
	})
	// !!!!!!!!!!!!!!!!!!!!!!!!!! может нужна валидация маппинга
	// if err != nil {
	// 	server.RespondWithError(err, w, r)
	// 	return
	// }

	updatedStudy, err := h.studyService.UpdateStudyDicomLink(r.Context(), study)
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}

	response := toResponseStudy(updatedStudy)

	server.RespondOK(response, w, r)
}

// DeleteStudy deletes a Study by ID
func (h HttpServer) DeleteStudy(w http.ResponseWriter, r *http.Request) {
	// Получаем study ID из пути
	vars := mux.Vars(r)
	studyIDstr := vars["study_id"]
	if studyIDstr == "" {
		err := errors.New("study ID is required")
		server.BadRequest("invalid study ID", err, w, r)
		return
	}

	// Парсим строку в UUID
	studyID, err := uuid.Parse(studyIDstr)
	if err != nil {
		server.BadRequest("invalid study ID format", err, w, r)
		return
	}

	_, err = h.studyService.GetStudyByID(r.Context(), studyID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			server.NotFound("Study-not-found", err, w, r)
			return
		}
		server.RespondWithError(err, w, r)
		return
	}

	err = h.studyService.DeleteStudy(r.Context(), studyID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			server.NotFound("Study-not-found", err, w, r)
			return
		}
		server.RespondWithError(err, w, r)
		return
	}

	server.RespondOK(map[string]bool{"deleted": true}, w, r)
}

// DeleteStudy deletes all Studies
func (h HttpServer) DeleteAllStudies(w http.ResponseWriter, r *http.Request) {

	err := h.studyService.DeleteAllStudies(r.Context())
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			server.NotFound("Study-not-found", err, w, r)
			return
		}
		server.RespondWithError(err, w, r)
		return
	}

	server.RespondOK(map[string]bool{"deleted": true}, w, r)
}
