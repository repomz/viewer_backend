package httpserver

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/repomz/viewer_backend/internal/app/common/server"
	"github.com/repomz/viewer_backend/internal/app/domain"

	"github.com/repomz/viewer_backend/internal/app/transport/httpmodels"
)

func (h HttpServer) SuggestProtocolStudies(w http.ResponseWriter, r *http.Request) {
	query := normalizePatientSearch(r.URL.Query().Get("patient"))
	if len([]rune(query)) < 2 {
		server.BadRequest("invalid-patient-query", errors.New("patient must contain at least two characters"), w, r)
		return
	}
	limit := 20
	if rawLimit := r.URL.Query().Get("limit"); rawLimit != "" {
		value, err := strconv.Atoi(rawLimit)
		if err != nil || value < 1 || value > 50 {
			server.BadRequest("invalid-limit", errors.New("limit must be between 1 and 50"), w, r)
			return
		}
		limit = value
	}
	result := make([]httpmodels.StudyResponse, 0, limit)
	currentYear := time.Now().In(time.Local).Year()
	const pageSize = 1000
	for offset := 0; len(result) < limit; offset += pageSize {
		studies, err := h.studyService.GetAllStudies(r.Context(), pageSize, offset)
		if err != nil {
			server.RespondWithError(err, w, r)
			return
		}
		for _, study := range studies {
			modality := strings.ToLower(strings.TrimSpace(study.StudyType()))
			patient := normalizePatientSearch(study.Patient())
			beginning := study.TimeBeginning()
			if modality != "xa" && modality != "ct" &&
				beginning.Valid && beginning.Time.In(time.Local).Year() == currentYear &&
				patientSearchPrefixMatches(patient, query) {
				result = append(result, toResponseStudy(study))
				if len(result) == limit {
					break
				}
			}
		}
		if len(studies) < pageSize {
			break
		}
	}
	server.RespondOK(result, w, r)
}

func normalizePatientSearch(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "ё", "е")
	value = strings.NewReplacer(".", "", ",", " ", ";", " ").Replace(value)
	return strings.Join(strings.Fields(value), " ")
}

func patientSearchPrefixMatches(patient, query string) bool {
	patientParts := strings.Fields(patient)
	queryParts := strings.Fields(query)
	if len(patientParts) == 0 || len(queryParts) == 0 || !strings.HasPrefix(patientParts[0], queryParts[0]) {
		return false
	}
	if len(queryParts) == 1 {
		return true
	}

	patientIndex := 1
	for queryIndex := 1; queryIndex < len(queryParts); queryIndex++ {
		queryPart := queryParts[queryIndex]
		queryRunes := []rune(queryPart)
		remainingPatientParts := len(patientParts) - patientIndex
		if len(queryParts) == 2 && len(queryRunes) > 1 && len(queryRunes) <= remainingPatientParts {
			allInitialsMatch := true
			for index, initial := range queryRunes {
				patientRunes := []rune(patientParts[patientIndex+index])
				if len(patientRunes) == 0 || patientRunes[0] != initial {
					allInitialsMatch = false
					break
				}
			}
			if allInitialsMatch {
				patientIndex += len(queryRunes)
				continue
			}
		}
		if patientIndex >= len(patientParts) || !strings.HasPrefix(patientParts[patientIndex], queryPart) {
			return false
		}
		patientIndex++
	}
	return true
}

func (h HttpServer) GetAllStudies(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	page := 1
	if rawPage := query.Get("page"); rawPage != "" {
		parsedPage, err := strconv.Atoi(rawPage)
		if err != nil || parsedPage < 1 {
			server.BadRequest("invalid-page", errors.New("page must be a positive integer"), w, r)
			return
		}
		page = parsedPage
	}

	pageSize := 10
	if rawPageSize := query.Get("page_size"); rawPageSize != "" {
		parsedPageSize, err := strconv.Atoi(rawPageSize)
		if err != nil || parsedPageSize < 1 || parsedPageSize > 100 {
			server.BadRequest("invalid-page-size", errors.New("page_size must be between 1 and 100"), w, r)
			return
		}
		pageSize = parsedPageSize
	}
	offset := (page - 1) * pageSize

	var studies []domain.Study
	var err error
	if query.Get("scope") == "all" {
		studies, err = h.studyService.GetAllStudies(r.Context(), pageSize, offset)
	} else {
		start := monday(time.Now().In(time.Local)).AddDate(0, 0, -2)
		studies, err = h.studyService.GetProtocolStudiesSince(r.Context(), start, pageSize, offset)
	}
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
	if err := decodeJSON(w, r, &studyRequest); err != nil {
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
	invalidateStudyAnalysisResponseCaches()

	response := toResponseStudy(insertedStudy)

	server.RespondCreated(response, w, r)
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
	if err := decodeJSON(w, r, &studyRequest); err != nil {
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
	invalidateStudyAnalysisResponseCaches()

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
	invalidateStudyAnalysisResponseCaches()

	server.RespondOK(map[string]bool{"deleted": true}, w, r)
}
