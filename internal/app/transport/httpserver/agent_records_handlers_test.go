package httpserver

import (
	"testing"
)

func TestAgentRecordLimit(t *testing.T) {
	if got := agentRecordLimit("1"); got != 1 {
		t.Fatalf("limit = %d, want 1", got)
	}
	if got := agentRecordLimit("invalid"); got != 1000 {
		t.Fatalf("invalid limit = %d, want default", got)
	}
	if got := agentRecordLimit("5000"); got != 1000 {
		t.Fatalf("oversized limit = %d, want cap", got)
	}
}
