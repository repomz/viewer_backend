package httpmodels

import "testing"

func validModalityStudyRequest() ModalityStudyRequest {
	return ModalityStudyRequest{
		StudyUID:   "1.2.3",
		Patient:    "Иванов И.И.",
		Age:        50,
		StudyDate:  "20260726",
		StudyTime:  "101500",
		Modality:   "CT",
		DicomLink:  "s3://bucket/study",
		DicomFiles: []DicomFile{{Name: "1.dcm", Size: 100, URL: "https://example/1"}},
	}
}

func TestModalityStudyValidatesDateTimeAndFiles(t *testing.T) {
	tests := []struct {
		name   string
		change func(*ModalityStudyRequest)
	}{
		{"invalid date", func(r *ModalityStudyRequest) { r.StudyDate = "26.07.2026" }},
		{"invalid time", func(r *ModalityStudyRequest) { r.StudyTime = "256100" }},
		{"invalid fractional time", func(r *ModalityStudyRequest) { r.StudyTime = "101500.bad" }},
		{"missing filename", func(r *ModalityStudyRequest) { r.DicomFiles[0].Name = "" }},
		{"negative size", func(r *ModalityStudyRequest) { r.DicomFiles[0].Size = -1 }},
		{"invalid URL", func(r *ModalityStudyRequest) { r.DicomFiles[0].URL = "file:///tmp/1.dcm" }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := validModalityStudyRequest()
			test.change(&request)
			if err := request.Validate("CT"); err == nil {
				t.Fatal("Validate() unexpectedly succeeded")
			}
		})
	}
}

func TestModalityStudyNormalizesValuesUsedAfterValidation(t *testing.T) {
	request := validModalityStudyRequest()
	request.StudyDate = " 20260726 "
	request.StudyTime = " 101500.123 "
	request.DicomFiles[0].Name = " 1.dcm "
	request.DicomFiles[0].URL = " https://example/1 "

	if err := request.Validate("CT"); err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}
	if request.StudyDate != "20260726" || request.StudyTime != "101500.123" {
		t.Fatalf("date/time were not normalized: %#v", request)
	}
	if request.DicomFiles[0].Name != "1.dcm" ||
		request.DicomFiles[0].URL != "https://example/1" {
		t.Fatalf("DICOM file was not normalized: %#v", request.DicomFiles[0])
	}
}

func TestModalityStudyAcceptsPartialDICOMTime(t *testing.T) {
	for _, value := range []string{"", "10", "1015", "101500", "101500.123"} {
		request := validModalityStudyRequest()
		request.StudyTime = value
		if err := request.Validate("CT"); err != nil {
			t.Fatalf("study_time %q rejected: %v", value, err)
		}
	}
}
