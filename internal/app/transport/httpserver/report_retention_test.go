package httpserver

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCleanupReportsKeepsRecentAndUnrelatedFiles(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("REPORTS_DIR", dir)
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	for name, content := range map[string]string{
		"old.json":    `{"generated_at":"2026-09-18T12:00:00Z","report":{}}`,
		"recent.json": `{"generated_at":"2026-09-26T10:00:00Z","report":{}}`,
		"other.json":  `{"unrelated":true}`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	cleanupExpiredReports(now)
	if _, err := os.Stat(filepath.Join(dir, "old.json")); !os.IsNotExist(err) {
		t.Fatal("expired report retained")
	}
	for _, name := range []string{"recent.json", "other.json"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestMorningReportBoundary(t *testing.T) {
	now := time.Date(2026, 9, 28, 7, 45, 0, 0, time.UTC)
	start, end, days, err := reportPeriod(reportGenerateRequest{Days: scheduledReportDays(now)}, now)
	if err != nil || days != 3 || start.Weekday() != time.Friday || end.Minute() != 45 {
		t.Fatalf("invalid Monday report %v %v %d %v", start, end, days, err)
	}
	if lastCompletedDutyEnd(now.Add(-time.Second)).Day() != 27 {
		t.Fatal("report boundary advanced before 07:45")
	}
}
