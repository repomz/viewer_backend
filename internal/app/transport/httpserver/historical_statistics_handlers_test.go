package httpserver

import (
	"testing"
	"time"
)

func TestHistoricalStatisticsRoundTrip(t *testing.T) {
	t.Setenv("PLANS_DIR", t.TempDir())
	document := historicalStatisticsDocument{
		SchemaVersion: 2,
		Source:        "test", StartYear: 2024, EndYear: 2026, GeneratedAt: time.Now(),
		OperationTypes: []string{"КАГ", "ЦАГ"},
		Years:          []historicalStatisticsYear{{Year: 2026, Counts: map[string]int{"КАГ": 3, "ЦАГ": 2}}},
	}
	if err := normalizeHistoricalStatistics(&document); err != nil {
		t.Fatal(err)
	}
	if document.Years[0].Total != 5 {
		t.Fatalf("unexpected total: %d", document.Years[0].Total)
	}
	if err := saveHistoricalStatistics(document); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadHistoricalStatistics()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Source != "test" || loaded.Years[0].Counts["КАГ"] != 3 {
		t.Fatalf("unexpected historical statistics: %#v", loaded)
	}
}

func TestHistoricalStatisticsIVUSOverlay(t *testing.T) {
	document := historicalStatisticsDocument{
		SchemaVersion: 3, StartYear: 2025, EndYear: 2025,
		OperationTypes: []string{"СТЕНТ КОР", "ВСУЗИ"},
		Years:          []historicalStatisticsYear{{Year: 2025, Counts: map[string]int{"СТЕНТ КОР": 2, "ВСУЗИ": 1}}},
	}
	if err := normalizeHistoricalStatistics(&document); err != nil {
		t.Fatal(err)
	}
	if document.Years[0].Total != 2 {
		t.Fatalf("IVUS double-counted: %d", document.Years[0].Total)
	}
	document.Years[0].Counts["ВСУЗИ"] = 3
	if err := normalizeHistoricalStatistics(&document); err == nil {
		t.Fatal("accepted impossible IVUS count")
	}
}
