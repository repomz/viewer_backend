package httpserver

import (
	"context"
	"encoding/json"
	"github.com/gorilla/mux"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDutySchedulePublication(t *testing.T) {
	t.Setenv("PLANS_DIR", t.TempDir())
	doc, err := photoSchedule(photoFixture(), "2026-10")
	if err != nil {
		t.Fatal(err)
	}
	if scheduleReady(doc) {
		t.Fatal("legacy schedule published without upload")
	}
	now := time.Now()
	doc.SourceUploadedAt = &now
	if !scheduleReady(doc) {
		t.Fatal("clean uploaded schedule not ready")
	}
	doc.Groups[1].Staff[0].Review["1:duty"] = true
	if scheduleReady(doc) {
		t.Fatal("unresolved schedule published")
	}
	doc.Published = true // A stored/client flag must not override actual validation.
	if err := saveDutySchedule(doc); err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"admin", "user"} {
		req := mux.SetURLVars(httptest.NewRequest("GET", "/", nil), map[string]string{"month": "2026-10"})
		req = req.WithContext(context.WithValue(req.Context(), userContextKey{}, authenticatedUser{Role: role}))
		w := httptest.NewRecorder()
		(HttpServer{}).GetDutySchedule(w, req)
		var got dutyScheduleDocument
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.Published || (role == "user" && len(got.Groups) != 0) || (role == "admin" && len(got.Groups) == 0) {
			t.Fatalf("invalid access for %s", role)
		}
	}
}
