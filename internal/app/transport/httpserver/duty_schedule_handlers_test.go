package httpserver

import (
	"testing"
)

func TestDutyScheduleRoundTrip(t *testing.T) {
	t.Setenv("PLANS_DIR", t.TempDir())
	document := defaultDutySchedule("2026-09")
	document.Holidays = []int{6, 6, 13}
	document.Groups[0].Staff[0].Shifts["3"] = "24"
	if err := normalizeDutySchedule(&document, "2026-09"); err != nil {
		t.Fatal(err)
	}
	if len(document.Holidays) != 2 {
		t.Fatalf("holidays were not normalized: %#v", document.Holidays)
	}
	if err := saveDutySchedule(document); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadDutySchedule("2026-09")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Groups[0].Staff[0].Shifts["3"] != "24" {
		t.Fatalf("shift was not saved: %#v", loaded)
	}
}

func TestDutyScheduleRejectsInvalidMonth(t *testing.T) {
	document := defaultDutySchedule("2026-09")
	if err := normalizeDutySchedule(&document, "09-2026"); err == nil {
		t.Fatal("invalid month must be rejected")
	}
}

func TestDutyScheduleExcludesTereshinFromStoredAndNewSchedules(t *testing.T) {
	t.Setenv("PLANS_DIR", t.TempDir())
	document := defaultDutySchedule("2026-10")
	document.Groups[0].Staff = append(document.Groups[0].Staff, dutyScheduleStaff{ID: "legacy", Name: "Терешин А.А.", Shifts: map[string]string{"1:day": "6"}})
	if err := saveDutySchedule(document); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadDutySchedule("2026-10")
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Groups[0].Staff) != 5 {
		t.Fatal("excluded surgeon still visible in saved schedule")
	}
	if err := normalizeDutySchedule(&document, "2026-10"); err != nil {
		t.Fatal(err)
	}
	if len(document.Groups[0].Staff) != 5 {
		t.Fatal("excluded surgeon accepted on save")
	}
}
