package httpserver

import (
	"bytes"
	"encoding/json"
	"github.com/gorilla/mux"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func photoFixture() schedulePhotoResult {
	return schedulePhotoResult{Month: "2026-10", Rows: []schedulePhotoRow{
		{Name: "Идрисов М.З.", Group: "surgeons", Row: "duty", Cells: make([]schedulePhotoCell, 31)},
		{Name: "Воронова Ж.А.", Group: "nurses", Row: "duty", Cells: make([]schedulePhotoCell, 31)},
	}}
}
func TestDutySchedulePhotoAndReviewRoundTrip(t *testing.T) {
	t.Setenv("PLANS_DIR", t.TempDir())
	photo := photoFixture()
	photo.Rows[1].Cells[0] = schedulePhotoCell{Value: "16", Uncertain: true}
	doc, err := photoSchedule(photo, "2026-10")
	if err != nil {
		t.Fatal(err)
	}
	if err = saveDutySchedule(doc); err != nil {
		t.Fatal(err)
	}
	got, err := loadDutySchedule("2026-10")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Groups) != 2 || got.Groups[1].Staff[0].Shifts["1:duty"] != "16" || !got.Groups[1].Staff[0].Review["1:duty"] {
		t.Fatal("nurses or review metadata lost")
	}
}
func TestDutySchedulePhotoValidation(t *testing.T) {
	photo := photoFixture()
	if _, err := photoSchedule(photo, "2026-11"); err == nil {
		t.Fatal("month mismatch accepted")
	}
	photo.Rows[1].Cells = photo.Rows[1].Cells[:30]
	if _, err := photoSchedule(photo, "2026-10"); err == nil {
		t.Fatal("missing day accepted")
	}
	photo = photoFixture()
	photo.Rows[1].Cells[0].Value = "?"
	doc, err := photoSchedule(photo, "2026-10")
	if err != nil {
		t.Fatal(err)
	}
	if !doc.Groups[1].Staff[0].Review["1:duty"] {
		t.Fatal("unreadable value not flagged")
	}
}

// Explicit opt-in smoke test. Credentials and source photographs never enter Git.
func TestDutyScheduleVisionLive(t *testing.T) {
	path := os.Getenv("SCHEDULE_TEST_PHOTO")
	if path == "" {
		t.Skip("live provider test disabled")
	}
	t.Setenv("PLANS_DIR", t.TempDir())
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/duty-schedule/2026-10/recognize", bytes.NewReader(data))
	req = mux.SetURLVars(req, map[string]string{"month": "2026-10"})
	w := httptest.NewRecorder()
	(HttpServer{}).RecognizeDutySchedule(w, req)
	if w.Code != 200 {
		t.Fatalf("recognition status %d: %s", w.Code, w.Body.String())
	}
	var doc dutyScheduleDocument
	if err = json.Unmarshal(w.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Groups) != 2 || len(doc.Groups[1].Staff) != 8 {
		t.Fatal("unexpected staff count")
	}
	flags := 0
	for _, group := range doc.Groups {
		for _, staff := range group.Staff {
			flags += len(staff.Review)
		}
	}
	t.Logf("Recognized surgeons=%d nurses=%d", len(doc.Groups[0].Staff), len(doc.Groups[1].Staff))
	t.Logf("Cells requiring review: %d", flags)
}
