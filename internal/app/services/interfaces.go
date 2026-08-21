package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/repomz/viewer_backend/internal/app/domain"
)

type StudyRepository interface {
	GetAllStudies(ctx context.Context, limit, offset int) ([]domain.Study, error)
	GetProtocolStudiesSince(ctx context.Context, since time.Time, limit, offset int) ([]domain.Study, error)
	GetStudiesByFilter(ctx context.Context, filter domain.StudyFilter) ([]domain.Study, error)
	GetStudyByID(ctx context.Context, id uuid.UUID) (domain.Study, error)
	GetStudyByStudyIDAndType(ctx context.Context, studyID, studyType string) (domain.Study, error)
	GetStudyByPatient(ctx context.Context, patient domain.PatientFilter) (domain.Study, error)
	CreateStudy(ctx context.Context, study domain.Study) (domain.Study, error)
	UpdateStudyDicomLink(ctx context.Context, study domain.Study) (domain.Study, error)
	DeleteStudy(ctx context.Context, id uuid.UUID) error
	DeleteAllStudies(ctx context.Context) error
}

type AgentRecordsRepository interface {
	GetAgentIDs(ctx context.Context) ([]int32, error)
	GetAgentRecordsByAgentID(ctx context.Context, id, limit int32) ([]time.Time, error)
	GetAgentRecordsByAgentIDandStatus(ctx context.Context, id int32, status string, limit int32) ([]time.Time, error)
	CreateAgentRecord(ctx context.Context, record domain.AgentRecord) error
	DeleteAllAgentRecords(ctx context.Context, agent_id int32) error
}

type UserRequestRepository interface {
	Create(ctx context.Context, request domain.NewUserRequest) (domain.UserRequest, error)
	ClaimNext(ctx context.Context, agentID int32) (domain.UserRequest, error)
	RecordResult(ctx context.Context, result domain.UserRequestResult) (domain.UserRequest, error)
	GetByID(ctx context.Context, id uuid.UUID) (domain.UserRequest, error)
	List(ctx context.Context, userID string, agentID int32, limit int32) ([]domain.UserRequest, error)
	Delete(ctx context.Context, id uuid.UUID, userID string) error
	DeleteAll(ctx context.Context, userID string, agentID int32) error
}
