package httpserver

import (
	"context"
	"encoding/json"
	"github.com/repomz/viewer_backend/internal/app/domain"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReportRefreshAddsLateProtocolWithoutNewCard(t *testing.T) {
	t.Setenv("REPORTS_DIR", t.TempDir())
	t.Setenv("PLANS_DIR", t.TempDir())
	service := &studyServiceStub{}
	handler := NewHttpServer(service, nil)
	now := time.Date(2026, 9, 30, 7, 45, 0, 0, time.Local)
	original, filename, err := handler.generateAndStoreReport(context.Background(), reportGenerateRequest{Days: 1}, now)
	if err != nil {
		t.Fatal(err)
	}
	service.studies = []domain.Study{domain.ResponseToDBStudy(domain.DBStudyData{
		StudyID: "late", Patient: "Тестова Нина Алексеевна", StudyType: "каг", NameOperation: "КАГ", TimeBeginning: now.Add(-16 * time.Hour),
	})}
	if err := handler.refreshStoredReports(context.Background()); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(reportsDirectory(), filename))
	if err != nil {
		t.Fatal(err)
	}
	var updated reportRequest
	if err := json.Unmarshal(body, &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Report["emergency_total"] != float64(1) || updated.GeneratedAt != original.GeneratedAt {
		t.Fatalf("late report was not updated correctly: %#v", updated)
	}
	files, _ := os.ReadDir(reportsDirectory())
	if len(files) != 1 {
		t.Fatal("refresh created extra cards")
	}
	if err := handler.refreshStoredReports(context.Background()); err != nil {
		t.Fatal(err)
	}
	again, _ := os.ReadFile(filepath.Join(reportsDirectory(), filename))
	if string(again) != string(body) {
		t.Fatal("unchanged refresh rewrote document")
	}
}

func TestReportDoesNotCollapseRepeatedOperationsOrNamesakes(t *testing.T) {
	t.Setenv("PLANS_DIR", t.TempDir())
	if err := saveOperationPlan(operationPlanFile{Days: map[string][]operationPlanEntry{"2026-09-29": {{Patient: "Иванов ИИ"}}}}); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 9, 29, 7, 45, 0, 0, time.Local)
	service := &studyServiceStub{}
	for i, name := range []string{"Иванов Иван Иванович", "Иванов Иван Иванович", "Иванов Петр Петрович"} {
		service.studies = append(service.studies, domain.ResponseToDBStudy(domain.DBStudyData{Patient: name, StudyType: "каг", TimeBeginning: start.Add(time.Duration(i+1) * time.Hour)}))
	}
	report, err := NewHttpServer(service, nil).buildOperationsReport(context.Background(), start, start.AddDate(0, 0, 1), 1)
	if err != nil {
		t.Fatal(err)
	}
	if report["planned_count"] != 2 || report["emergency_total"] != 1 {
		t.Fatalf("incorrect counts: %#v", report)
	}
	if dutyDate(start.Add(-time.Minute)) != "2026-09-28" || dutyDate(start) != "2026-09-29" {
		t.Fatal("incorrect 07:45 boundary")
	}
}
