package httpserver

import (
	"github.com/repomz/viewer_backend/internal/app/domain"
	"testing"
	"time"
)

func TestPlanBirthMatching(t *testing.T) {
	date := time.Date(2023, 9, 26, 10, 0, 0, 0, time.Local)
	study := func(age int32, birth string) domain.Study {
		return domain.ResponseToDBStudy(domain.DBStudyData{Age: age, BirthDate: parseStudyBirthDate(birth), TimeBeginning: date})
	}
	for _, tc := range []struct {
		name, birth string
		study       domain.Study
		want        bool
	}{
		{"missing plan birth", "", study(69, ""), false},
		{"different Smirnov", "1954-01-01", study(64, ""), false},
		{"correct historical age", "1954-01-01", study(69, ""), true},
		{"birthday tomorrow", "1954-09-27", study(68, ""), true},
		{"no evidence", "1954-01-01", study(0, ""), false},
		{"exact birthday wins", "1954-01-01", study(0, "1954-01-01"), true},
		{"different birthday same age", "1954-01-01", study(69, "1954-01-02"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := planBirthMatches(tc.birth, tc.study); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestPlanWithoutBirthdayNeverReturnsHistoricalNamesake(t *testing.T) {
	study := domain.ResponseToDBStudy(domain.DBStudyData{
		Patient: "Смирнов Александр Иванович", Age: 64, StudyType: "каг",
		TimeBeginning: time.Date(2023, 7, 24, 9, 0, 0, 0, time.Local),
	})
	entries := responsePlanEntries([]operationPlanEntry{{Patient: "Смирнов АИ"}}, []domain.Study{study}, time.Date(2026, 9, 25, 0, 0, 0, 0, time.Local))
	if !entries[0].HistorySearched || len(entries[0].PreviousOperations) != 0 {
		t.Fatal("without birthday, history is searched but older than one year is excluded")
	}
}

func TestPlanWithoutBirthdaySearchesRollingYear(t *testing.T) {
	date := time.Date(2026, 10, 10, 0, 0, 0, 0, time.Local)
	studies := []domain.Study{}
	for _, day := range []time.Time{date.AddDate(-1, 0, 0), date.AddDate(-1, 0, -1), date.AddDate(0, 0, -1)} {
		studies = append(studies, domain.ResponseToDBStudy(domain.DBStudyData{Patient: "Иванов Иван Иванович", StudyType: "каг", TimeBeginning: day}))
	}
	previous, _ := planProtocols(operationPlanEntry{Patient: "Иванов ИИ"}, studies, date)
	if len(previous) != 2 {
		t.Fatalf("got %d, want rolling year only", len(previous))
	}
	previous, _ = planProtocols(operationPlanEntry{Patient: "Иванов"}, studies, date)
	if len(previous) != 0 {
		t.Fatal("surname alone must not search history")
	}
}
