package httpserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateCTStudyImportsEveryDicomBeforePersisting(t *testing.T) {
	dicomSource := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/dicom")
		_, _ = w.Write([]byte("dicom"))
	}))
	defer dicomSource.Close()

	imported := 0
	remotePACS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		imported++
		if r.Header.Get("Content-Type") != "application/dicom" {
			t.Fatalf("Content-Type = %q", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer remotePACS.Close()
	t.Setenv("REMOTE_PACS_URL", remotePACS.URL)

	service := &studyServiceStub{}
	handler := NewHttpServer(service, nil, nil)
	body, _ := json.Marshal(map[string]any{
		"study_uid":   "1.2.3",
		"patient":     "Иванов И.И.",
		"age":         55,
		"study_date":  "20260726",
		"study_time":  "101500",
		"description": "КТ органов грудной клетки",
		"modality":    "CT",
		"dicom_link":  "s3://bucket/1.2.3",
		"dicom_files": []map[string]any{
			{"name": "1.dcm", "size": 5, "url": dicomSource.URL + "/1"},
			{"name": "2.dcm", "size": 5, "url": dicomSource.URL + "/2"},
		},
	})
	request := httptest.NewRequest(http.MethodPost, "/ct_studies", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	handler.CreateCTStudy(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d; body=%s", recorder.Code, recorder.Body)
	}
	if imported != 2 {
		t.Fatalf("imported = %d, want 2", imported)
	}
	dicomLink := service.created.DicomLink()
	if service.created.StudyID() != "1.2.3" ||
		!dicomLink.Valid ||
		dicomLink.String != "s3://bucket/1.2.3" {
		t.Fatalf("created study = %#v", service.created)
	}
}

func TestCreateModalityStudyRequiresRemotePACSConfiguration(t *testing.T) {
	t.Setenv("REMOTE_PACS_URL", "")
	service := &studyServiceStub{}
	handler := NewHttpServer(service, nil, nil)
	body := bytes.NewBufferString(`{
		"study_uid":"1.2.3",
		"patient":"Иванов И.И.",
		"study_date":"20260726",
		"modality":"XA",
		"dicom_link":"s3://bucket/1.2.3",
		"dicom_files":[{"name":"1.dcm","size":5,"url":"https://example/1"}]
	}`)
	request := httptest.NewRequest(http.MethodPost, "/xa_studies", body)
	recorder := httptest.NewRecorder()

	handler.CreateXAStudy(recorder, request)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadGateway)
	}
}

func TestCreateModalityStudyRejectsTruncatedDicomBeforeRemoteImport(t *testing.T) {
	dicomSource := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("short"))
	}))
	defer dicomSource.Close()

	imported := 0
	remotePACS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		imported++
		w.WriteHeader(http.StatusOK)
	}))
	defer remotePACS.Close()
	t.Setenv("REMOTE_PACS_URL", remotePACS.URL)

	service := &studyServiceStub{}
	handler := NewHttpServer(service, nil, nil)
	body, _ := json.Marshal(map[string]any{
		"study_uid":  "1.2.3",
		"patient":    "Иванов И.И.",
		"study_date": "20260726",
		"modality":   "CT",
		"dicom_link": "s3://bucket/1.2.3",
		"dicom_files": []map[string]any{
			{"name": "1.dcm", "size": 6, "url": dicomSource.URL + "/1"},
		},
	})
	request := httptest.NewRequest(http.MethodPost, "/ct_studies", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	handler.CreateCTStudy(recorder, request)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadGateway, recorder.Body)
	}
	if imported != 0 {
		t.Fatalf("remote imports = %d, want 0", imported)
	}
	if service.created.StudyID() != "" {
		t.Fatalf("study was persisted after truncated import: %#v", service.created)
	}
}

func TestReportsAreStoredAndReturned(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("REPORTS_DIR", directory)
	handler := NewHttpServer(nil, nil, nil)
	request := httptest.NewRequest(
		http.MethodPost,
		"/reports",
		bytes.NewBufferString(`{
			"agent_id":2,
			"generated_at":"2026-07-26T08:00:00Z",
			"report":{"planned_count":3}
		}`),
	)
	recorder := httptest.NewRecorder()
	handler.CreateReport(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d; body=%s", recorder.Code, recorder.Body)
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 1 {
		t.Fatalf("reports = %v, err=%v", entries, err)
	}
	if filepath.Ext(entries[0].Name()) != ".json" {
		t.Fatalf("filename = %q", entries[0].Name())
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/reports", nil)
	listRecorder := httptest.NewRecorder()
	handler.GetReports(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK ||
		!bytes.Contains(listRecorder.Body.Bytes(), []byte(`"planned_count":3`)) ||
		!bytes.Contains(listRecorder.Body.Bytes(), []byte(`"filename":`)) {
		t.Fatalf("status = %d; body=%s", listRecorder.Code, listRecorder.Body)
	}
}

func TestReportRejectsInvalidGeneratedAt(t *testing.T) {
	handler := NewHttpServer(nil, nil, nil)
	request := httptest.NewRequest(
		http.MethodPost,
		"/reports",
		bytes.NewBufferString(`{
			"agent_id":2,
			"generated_at":"not-a-date",
			"report":{"planned_count":3}
		}`),
	)
	recorder := httptest.NewRecorder()

	handler.CreateReport(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}
