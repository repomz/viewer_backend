package httpserver

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/repomz/viewer_backend/internal/app/domain"
)

type studyServiceStub struct {
	created     domain.Study
	existing    domain.Study
	studies     []domain.Study
	limit       int
	offset      int
	getAllCalls int
}

func (s *studyServiceStub) GetAllStudies(_ context.Context, limit, offset int) ([]domain.Study, error) {
	s.getAllCalls++
	s.limit, s.offset = limit, offset
	return s.studies, nil
}

func (s *studyServiceStub) GetProtocolStudiesSince(_ context.Context, _ time.Time, limit, offset int) ([]domain.Study, error) {
	s.limit, s.offset = limit, offset
	return s.studies, nil
}

func (s *studyServiceStub) GetStudiesByFilter(context.Context, domain.StudyFilter) ([]domain.Study, error) {
	return nil, nil
}

func (s *studyServiceStub) GetStudyByID(context.Context, uuid.UUID) (domain.Study, error) {
	return domain.Study{}, nil
}

func (s *studyServiceStub) GetStudyByStudyIDAndType(
	context.Context,
	string,
	string,
) (domain.Study, error) {
	if s.existing.StudyID() != "" {
		return s.existing, nil
	}
	return domain.Study{}, domain.ErrNotFound
}

func (s *studyServiceStub) GetStudyByPatient(context.Context, domain.PatientFilter) (domain.Study, error) {
	return domain.Study{}, nil
}

func (s *studyServiceStub) CreateStudy(_ context.Context, study domain.Study) (domain.Study, error) {
	s.created = study
	return study, nil
}

func (s *studyServiceStub) UpdateStudyDicomLink(context.Context, domain.Study) (domain.Study, error) {
	return domain.Study{}, nil
}

func (s *studyServiceStub) DeleteStudy(context.Context, uuid.UUID) error {
	return nil
}

func (s *studyServiceStub) DeleteAllStudies(context.Context) error {
	return nil
}

func TestGetAllStudiesAppliesPagination(t *testing.T) {
	service := &studyServiceStub{}
	handler := NewHttpServer(service, nil)
	request := httptest.NewRequest(http.MethodGet, "/studies?scope=all&page=3&page_size=25", nil)
	recorder := httptest.NewRecorder()

	handler.GetAllStudies(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if service.limit != 25 || service.offset != 50 {
		t.Fatalf("limit/offset = %d/%d, want 25/50", service.limit, service.offset)
	}
}

func TestSuggestProtocolStudiesSearchesPatientAndExcludesImaging(t *testing.T) {
	currentDate := time.Date(time.Now().Year(), time.June, 10, 10, 0, 0, 0, time.Local)
	service := &studyServiceStub{studies: []domain.Study{
		domain.ResponseToDBStudy(domain.DBStudyData{ID: uuid.New(), Patient: "Петров Иван Викторович", StudyType: "каг", TimeBeginning: currentDate}),
		domain.ResponseToDBStudy(domain.DBStudyData{ID: uuid.New(), Patient: "Петров Иван Викторович", StudyType: "XA", TimeBeginning: currentDate}),
		domain.ResponseToDBStudy(domain.DBStudyData{ID: uuid.New(), Patient: "Иван Петрович Петров", StudyType: "каг", TimeBeginning: currentDate}),
		domain.ResponseToDBStudy(domain.DBStudyData{ID: uuid.New(), Patient: "Иванов Иван", StudyType: "цаг", TimeBeginning: currentDate}),
		domain.ResponseToDBStudy(domain.DBStudyData{ID: uuid.New(), Patient: "Петров Петр Петрович", StudyType: "каг", TimeBeginning: currentDate.AddDate(-1, 0, 0)}),
	}}
	handler := NewHttpServer(service, nil)
	request := httptest.NewRequest(http.MethodGet, "/studies/suggest?patient=петр", nil)
	recorder := httptest.NewRecorder()

	handler.SuggestProtocolStudies(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	if count := bytes.Count(recorder.Body.Bytes(), []byte(`"patient"`)); count != 1 {
		t.Fatalf("suggestions = %d, want one protocol: %s", count, recorder.Body.String())
	}
}

func TestSuggestProtocolStudiesMatchesPatientNamePrefixesAndCompactInitials(t *testing.T) {
	currentDate := time.Date(time.Now().Year(), time.June, 10, 10, 0, 0, 0, time.Local)
	for _, query := range []string{"пет", "Петров ИВ", "петров и. в.", "Петров Иван Вик"} {
		service := &studyServiceStub{studies: []domain.Study{
			domain.ResponseToDBStudy(domain.DBStudyData{ID: uuid.New(), Patient: "Петров Иван Викторович", StudyType: "каг", TimeBeginning: currentDate}),
			domain.ResponseToDBStudy(domain.DBStudyData{ID: uuid.New(), Patient: "Иван Петрович Петров", StudyType: "каг", TimeBeginning: currentDate}),
		}}
		handler := NewHttpServer(service, nil)
		request := httptest.NewRequest(http.MethodGet, "/studies/suggest?patient="+url.QueryEscape(query), nil)
		recorder := httptest.NewRecorder()

		handler.SuggestProtocolStudies(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("query %q: status = %d, body=%s", query, recorder.Code, recorder.Body.String())
		}
		if count := bytes.Count(recorder.Body.Bytes(), []byte(`"patient"`)); count != 1 {
			t.Fatalf("query %q: suggestions = %d, want one prefix match: %s", query, count, recorder.Body.String())
		}
	}
}

func TestCreateStudyPreservesStudyTypeAndTime(t *testing.T) {
	service := &studyServiceStub{}
	handler := NewHttpServer(service, nil)
	body := []byte(`{
		"study_id":"STUDY-1",
		"patient":"Иванов И.И.",
		"age":50,
		"department":"кардиология",
		"name_operation":"КАГ",
		"study_type":"каг",
		"descr_operation":"плановая",
		"recommendation":"Стентирование в плановом порядке",
		"time_beginning":"2026-07-17T10:00:00Z",
		"time_duration":30,
		"surgeon":"идрисов",
		"dicom_link":"https://pacs/studies/1"
	}`)
	request := httptest.NewRequest(http.MethodPost, "/studies", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	handler.CreateStudy(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusCreated, recorder.Body.String())
	}
	if service.created.StudyType() != "каг" {
		t.Fatalf("StudyType = %q, want каг", service.created.StudyType())
	}
	if service.created.TimeBeginning().Time.IsZero() {
		t.Fatal("TimeBeginning was lost during JSON/domain mapping")
	}
	if service.created.Recommendation() != "Стентирование в плановом порядке" {
		t.Fatalf("Recommendation = %q", service.created.Recommendation())
	}
}

func TestCreateStudyRejectsUnknownJSONField(t *testing.T) {
	service := &studyServiceStub{}
	handler := NewHttpServer(service, nil)
	request := httptest.NewRequest(http.MethodPost, "/studies", bytes.NewBufferString(`{"unknown":true}`))
	recorder := httptest.NewRecorder()

	handler.CreateStudy(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}
