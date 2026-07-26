package httpmodels

import (
	"fmt"
	"strings"

	"github.com/repomz/viewer_backend/internal/app/domain"
)

type DicomFile struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	URL  string `json:"url"`
}

type ModalityStudyRequest struct {
	StudyUID    string      `json:"study_uid"`
	Patient     string      `json:"patient"`
	Age         int32       `json:"age"`
	StudyDate   string      `json:"study_date"`
	StudyTime   string      `json:"study_time"`
	Description string      `json:"description"`
	Modality    string      `json:"modality"`
	DicomLink   string      `json:"dicom_link"`
	DicomFiles  []DicomFile `json:"dicom_files"`
}

func (r *ModalityStudyRequest) Validate(expectedModality string) error {
	r.StudyUID = strings.TrimSpace(r.StudyUID)
	r.Patient = strings.TrimSpace(r.Patient)
	r.Description = strings.TrimSpace(r.Description)
	r.Modality = strings.ToUpper(strings.TrimSpace(r.Modality))
	r.DicomLink = strings.TrimSpace(r.DicomLink)
	if r.StudyUID == "" {
		return fmt.Errorf("%w: study_uid", domain.ErrRequired)
	}
	if r.Patient == "" {
		return fmt.Errorf("%w: patient", domain.ErrRequired)
	}
	if r.Modality != expectedModality {
		return fmt.Errorf("%w: modality must be %s", domain.ErrInvalidRequest, expectedModality)
	}
	if r.DicomLink == "" {
		return fmt.Errorf("%w: dicom_link", domain.ErrRequired)
	}
	if len(r.DicomFiles) == 0 {
		return fmt.Errorf("%w: dicom_files", domain.ErrRequired)
	}
	for _, file := range r.DicomFiles {
		if strings.TrimSpace(file.URL) == "" {
			return fmt.Errorf("%w: dicom_files.url", domain.ErrRequired)
		}
	}
	return nil
}
