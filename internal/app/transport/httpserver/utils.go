package httpserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/repomz/viewer_backend/internal/app/domain"
	"github.com/repomz/viewer_backend/internal/app/transport/httpmodels"
)

const maxRequestBodySize = 1 << 20

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("request body must contain a single JSON object")
		}
		return err
	}
	return nil
}

func parseAgentID(raw string) (int32, error) {
	value, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%w: agent_id", domain.ErrInvalidAgentID)
	}
	return int32(value), nil
}

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
		StudyType:      study.StudyType(),
		DescrOperation: study.DescrOperation(),
		TimeBeginning:  study.TimeBeginning().Time,
		TimeDuration:   study.TimeDuration().Int32,
		Surgeon:        study.Surgeon(),
		DicomLink:      study.DicomLink().String,
	}
}

func toDomainStudy(studyRequest httpmodels.StudyRequest) domain.Study {
	return domain.ResponseToDBStudy(domain.DBStudyData{
		StudyID:        studyRequest.StudyID,
		Patient:        studyRequest.Patient,
		Age:            studyRequest.Age,
		Department:     studyRequest.Department,
		NameOperation:  studyRequest.NameOperation,
		StudyType:      studyRequest.StudyType,
		DescrOperation: studyRequest.DescrOperation,
		TimeBeginning:  studyRequest.TimeBeginning,
		TimeDuration:   studyRequest.TimeDuration,
		Surgeon:        studyRequest.Surgeon,
		DicomLink:      studyRequest.DicomLink,
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
