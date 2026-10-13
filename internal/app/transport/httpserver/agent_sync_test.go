package httpserver

import (
	"testing"
	"time"
)

func TestProbeStatus(t *testing.T) {
	p := protocolProbe{StudyID: "123", Patient: "Иванов Иван Иванович", Room: 1, Time: time.Now()}
	if probeStatus(p, nil) != "missing" {
		t.Fatal("missing")
	}
	if probeStatus(p, []protocolProbe{p}) != "present" {
		t.Fatal("present")
	}
	if probeStatus(p, []protocolProbe{p, p}) != "review" {
		t.Fatal("duplicate")
	}
	other := p
	other.Room = 2
	if probeStatus(p, []protocolProbe{other}) != "missing" {
		t.Fatal("different rooms")
	}
	other = p
	other.Patient = "Другой Пациент"
	if probeStatus(p, []protocolProbe{other}) != "review" {
		t.Fatal("conflicting identity")
	}
	other = p
	other.StudyID = "124"
	if probeStatus(p, []protocolProbe{other}) != "review" {
		t.Fatal("renumbered protocol")
	}
	p.Room = 0
	if probeStatus(p, nil) != "review" {
		t.Fatal("unknown room")
	}
}
