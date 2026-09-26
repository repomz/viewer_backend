package httpserver

import (
	"encoding/json"
	"github.com/google/uuid"
	"net/http"
)

func (h HttpServer) RecordAppOpen(w http.ResponseWriter, r *http.Request) {
	user, _ := currentAuthenticatedUser(r)
	if user.Role == "admin" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var input struct {
		EventID string `json:"event_id"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writePlatformError(w, 400, "Некорректное событие")
		return
	}
	id, err := uuid.Parse(input.EventID)
	if err != nil {
		writePlatformError(w, 400, "Некорректный идентификатор события")
		return
	}
	_, err = h.sqlDB.ExecContext(r.Context(), `INSERT INTO login_events (user_id, event_id) VALUES ($1, $2) ON CONFLICT (event_id) DO NOTHING`, user.ID, id)
	if err != nil {
		writePlatformError(w, 500, "Не удалось записать открытие")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
