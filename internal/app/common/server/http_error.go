package server

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"

	"github.com/repomz/viewer_backend/internal/app/common/slugerrors"
	"github.com/repomz/viewer_backend/internal/app/domain"
)

func InternalError(slug string, err error, w http.ResponseWriter, r *http.Request) {
	httpRespondWithError(err, slug, w, r, "Internal server error", http.StatusInternalServerError)
}

func BadGateway(slug string, err error, w http.ResponseWriter, r *http.Request) {
	httpRespondWithError(err, slug, w, r, "Bad Gateway", http.StatusBadGateway)
}

func Unauthorised(slug string, err error, w http.ResponseWriter, r *http.Request) {
	httpRespondWithError(err, slug, w, r, "Unauthorised", http.StatusUnauthorized)
}

func BadRequest(slug string, err error, w http.ResponseWriter, r *http.Request) {
	httpRespondWithError(err, slug, w, r, "Bad request", http.StatusBadRequest)
}

func NotFound(slug string, err error, w http.ResponseWriter, r *http.Request) {
	httpRespondWithError(err, slug, w, r, "Not found", http.StatusNotFound)
}

func Conflict(slug string, err error, w http.ResponseWriter, r *http.Request) {
	httpRespondWithError(err, slug, w, r, "Conflict", http.StatusConflict)
}

func RespondWithError(err error, w http.ResponseWriter, r *http.Request) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		NotFound("not-found", err, w, r)
		return
	case errors.Is(err, domain.ErrConflict):
		Conflict("conflict", err, w, r)
		return
	case errors.Is(err, domain.ErrRequired),
		errors.Is(err, domain.ErrNegative),
		errors.Is(err, domain.ErrInvalidPatient),
		errors.Is(err, domain.ErrInvalidAgentID),
		errors.Is(err, domain.ErrInvalidSurgeon),
		errors.Is(err, domain.ErrInvalidStudyType),
		errors.Is(err, domain.ErrInvalidStatus),
		errors.Is(err, domain.ErrInvalidCommand),
		errors.Is(err, domain.ErrInvalidRequest):
		BadRequest("invalid-request", err, w, r)
		return
	}

	var slugError slugerrors.SlugError
	if !errors.As(err, &slugError) {
		InternalError("internal-server-error", err, w, r)
		return
	}

	switch slugError.ErrorType() {
	case slugerrors.ErrorTypeAuthorization:
		Unauthorised(slugError.Slug(), slugError, w, r)
	case slugerrors.ErrorTypeBadRequest:
		BadRequest(slugError.Slug(), slugError, w, r)
	case slugerrors.ErrorTypeNotFound:
		NotFound(slugError.Slug(), slugError, w, r)
	default:
		InternalError(slugError.Slug(), slugError, w, r)
	}
}

func httpRespondWithError(err error, slug string, w http.ResponseWriter, _ *http.Request, msg string, status int) {
	log.Printf("error: %s, slug: %s, msg: %s", err, slug, msg)

	resp := ErrorResponse{Slug: slug, httpStatus: status}
	if os.Getenv("DEBUG_ERRORS") != "" && err != nil {
		resp.Error = err.Error()
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}

type ErrorResponse struct {
	Slug       string `json:"slug"`
	Error      string `json:"error,omitempty"`
	httpStatus int
}

func (e ErrorResponse) Render(w http.ResponseWriter, _ *http.Request) error {
	w.WriteHeader(e.httpStatus)
	return nil
}
