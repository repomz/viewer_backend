package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAgentConfigurationDoesNotStoreSecrets(t *testing.T) {
	value, err := safeAgentConfiguration(json.RawMessage(`{"version":"0.2.8","password":"secret","study_polling":{"state":true,"interval_min":5,"operations_dir":["operations"],"token":"secret"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(value), "secret") || !strings.Contains(string(value), "operations") {
		t.Fatalf("unexpected sanitized snapshot: %s", value)
	}
}

func TestAgentConfigurationRequiresAdmin(t *testing.T) {
	h := HttpServer{}
	response := httptest.NewRecorder()
	h.RequireAdmin(h.GetAgentConfigurations)(response, httptest.NewRequest(http.MethodGet, "/agent_configurations", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status %d", response.Code)
	}
}
