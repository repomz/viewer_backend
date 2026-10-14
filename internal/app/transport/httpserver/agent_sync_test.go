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
	for _, records := range [][]protocolProbe{{p, other}, {other, p}} {
		if probeStatus(p, records) != "present" {
			t.Fatal("unrelated repeated number must not hide an exact receipt")
		}
	}
	unknownRoom := p
	unknownRoom.Room = 0
	if probeStatus(unknownRoom, []protocolProbe{p, other}) != "present" {
		t.Fatal("unique exact receipt also confirms a source with unknown room")
	}
	other = p
	other.StudyID = "124"
	if probeStatus(p, []protocolProbe{other}) != "review" {
		t.Fatal("renumbered protocol")
	}
	if probeStatus(p, []protocolProbe{p, other}) != "review" {
		t.Fatal("ambiguous same-patient same-time records still require review")
	}
	p.Room = 0
	if probeStatus(p, nil) != "review" {
		t.Fatal("unknown room")
	}
}
