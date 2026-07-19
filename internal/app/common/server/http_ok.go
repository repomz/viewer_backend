package server

import (
	"encoding/json"
	"net/http"
)

func RespondOK(data any, w http.ResponseWriter, r *http.Request) {
	RespondJSON(http.StatusOK, data, w)
}

func RespondCreated(data any, w http.ResponseWriter, r *http.Request) {
	RespondJSON(http.StatusCreated, data, w)
}

func RespondJSON(status int, data any, w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
