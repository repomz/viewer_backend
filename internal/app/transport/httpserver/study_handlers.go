package httpserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/repomz/viewer_backend/internal/app/common/server"
	"github.com/repomz/viewer_backend/internal/app/domain"
)

func (h HttpServer) GetStudies(w http.ResponseWriter, r *http.Request) {
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

	studies, err := h.studyService.GetStudies(r.Context(), categoryIDs, limit, offset)
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}

	response := make([]StudyResponse, 0, len(studies))
	for _, study := range studies {
		response = append(response, toResponseStudy(study))
	}

	server.RespondOK(response, w, r)
}

// GetBook returns a book by ID
func (h HttpServer) GetStudy(w http.ResponseWriter, r *http.Request) {
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

	study, err := h.studyService.GetStudy(r.Context(), studyID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			server.NotFound("book-not-found", err, w, r)
			return
		}
		server.RespondWithError(err, w, r)
		return
	}

	response := toResponseStudy(study)

	server.RespondOK(response, w, r)
}

// CreateBook creates a new book
func (h HttpServer) CreateStudy(w http.ResponseWriter, r *http.Request) {
	// Получаем study запрос
	var studyRequest StudyRequest
	if err := json.NewDecoder(r.Body).Decode(&studyRequest); err != nil {
		server.BadRequest("invalid-json", err, w, r)
		return
	}

	// Валидируем study запрос
	if err := studyRequest.Validate(); err != nil {
		server.BadRequest("invalid-request", err, w, r)
		return
	}

	study, err := toDomainStudy(studyRequest)
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}

	insertedBook, err := h.studyService.CreateStudy(r.Context(), study)
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}

	response := toResponseStudy(insertedBook)

	server.RespondOK(response, w, r)
}

// UpdateBook updates a book by ID
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
	var studyRequest StudyDicomLinkRequest
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
	_, err = h.studyService.GetStudy(r.Context(), studyID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			server.NotFound("book-not-found", err, w, r)
			return
		}
		server.RespondWithError(err, w, r)
		return
	}

	study, err := domain.NewStudy(domain.NewStudyData{
		ID:        studyID,
		DicomLink: studyRequest.DicomLink,
	})
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}

	updatedStudy, err := h.studyService.UpdateStudyDicomLink(r.Context(), study)
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}

	response := toResponseStudy(updatedStudy)

	server.RespondOK(response, w, r)
}

// DeleteBook deletes a book by ID
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

	_, err = h.studyService.GetStudy(r.Context(), studyID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			server.NotFound("book-not-found", err, w, r)
			return
		}
		server.RespondWithError(err, w, r)
		return
	}

	err = h.studyService.DeleteStudy(r.Context(), studyID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			server.NotFound("book-not-found", err, w, r)
			return
		}
		server.RespondWithError(err, w, r)
		return
	}

	server.RespondOK(map[string]bool{"deleted": true}, w, r)
}
