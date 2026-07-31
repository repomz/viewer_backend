package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
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
