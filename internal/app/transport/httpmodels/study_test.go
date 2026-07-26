package httpmodels

import (
	"errors"
	"testing"
	"time"

	"github.com/repomz/viewer_backend/internal/app/domain"
)

func validStudyRequest() StudyRequest {
	return StudyRequest{
		StudyID:        "STUDY-1",
		Patient:        "Иванов И.И.",
		Age:            50,
		Department:     "кардиология",
		NameOperation:  "Коронарная ангиография",
		StudyType:      " КАГ ",
		DescrOperation: "Плановое исследование",
		TimeBeginning:  time.Now().Add(-time.Hour),
		TimeDuration:   30,
		Surgeon:        " ИДРИСОВ ",
	}
}

func TestStudyRequestValidateAndNormalize(t *testing.T) {
	request := validStudyRequest()
	request.StudyType = " ЭМБОЛИЗАЦИЯ МАТОЧНЫХ АРТЕРИЙ "
	request.Surgeon = " ПЕТРОВ "

	if err := request.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if request.StudyType != "эмболизация маточных артерий" {
		t.Fatalf(
			"StudyType = %q, want %q",
			request.StudyType,
			"эмболизация маточных артерий",
		)
	}
	if request.Surgeon != "петров" {
		t.Fatalf("Surgeon = %q, want %q", request.Surgeon, "петров")
	}
}

func TestStudyRequestRejectsMissingAndInvalidValues(t *testing.T) {
	tests := []struct {
		name   string
		change func(*StudyRequest)
		target error
	}{
		{"missing study id", func(r *StudyRequest) { r.StudyID = "" }, domain.ErrRequired},
		{"negative age", func(r *StudyRequest) { r.Age = -1 }, domain.ErrNegative},
		{"missing study type", func(r *StudyRequest) { r.StudyType = "" }, domain.ErrRequired},
		{"missing operation time", func(r *StudyRequest) { r.TimeBeginning = time.Time{} }, domain.ErrRequired},
		{"negative duration", func(r *StudyRequest) { r.TimeDuration = -1 }, domain.ErrNegative},
		{"missing surgeon", func(r *StudyRequest) { r.Surgeon = "" }, domain.ErrRequired},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := validStudyRequest()
			tt.change(&request)
			if err := request.Validate(); !errors.Is(err, tt.target) {
				t.Fatalf("Validate() error = %v, want errors.Is(%v)", err, tt.target)
			}
		})
	}
}

func TestStudyFilterAllowsArbitrarySurgeonAndStudyType(t *testing.T) {
	filter := StudyFilter{
		Surgeon:   " ПЕТРОВ ",
		StudyType: " ЭМБОЛИЗАЦИЯ ",
	}

	if err := filter.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	filter.Normalize()
	if filter.Surgeon != "петров" || filter.StudyType != "эмболизация" {
		t.Fatalf("Normalize() = surgeon %q, type %q", filter.Surgeon, filter.StudyType)
	}
}

func TestAgentRecordRequestValidate(t *testing.T) {
	valid := AgentRecordRequest{AgentID: "1", Status: " WELL "}
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if valid.Status != "well" {
		t.Fatalf("Status = %q, want well", valid.Status)
	}

	invalid := AgentRecordRequest{AgentID: "3", Status: "broken"}
	if err := invalid.Validate(); !errors.Is(err, domain.ErrInvalidAgentID) {
		t.Fatalf("Validate() error = %v, want invalid agent ID", err)
	}
}

func TestPatientFilterAllowsInitials(t *testing.T) {
	filter := PatientFilter{Patient: " Николаев П.С. "}
	if err := filter.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if filter.Patient != "Николаев П.С." {
		t.Fatalf("Patient = %q, want trimmed value", filter.Patient)
	}
}
