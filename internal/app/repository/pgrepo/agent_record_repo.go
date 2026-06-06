package pgrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/repomz/viewer_backend/internal/app/db"
	"github.com/repomz/viewer_backend/internal/app/domain"
)

type AgentRecordRepo struct {
	query *db.Queries
}

func NewAgentRecordRepo(qr *db.Queries) *AgentRecordRepo {
	return &AgentRecordRepo{
		query: qr,
	}
}

func (s StudyRepo) DeleteAllAgentReecords(ctx context.Context) error {

	err := s.query.SoftDeleteAllStudies(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete studies: %w", err)
	}

	return nil
}

func (s StudyRepo) CreateAgentRecord(ctx context.Context, study domain.Study) error {

	studyParams := domainToDBStudyParams(study)

	insertedStudy, err := s.query.CreateStudy(ctx, studyParams)
	if err != nil {
		return domain.Study{}, fmt.Errorf("failed to insert a book: %w", err)
	}

	domainBook, err := dbStudyToDomain(insertedStudy)
	if err != nil {
		return domain.Study{}, fmt.Errorf("failed to create domain book: %w", err)
	}

	return domainBook, nil

}

func (s StudyRepo) GetAgentRecordByAgentIDandStaus(ctx context.Context, id int16, status string) (domain.AgentRecord, error) {

	if id == uuid.Nil {
		return domain.Study{}, fmt.Errorf("%w: id", domain.ErrRequired)
	}

	study, err := s.query.GetStudyByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Study{}, domain.ErrNotFound
		}
		return domain.Study{}, fmt.Errorf("failed to get a book: %w", err)
	}

	domainStudy, err := dbStudyToDomain(study)
	if err != nil {
		return domain.Study{}, fmt.Errorf("failed to create domain book: %w", err)
	}

	return domainStudy, nil
}
func (s StudyRepo) GetAgentRecordByAgentID(ctx context.Context, id int16) (domain.AgentRecord, error) {

	if id == uuid.Nil {
		return domain.Study{}, fmt.Errorf("%w: id", domain.ErrRequired)
	}

	study, err := s.query.GetStudyByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Study{}, domain.ErrNotFound
		}
		return domain.Study{}, fmt.Errorf("failed to get a book: %w", err)
	}

	domainStudy, err := dbStudyToDomain(study)
	if err != nil {
		return domain.Study{}, fmt.Errorf("failed to create domain book: %w", err)
	}

	return domainStudy, nil
}
