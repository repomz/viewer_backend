package pgrepo

import (
	"github.com/repomz/viewer_backend/internal/app/domain"
	"github.com/repomz/viewer_backend/internal/app/repository/db"
)

// func domainTostudy(study domain.Study) db.study {
// 	return db.study{
// 		ID:        study.ID(),
// 		CreatedAt: study.CreatedAt(),
// 		UpdatedAt: study.UpdatedAt(),
// 		StudyID:   study.StudyID(),
// 		Patient:   study.Patient(),
// 		CreatedAt: study.Age(),
// 		UpdatedAt: study.Department(),
// 		Body:      study.NameOperation(),
// 		ID:        study.Descroperation(),
// 		CreatedAt: study.TimeBeginning(),
// 		UpdatedAt: study.TimeDuration(),
// 		Body:      study.Surgeon(),
// 		Dicomlink: study.DicomLink(),
// //
// // 	}
// // }

func studyToDomain(study db.Study) (domain.Study, error) {
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
