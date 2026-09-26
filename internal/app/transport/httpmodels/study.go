package httpmodels

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/repomz/viewer_backend/internal/app/domain"
)

type StudyRequest struct {
	BirthDate      string    `json:"birth_date"`
	StudyID        string    `json:"study_id"`
	Patient        string    `json:"patient"`
	Age            int32     `json:"age"`
	Department     string    `json:"department"`
	NameOperation  string    `json:"name_operation"`
	StudyType      string    `json:"study_type"`
	DescrOperation string    `json:"descr_operation"`
	Recommendation string    `json:"recommendation"`
	TimeBeginning  time.Time `json:"time_beginning"`
	TimeDuration   int32     `json:"time_duration"`
	Surgeon        string    `json:"surgeon"`
	DicomLink      string    `json:"dicom_link"`
}

func (s *StudyRequest) Validate() error {
	if s.BirthDate != "" {
		birth, err := time.Parse("2006-01-02", s.BirthDate)
		if err != nil || birth.After(s.TimeBeginning) {
			return fmt.Errorf("invalid birth_date: expected YYYY-MM-DD not after operation")
		}
	}
	s.StudyID = strings.TrimSpace(s.StudyID)
	s.Patient = strings.TrimSpace(s.Patient)
	s.Department = strings.TrimSpace(s.Department)
	s.NameOperation = strings.TrimSpace(s.NameOperation)
	s.StudyType = strings.ToLower(strings.TrimSpace(s.StudyType))
	s.DescrOperation = strings.TrimSpace(s.DescrOperation)
	s.Recommendation = strings.TrimSpace(s.Recommendation)
	s.Surgeon = strings.ToLower(strings.TrimSpace(s.Surgeon))
	s.DicomLink = strings.TrimSpace(s.DicomLink)

	if s.StudyID == "" {
		return fmt.Errorf("%w: study_id", domain.ErrRequired)
	}
	if s.Patient == "" {
		return fmt.Errorf("%w: patient", domain.ErrRequired)
	}
	if s.Age < 0 {
		return fmt.Errorf("%w: age", domain.ErrNegative)
	}
	if s.Department == "" {
		return fmt.Errorf("%w: department", domain.ErrRequired)
	}
	if s.NameOperation == "" {
		return fmt.Errorf("%w: name_operation", domain.ErrRequired)
	}
	if s.StudyType == "" {
		return fmt.Errorf("%w: study_type", domain.ErrRequired)
	}
	if s.DescrOperation == "" {
		return fmt.Errorf("%w: descr_operation", domain.ErrRequired)
	}
	if s.TimeBeginning.IsZero() {
		return fmt.Errorf("%w: time_beginning", domain.ErrRequired)
	}
	if s.TimeDuration < 0 {
		return fmt.Errorf("%w: time_duration", domain.ErrNegative)
	}
	if s.Surgeon == "" {
		return fmt.Errorf("%w: surgeon", domain.ErrRequired)
	}
	return nil
}

type StudyDicomLinkRequest struct {
	DicomLink string `json:"dicom_link"`
}

func (s *StudyDicomLinkRequest) Validate() error {
	s.DicomLink = strings.TrimSpace(s.DicomLink)
	if s.DicomLink == "" {
		return fmt.Errorf("%w: dicom_link", domain.ErrRequired)
	}
	return nil
}

type StudyResponse struct {
	BirthDate      string    `json:"birth_date,omitempty"`
	ID             uuid.UUID `json:"id"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	StudyID        string    `json:"study_id"`
	Patient        string    `json:"patient"`
	Age            int32     `json:"age"`
	Department     string    `json:"department"`
	NameOperation  string    `json:"name_operation"`
	StudyType      string    `json:"study_type"`
	DescrOperation string    `json:"descr_operation"`
	Recommendation string    `json:"recommendation"`
	TimeBeginning  time.Time `json:"time_beginning"`
	TimeDuration   int32     `json:"time_duration"`
	Surgeon        string    `json:"surgeon"`
	DicomLink      string    `json:"dicom_link"`
}
