package httpserver

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/repomz/viewer_backend/internal/app/domain"
)

func TestShouldDeleteProtocolStudyAppliesWeeklyPolicy(t *testing.T) {
	location := time.FixedZone("Asia/Tomsk", 7*60*60)
	now := time.Date(2026, time.August, 3, 10, 0, 0, 0, location)
	plan := operationPlanFile{
		Days: map[string][]operationPlanEntry{
			"2026-07-31": {{Patient: "Планов П.П."}},
		},
	}
	tests := []struct {
		name      string
		date      time.Time
		patient   string
		studyType string
		want      bool
	}{
		{"previous Monday", time.Date(2026, 7, 27, 12, 0, 0, 0, location), "Иванов", "каг", true},
		{"previous Thursday", time.Date(2026, 7, 30, 12, 0, 0, 0, location), "Иванов", "цаг", true},
		{"planned previous Friday", time.Date(2026, 7, 31, 12, 0, 0, 0, location), "Планов Пётр", "каг", true},
		{"emergency previous Friday", time.Date(2026, 7, 31, 12, 0, 0, 0, location), "Экстренный Е.", "каг", false},
		{"previous Saturday", time.Date(2026, 8, 1, 12, 0, 0, 0, location), "Экстренный Е.", "каг", false},
		{"older emergency Friday", time.Date(2026, 7, 24, 12, 0, 0, 0, location), "Экстренный Е.", "каг", true},
		{"current Monday", time.Date(2026, 8, 3, 12, 0, 0, 0, location), "Сегодня С.", "каг", false},
		{"old XA is not a protocol", time.Date(2026, 7, 1, 12, 0, 0, 0, location), "Пациент", "XA", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			study := domain.ResponseToDBStudy(domain.DBStudyData{
				ID:            uuid.New(),
				Patient:       test.patient,
				StudyType:     test.studyType,
				TimeBeginning: test.date,
			})
			if got := shouldDeleteProtocolStudy(study, plan, now); got != test.want {
				t.Fatalf("shouldDeleteProtocolStudy() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestRunStudyRetentionWaitsUntilMondayMorning(t *testing.T) {
	location := time.FixedZone("Asia/Tomsk", 7*60*60)
	sunday := time.Date(2026, 8, 2, 23, 0, 0, 0, location)
	mondayEarly := time.Date(2026, 8, 3, 8, 59, 0, 0, location)
	mondayMorning := time.Date(2026, 8, 3, 9, 0, 0, 0, location)
	tuesday := time.Date(2026, 8, 4, 9, 0, 0, 0, location)
	if studyRetentionIsDue(sunday) {
		t.Fatal("retention must not run on Sunday")
	}
	if studyRetentionIsDue(mondayEarly) {
		t.Fatal("retention must wait until Monday 09:00")
	}
	if !studyRetentionIsDue(mondayMorning) || !studyRetentionIsDue(tuesday) {
		t.Fatal("retention must run Monday morning and recover on later weekdays")
	}
}
