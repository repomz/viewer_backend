package httpmodels

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/repomz/viewer_backend/internal/app/domain"
)

type StudyRequest struct {
	ID             uuid.UUID `json:"id"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	StudyID        string    `json:"study_id"`
	Patient        string    `json:"patient"`
	Age            int32     `json:"age"`
	Department     string    `json:"department"`
	NameOperation  string    `json:"name_operation"`
	DescrOperation string    `json:"descr_operation"`
	TimeBeginning  time.Time `json:"time_begining"`
	TimeDuration   int32     `json:"time_duration"`
	Surgeon        string    `json:"surgeon"`
	DicomLink      string    `json:"dicom_link"`
}

func (s *StudyRequest) Validate() error {
	if s.NameOperation == "" {
		return fmt.Errorf("%w: name operation", domain.ErrRequired)
	}
	if s.DescrOperation == "" {
		return fmt.Errorf("%w: description operationr", domain.ErrNegative)
	}
	if s.Surgeon == "" {
		return fmt.Errorf("%w: surgeon", domain.ErrRequired)
	}
	return nil
}

type StudyDicomLinkRequest struct {
	ID        uuid.UUID `json:"id"`
	DicomLink string    `json:"dicom_link"`
}

func (s *StudyDicomLinkRequest) Validate() error {
	if s.ID != uuid.Nil {
		return fmt.Errorf("%w: id", domain.ErrRequired)
	}
	if s.DicomLink == "" {
		return fmt.Errorf("%w: dicom link", domain.ErrRequired)
	}
	return nil
}

type StudyResponse struct {
	ID             uuid.UUID `json:"id"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	StudyID        string    `json:"study_id"`
	Patient        string    `json:"patient"`
	Age            int32     `json:"age"`
	Department     string    `json:"department"`
	NameOperation  string    `json:"name_operation"`
	DescrOperation string    `json:"descr_operation"`
	TimeBeginning  time.Time `json:"time_begining"`
	TimeDuration   int32     `json:"time_duration"`
	Surgeon        string    `json:"surgeon"`
	DicomLink      string    `json:"dicom_link"`
}
