package httpserver

import (
	"github.com/repomz/viewer_backend/internal/app/domain"
)

func toResponseStudy(study domain.Study) StudyResponse {
	return StudyResponse{
		ID:             study.ID(),
		CreatedAt:      study.CreatedAt(),
		UpdatedAt:      study.UpdatedAt(),
		StudyID:        study.StudyID(),
		Patient:        study.Patient(),
		Age:            study.Age(),
		Department:     study.Department(),
		NameOperation:  study.NameOperation(),
		DescrOperation: study.Descroperation(),
		TimeBeginning:  study.TimeBeginning(),
		TimeDuration:   study.TimeDuration(),
		Surgeon:        study.Surgeon(),
		DicomLink:      study.DicomLink(),
	}
}

func toDomainStudy(studyRequest StudyRequest) (domain.Study, error) {
	return domain.NewStudy(domain.NewStudyData{
		ID:             studyRequest.ID,
		CreatedAt:      studyRequest.CreatedAt,
		UpdatedAt:      studyRequest.UpdatedAt,
		StudyID:        studyRequest.StudyID,
		Patient:        studyRequest.Patient,
		Age:            studyRequest.Age,
		Department:     studyRequest.Department,
		NameOperation:  studyRequest.NameOperation,
		DescrOperation: studyRequest.DescrOperation,
		TimeBeginning:  studyRequest.TimeBeginning,
		TimeDuration:   studyRequest.TimeDuration,
		Surgeon:        studyRequest.Surgeon,
		DicomLink:      studyRequest.DicomLink,
	})
}
