package httpserver

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/repomz/viewer_backend/internal/app/domain"
)

type studyServiceStub struct {
	created domain.Study
	limit   int
	offset  int
}

func (s *studyServiceStub) GetAllStudies(_ context.Context, limit, offset int) ([]domain.Study, error) {
	s.limit, s.offset = limit, offset
	return []domain.Study{}, nil
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
	request := httptest.NewRequest(http.MethodGet, "/studies?page=3&page_size=25", nil)
	recorder := httptest.NewRecorder()

	handler.GetAllStudies(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if service.limit != 25 || service.offset != 50 {
		t.Fatalf("limit/offset = %d/%d, want 25/50", service.limit, service.offset)
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
