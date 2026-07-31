package httpserver

import (
	"testing"
	"time"
)

func TestLimitAgentRecords(t *testing.T) {
	records := []time.Time{time.Now(), time.Now().Add(-time.Minute)}
	limited := limitAgentRecords(records, "1")
	if len(limited) != 1 || !limited[0].Equal(records[0]) {
		t.Fatalf("unexpected limited records: %#v", limited)
	}
	if got := limitAgentRecords(records, "invalid"); len(got) != len(records) {
		t.Fatalf("invalid limit changed records: %#v", got)
	}
}
