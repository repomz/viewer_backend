package httpserver

import (
	"strconv"

	"github.com/repomz/viewer_backend/internal/app/domain"
	"github.com/repomz/viewer_backend/internal/app/transport/httpmodels"
)

func toDomainAgentRecord(agentRequest httpmodels.AgentRecordRequest) (domain.AgentRecord, error) {
	agentIDint, err := strconv.Atoi(agentRequest.AgentID)
	if err != nil {
		return domain.AgentRecord{}, err
	}
	agentID := int32(agentIDint)

	return domain.RequestToDBAgentRecord(domain.DBAgentRecordData{
		AgentID: agentID,
		Status:  agentRequest.Status,
	}), nil
}

func toResponseStudy(study domain.Study) httpmodels.StudyResponse {
	return httpmodels.StudyResponse{
		ID:             study.ID(),
		CreatedAt:      study.CreatedAt(),
		UpdatedAt:      study.UpdatedAt(),
		StudyID:        study.StudyID(),
		Patient:        study.Patient(),
		Age:            study.Age().Int32,
		Department:     study.Department(),
		NameOperation:  study.NameOperation(),
		DescrOperation: study.Descroperation(),
		TimeBeginning:  study.TimeBeginning().Time,
		TimeDuration:   study.TimeDuration().Int32,
		Surgeon:        study.Surgeon(),
		DicomLink:      study.DicomLink().String,
	}
}

func toDomainStudy(studyRequest httpmodels.StudyRequest) domain.Study {
	return domain.ResponseToDBStudy(domain.DBStudyData{
		ID:             studyRequest.ID,
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
		CreatedAt:      studyRequest.CreatedAt,
		UpdatedAt:      studyRequest.UpdatedAt,
	})
}

func toDomainPatientFilter(patientRequest httpmodels.PatientFilter) domain.PatientFilter {
	return domain.PatientFilter{
		Patient: patientRequest.Patient,
	}
}

func toDomainStudyFilter(filterRequest httpmodels.StudyFilter) domain.StudyFilter {
	return domain.StudyFilter{
		StudyDate: filterRequest.StudyDate,
		StudyType: filterRequest.StudyType,
		Surgeon:   filterRequest.Surgeon,
	}
}
