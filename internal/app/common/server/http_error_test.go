package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/repomz/viewer_backend/internal/app/domain"
)

func TestNotFoundUsesHTTP404(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/missing", nil)

	RespondWithError(domain.ErrNotFound, recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func TestValidationErrorUsesHTTP400(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)

	RespondWithError(domain.ErrInvalidStudyType, recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}
