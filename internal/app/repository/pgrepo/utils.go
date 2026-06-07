package pgrepo

import (
	"database/sql"
	"time"

	"github.com/repomz/viewer_backend/internal/app/db"
	"github.com/repomz/viewer_backend/internal/app/domain"
)

// func domainToDBStudy(study domain.Study) db.Study {
// 	return db.Study{
// 		ID:             study.ID(),
// 		CreatedAt:      study.CreatedAt(),
// 		UpdatedAt:      study.UpdatedAt(),
// 		StudyID:        study.StudyID(),
// 		Patient:        study.Patient(),
// 		Age:            study.Age(),
// 		Department:     study.Department(),
// 		NameOperation:  study.NameOperation(),
// 		DescrOperation: study.Descroperation(),
// 		TimeBeginning:  study.TimeBeginning(),
// 		TimeDuration:   study.TimeDuration(),
// 		Surgeon:        study.Surgeon(),
// 		DicomLink:      study.DicomLink(),
// 	}
// }

func domainToDBagentRecordParams(agent domain.AgentRecord) db.CreateAgentRecordParams {
	return db.CreateAgentRecordParams{
		AgentID: agent.AgentID(),
		Status:  agent.Status(),
	}
}

func domainToDBStudyParams(study domain.Study) db.CreateStudyParams {
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

func domainToDBDicomLinkParams(study domain.Study) db.UpdateStudyDicomLinkParams {
	return db.UpdateStudyDicomLinkParams{
		ID:        study.ID(),
		DicomLink: study.DicomLink(),
	}
}

// func dbStudyToDomain(study db.Study) (domain.Study, error) {
// 	return domain.NewStudy(domain.NewStudyData{
// 		ID:             study.ID,
// 		CreatedAt:      study.CreatedAt,
// 		UpdatedAt:      study.UpdatedAt,
// 		StudyID:        study.StudyID,
// 		Patient:        study.Patient,
// 		Age:            sql.NullInt32.Int32,
// 		Department:     study.Department,
// 		NameOperation:  study.NameOperation,
// 		DescrOperation: study.DescrOperation,
// 		TimeBeginning:  study.TimeBeginning,
// 		TimeDuration:   study.TimeDuration,
// 		Surgeon:        study.Surgeon,
// 		DicomLink:      study.DicomLink,
// 	})
// }

func dbStudyToDomain(study db.Study) (domain.Study, error) {
	return domain.DBToNewStudy(study)
}

// Извлекает время из указателя и создаем объект NullTime заранее, чтобы не дублировать в кейсах
func sqlNullTimefromFilter(filter domain.StudyFilter) sql.NullTime {
	var studyTime time.Time
	if filter.StudyDate != nil {
		studyTime = *filter.StudyDate
	}
	sqlStudyTime := sql.NullTime{
		Time:  studyTime,
		Valid: filter.StudyDate != nil,
	}

	return sqlStudyTime
}
