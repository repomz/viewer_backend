package httpserver

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/repomz/viewer_backend/internal/app/db"
	"github.com/repomz/viewer_backend/internal/app/domain"
	"github.com/repomz/viewer_backend/internal/app/repository/pgrepo"
	"github.com/repomz/viewer_backend/internal/app/services"
)

func TestTransferDeduplicationAndHeartbeat(t *testing.T) {
	h := sourceTestServer(t)
	for _, name := range []string{"20260718000000_user_requests.sql", "20260726000000_user_request_statuses.sql"} {
		body, err := os.ReadFile("../../../sql/migrations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = h.sqlDB.Exec(strings.Split(string(body), "-- +goose Down")[0]); err != nil {
			t.Fatal(err)
		}
	}
	service := services.NewUserRequestService(pgrepo.NewUserRequestRepo(db.New(h.sqlDB)))
	h.userRequestService = service
	input := domain.NewUserRequest{UserID: "test", AgentID: 1, RequestType: "execute_command", Command: "get_ct", Payload: json.RawMessage(`{"study_uid":"1.2.3"}`), MaxAttempts: 3}
	a, err := h.createTransfer(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	b, err := h.createTransfer(context.Background(), input)
	if err != nil || a.ID != b.ID {
		t.Fatal("duplicate transfer", err)
	}
	h.sqlDB.Exec(`UPDATE user_requests SET status='in_progress',lease_expires_at=NOW()-INTERVAL '1 minute' WHERE id=$1`, a.ID)
	send := func(agent int, stage string) int {
		body, _ := json.Marshal(transferProgress{AgentID: agent, Stage: stage, Completed: 25, Total: 100})
		r := mux.SetURLVars(httptest.NewRequest("POST", "/", strings.NewReader(string(body))), map[string]string{"request_id": a.ID.String()})
		w := httptest.NewRecorder()
		h.RecordTransferProgress(w, r)
		return w.Code
	}
	if send(2, "download") != 409 {
		t.Fatal("another agent must not update")
	}
	if send(1, "download") != 200 {
		t.Fatal("heartbeat rejected")
	}
	var expires time.Time
	h.sqlDB.QueryRow(`SELECT lease_expires_at FROM user_requests WHERE id=$1`, a.ID).Scan(&expires)
	if !expires.After(time.Now().Add(4 * time.Minute)) {
		t.Fatal("lease not extended")
	}
	h.sqlDB.Exec(`UPDATE user_requests SET status='completed' WHERE id=$1`, a.ID)
	if send(1, "upload") != 409 {
		t.Fatal("late heartbeat must not reopen completed request")
	}
}

func TestCTManifestBodyLimit(t *testing.T) {
	body := `{"value":"` + strings.Repeat("x", 2<<20) + `"}`
	var value map[string]string
	if err := decodeJSONLimit(httptest.NewRecorder(), httptest.NewRequest("POST", "/ct_studies", strings.NewReader(body)), &value, 32<<20); err != nil {
		t.Fatal(err)
	}
	if err := decodeJSON(httptest.NewRecorder(), httptest.NewRequest("POST", "/studies", strings.NewReader(body)), &value); err == nil {
		t.Fatal("ordinary protocol limit must remain bounded")
	}
	if err := decodeJSONLimit(httptest.NewRecorder(), httptest.NewRequest("POST", "/ct_studies", strings.NewReader(body)), &value, 1024); err == nil {
		t.Fatal("must enforce limit")
	}
}

func TestInvalidTransferProgress(t *testing.T) {
	for _, body := range []string{`{}`, `{"agent_id":1,"stage":"completed"}`, `{"agent_id":1,"stage":"download","completed":-1}`} {
		w := httptest.NewRecorder()
		HttpServer{}.RecordTransferProgress(w, httptest.NewRequest("POST", "/", strings.NewReader(body)))
		if w.Code != 400 {
			t.Fatalf("status %d", w.Code)
		}
	}
}
