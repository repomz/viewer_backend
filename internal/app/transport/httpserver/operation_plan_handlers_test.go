package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/repomz/viewer_backend/internal/app/domain"
)

func TestOperationPlanRoundTrip(t *testing.T) {
	t.Setenv("REPORTS_DIR", t.TempDir())
	handler := NewHttpServer(nil, nil)
	router := mux.NewRouter()
	router.HandleFunc("/operation-plan", handler.GetOperationPlan).Methods(http.MethodGet)
	router.HandleFunc(
		"/operation-plan/{date}",
		handler.PutOperationPlanDay,
	).Methods(http.MethodPut)

	saveRequest := httptest.NewRequest(
		http.MethodPut,
		"/operation-plan/2026-07-28",
		strings.NewReader(`{"entries":[{
			"patient":"Петров",
			"department":"кардио 2",
			"operation":"каг + стент"
		}]}`),
	)
	saveRecorder := httptest.NewRecorder()
	router.ServeHTTP(saveRecorder, saveRequest)
	if saveRecorder.Code != http.StatusOK {
		t.Fatalf("save status=%d body=%s", saveRecorder.Code, saveRecorder.Body.String())
	}

	getRequest := httptest.NewRequest(
		http.MethodGet,
		"/operation-plan?week_start=2026-07-27",
		nil,
	)
	getRecorder := httptest.NewRecorder()
	router.ServeHTTP(getRecorder, getRequest)
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("get status=%d body=%s", getRecorder.Code, getRecorder.Body.String())
	}
	var plan operationPlanResponse
	if err := json.NewDecoder(getRecorder.Body).Decode(&plan); err != nil {
		t.Fatalf("decode plan: %v", err)
	}
	if len(plan.Days) != 5 ||
		plan.Days[0].Entries == nil ||
		len(plan.Days[1].Entries) != 1 ||
		plan.Days[1].Entries[0].Patient != "Петров" {
		t.Fatalf("unexpected plan: %#v", plan)
	}
}

func TestPlanPatientMatchesFullNameOrInitials(t *testing.T) {
	if !planPatientMatches("Иванов", "Иванов Иван Иванович") {
		t.Fatal("exact surname must match a full protocol name")
	}
	if !planPatientMatches("Иванов И И", "Иванов Иван Иванович") {
		t.Fatal("surname with initials must match full protocol name")
	}
	if !planPatientMatches("Иванов Иван Иванович", "Иванов Иван Иванович") {
		t.Fatal("full name must match exactly")
	}
	if planPatientMatches("Иванов Иван", "Иванов Иван Иванович") {
		t.Fatal("partial full name must not match")
	}
	if planPatientMatches("Иван", "Иванов Иван Иванович") {
		t.Fatal("partial surname must not match")
	}
}

func TestPlanProtocolsMatchesSurnameOnlyOnPlanDate(t *testing.T) {
	study := domain.ResponseToDBStudy(domain.DBStudyData{
		ID: uuid.New(), StudyID: uuid.NewString(), Patient: "Ларькин Ю.П.",
		StudyType: "бап_кор", NameOperation: "БАП коронарной артерии",
		TimeBeginning: time.Date(2026, 8, 7, 5, 30, 0, 0, time.Local),
	})
	previous, completed := planProtocols(
		operationPlanEntry{Patient: "Ларькин"}, []domain.Study{study},
		time.Date(2026, 8, 7, 0, 0, 0, 0, time.Local),
	)
	if len(previous) != 0 || completed == nil || completed.Patient != "Ларькин Ю.П." {
		t.Fatalf("unexpected protocols: previous=%#v completed=%#v", previous, completed)
	}
}

func TestPlanProtocolsReturnsThreePreviousAndCurrentCompletion(t *testing.T) {
	makeStudy := func(patient string, date time.Time) domain.Study {
		return domain.ResponseToDBStudy(domain.DBStudyData{
			ID: uuid.New(), StudyID: uuid.NewString(), Patient: patient,
			StudyType: "каг", NameOperation: "КАГ", TimeBeginning: date,
		})
	}
	studies := []domain.Study{
		makeStudy("Иванов Иван Иванович", time.Date(2026, 1, 1, 10, 0, 0, 0, time.Local)),
		makeStudy("Иванов Иван Иванович", time.Date(2026, 2, 1, 10, 0, 0, 0, time.Local)),
		makeStudy("Иванов Иван Иванович", time.Date(2026, 7, 1, 10, 0, 0, 0, time.Local)),
		makeStudy("Иванов Иван Иванович", time.Date(2026, 7, 20, 10, 0, 0, 0, time.Local)),
		makeStudy("Иванов Иван Иванович", time.Date(2026, 7, 21, 10, 0, 0, 0, time.Local)),
	}
	previous, completed := planProtocols(
		operationPlanEntry{Patient: "Иванов Иван Иванович"}, studies,
		time.Date(2026, 7, 21, 0, 0, 0, 0, time.Local),
	)
	if len(previous) != 3 || completed == nil || completed.TimeBeginning.Day() != 21 {
		t.Fatalf("unexpected protocols: previous=%#v completed=%#v", previous, completed)
	}
}
