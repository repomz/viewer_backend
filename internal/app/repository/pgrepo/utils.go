package pgrepo

import (
	"github.com/repomz/viewer_backend/internal/app/domain"
	"github.com/repomz/viewer_backend/internal/app/repository/db"
	"github.com/repomz/viewer_backend/internal/app/repository/models"
)

func domainToStudy(study domain.Study) models.Study {
	return models.Study{
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

func domainToStudyParams(study domain.Study) db.CreateStudyParams {
	return db.CreateStudyParams{
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

func domainToDicomLinkParams(study domain.Study) db.UpdateDicomLinkParams {
	return db.UpdateDicomLinkParams{
		ID:        study.ID(),
		DicomLink: study.DicomLink(),
	}
}

func studyToDomain(study models.Study) (domain.Study, error) {
	return domain.NewStudy(domain.NewStudyData{
		ID:             study.ID,
		CreatedAt:      study.CreatedAt,
		UpdatedAt:      study.UpdatedAt,
		StudyID:        study.StudyID,
		Patient:        study.Patient,
		Age:            study.Age,
		Department:     study.Department,
		NameOperation:  study.NameOperation,
		DescrOperation: study.DescrOperation,
		TimeBeginning:  study.TimeBeginning,
		TimeDuration:   study.TimeDuration,
		Surgeon:        study.Surgeon,
		DicomLink:      study.DicomLink,
	})
}
