package httpserver

import (
	"context"
	"testing"
	"time"

	"github.com/repomz/viewer_backend/internal/app/domain"
)

func TestReportConvertsAgentUTCAndMatchesPlannedDay(t *testing.T) {
	previousLocation := time.Local
	time.Local = time.FixedZone("Tomsk", 7*60*60)
	t.Cleanup(func() { time.Local = previousLocation })
	t.Setenv("PLANS_DIR", t.TempDir())
	if err := saveOperationPlan(operationPlanFile{Days: map[string][]operationPlanEntry{
		"2026-09-30": {{Patient: "Пациент ИИ"}},
	}}); err != nil {
		t.Fatal(err)
	}
	studies := []domain.Study{}
	for _, hour := range []int{0, 1, 2, 3, 4} {
		studies = append(studies, domain.ResponseToDBStudy(domain.DBStudyData{
			Patient: "Пациент Иван Иванович", StudyType: "каг", NameOperation: "КАГ",
			TimeBeginning: time.Date(2026, 9, 30, hour, 25, 0, 0, time.UTC),
		}))
	}
	service := &studyServiceStub{studies: studies}
	h := NewHttpServer(service, nil)
	report, err := h.buildOperationsReport(context.Background(),
		time.Date(2026, 9, 30, 7, 45, 0, 0, time.Local),
		time.Date(2026, 10, 1, 7, 45, 0, 0, time.Local), 1)
	if err != nil {
		t.Fatal(err)
	}
	if service.since.Location() != time.UTC || service.since.Hour() != 0 || service.since.Minute() != 45 {
		t.Fatalf("SQL timestamp boundary must be UTC 00:45, got %v", service.since)
	}
	if report["planned_count"] != 4 || report["emergency_total"] != 0 {
		t.Fatalf("incorrect classification: %v", report)
	}
	operations := report["planned_operations"].([]map[string]any)
	if operations[0]["time_beginning"] != "08:25" {
		t.Fatalf("wall-clock time shifted: %v", operations[0]["time_beginning"])
	}
}

func TestReportPeriodSupportsDaysAndInclusiveCalendarRange(t *testing.T) {
	now := time.Date(2026, 8, 3, 12, 0, 0, 0, time.Local)
	start, end, days, err := reportPeriod(reportGenerateRequest{Days: 3}, now)
	if err != nil || days != 3 || start.Hour() != 7 || start.Minute() != 45 || end.Day() != 3 {
		t.Fatalf("rolling period = %v %v %d, err=%v", start, end, days, err)
	}
	start, end, days, err = reportPeriod(reportGenerateRequest{
		DateFrom: "2026-07-30", DateTo: "2026-08-02",
	}, now)
	if err != nil || days != 4 || start.Day() != 30 || end.Day() != 3 {
		t.Fatalf("calendar period = %v %v %d, err=%v", start, end, days, err)
	}
}

func TestBuildOperationsReportUsesPlanToSeparateEmergencyOperations(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("PLANS_DIR", directory)
	plan := operationPlanFile{Days: map[string][]operationPlanEntry{
		"2026-08-02": {
			{Patient: "Иванов И.И.", Department: "кардио 2", Operation: "КАГ"},
			{Patient: "Несделан Н.Н.", Department: "кардио 2", Operation: "КАГ"},
		},
		"2026-08-03": {{Patient: "Сидоров С.С.", Department: "кардио 1", Operation: "КАГ"}},
	}}
	if err := saveOperationPlan(plan); err != nil {
		t.Fatal(err)
	}
	service := &studyServiceStub{studies: []domain.Study{
		domain.ResponseToDBStudy(domain.DBStudyData{
			StudyID: "1", Patient: "Иванов Иван Иванович", Age: 50, Department: "кардио 2",
			NameOperation: "КАГ", StudyType: "каг",
			TimeBeginning: time.Date(2026, 8, 2, 10, 0, 0, 0, time.Local), Surgeon: "Врач",
		}),
		domain.ResponseToDBStudy(domain.DBStudyData{
			StudyID: "2", Patient: "Петров Петр", Age: 60, Department: "рсц",
			NameOperation: "ЦАГ", StudyType: "цаг",
			TimeBeginning: time.Date(2026, 8, 2, 12, 0, 0, 0, time.Local), Surgeon: "Врач",
		}),
	}}
	handler := NewHttpServer(service, nil)
	report, err := handler.buildOperationsReport(
		context.Background(),
		time.Date(2026, 8, 2, 8, 0, 0, 0, time.Local),
		time.Date(2026, 8, 3, 8, 0, 0, 0, time.Local),
		1,
	)
	if err != nil {
		t.Fatal(err)
	}
	if report["planned_count"] != 1 || report["emergency_total"] != 1 || report["today_planned_count"] != 1 {
		t.Fatalf("report counts = %#v", report)
	}
	planned := report["planned_operations"].([]map[string]any)
	if len(planned) != 1 || planned[0]["patient"] != "Иванов Иван Иванович" {
		t.Fatalf("planned operations contain an unperformed plan entry: %#v", planned)
	}
}
