package httpmodels

import (
	"fmt"
	"net/url"
	"strings"
	"time"

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
	r.StudyDate = strings.TrimSpace(r.StudyDate)
	r.StudyTime = strings.TrimSpace(r.StudyTime)
	r.Description = strings.TrimSpace(r.Description)
	r.Modality = strings.ToUpper(strings.TrimSpace(r.Modality))
	r.DicomLink = strings.TrimSpace(r.DicomLink)
	if r.StudyUID == "" {
		return fmt.Errorf("%w: study_uid", domain.ErrRequired)
	}
	if r.Patient == "" {
		return fmt.Errorf("%w: patient", domain.ErrRequired)
	}
	if r.Age < 0 {
		return fmt.Errorf("%w: age", domain.ErrNegative)
	}
	if _, err := time.Parse("20060102", r.StudyDate); err != nil {
		return fmt.Errorf(
			"%w: study_date must use YYYYMMDD",
			domain.ErrInvalidRequest,
		)
	}
	if err := validateDICOMTime(r.StudyTime); err != nil {
		return err
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
	for index := range r.DicomFiles {
		file := &r.DicomFiles[index]
		file.Name = strings.TrimSpace(file.Name)
		file.URL = strings.TrimSpace(file.URL)
		if file.Name == "" {
			return fmt.Errorf("%w: dicom_files.name", domain.ErrRequired)
		}
		if file.Size <= 0 {
			return fmt.Errorf("%w: dicom_files.size must be positive", domain.ErrInvalidRequest)
		}
		parsedURL, err := url.Parse(file.URL)
		if err != nil || parsedURL.Host == "" ||
			(parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
			return fmt.Errorf("%w: dicom_files.url", domain.ErrInvalidRequest)
		}
	}
	return nil
}

func validateDICOMTime(value string) error {
	parts := strings.SplitN(value, ".", 2)
	value = parts[0]
	if value == "" {
		return nil
	}
	if len(parts) == 2 {
		fraction := parts[1]
		if len(fraction) == 0 || len(fraction) > 6 {
			return fmt.Errorf(
				"%w: study_time fractional seconds must contain 1 to 6 digits",
				domain.ErrInvalidRequest,
			)
		}
		for _, digit := range fraction {
			if digit < '0' || digit > '9' {
				return fmt.Errorf(
					"%w: study_time fractional seconds must contain only digits",
					domain.ErrInvalidRequest,
				)
			}
		}
	}
	if len(value) != 2 && len(value) != 4 && len(value) != 6 {
		return fmt.Errorf(
			"%w: study_time must use HH, HHMM or HHMMSS",
			domain.ErrInvalidRequest,
		)
	}
	value += strings.Repeat("0", 6-len(value))
	if _, err := time.Parse("150405", value); err != nil {
		return fmt.Errorf(
			"%w: study_time must be a valid DICOM time",
			domain.ErrInvalidRequest,
		)
	}
	return nil
}
