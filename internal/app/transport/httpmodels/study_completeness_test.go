package httpmodels

import (
	"github.com/repomz/viewer_backend/internal/app/domain"
	"testing"
	"time"
)

func TestProtocolCompleteness(t *testing.T) {
	data := domain.DBStudyData{StudyID: "pending-test", Patient: "Тест", StudyType: "каг", NameOperation: "КАГ", Department: "не указано", Surgeon: "не указано", TimeBeginning: time.Now()}
	if got := MissingProtocolFields(domain.ResponseToDBStudy(data)); len(got) != 4 {
		t.Fatal(got)
	}
	data.StudyID = "123"
	data.Room = 1
	data.Department = "рсц"
	data.Surgeon = "идрисов"
	if got := MissingProtocolFields(domain.ResponseToDBStudy(data)); len(got) != 0 {
		t.Fatal(got)
	}
}
