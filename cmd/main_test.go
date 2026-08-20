package main

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestVersionHandler(t *testing.T) {
	previousVersion, previousRevision := version, revision
	version, revision = "0.2.8", "abc123"
	t.Cleanup(func() {
		version, revision = previousVersion, previousRevision
	})

	response := httptest.NewRecorder()
	versionHandler(response, httptest.NewRequest("GET", "/version", nil))

	if response.Code != 200 {
		t.Fatalf("unexpected status: %d", response.Code)
	}
	var payload versionResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Version != "0.2.8" || payload.Revision != "abc123" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}
