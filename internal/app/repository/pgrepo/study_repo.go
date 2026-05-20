package pgrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/repomz/viewer_backend/internal/app/domain"
	"github.com/repomz/viewer_backend/internal/app/repository/db"
)

type StudyRepo struct {
	query *db.Queries
}

func NewStudyRepo(qr *db.Queries) *StudyRepo {
	return &StudyRepo{
		query: qr,
	}
}

func (s StudyRepo) GetStudies(ctx context.Context, categoryIDs []int, limit, offset int) ([]domain.Study, error) {

	studies, err := s.query.GetStudies(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get studys: %w", err)
	}

	domainStudies := make([]domain.Study, len(studies))
	for i, study := range studies {
		domainStudy, err := studyToDomain(study)
		if err != nil {
			return nil, fmt.Errorf("failed to create domain study: %w", err)
		}

		domainStudies[i] = domainStudy
	}

	return domainStudies, nil
}

func (s StudyRepo) GetStudiesByFilter(ctx context.Context, filter domain.StudyFilter) ([]domain.Study, error) {

	studies, err := s.query.GetStudies(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get studys: %w", err)
	}

	domainStudies := make([]domain.Study, len(studies))
	for i, study := range studies {
		domainStudy, err := studyToDomain(study)
		if err != nil {
			return nil, fmt.Errorf("failed to create domain study: %w", err)
		}

		domainStudies[i] = domainStudy
	}

	return domainStudies, nil
}

func (s StudyRepo) GetStudy(ctx context.Context, id uuid.UUID) (domain.Study, error) {

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

	domainStudy, err := studyToDomain(study)
	if err != nil {
		return domain.Study{}, fmt.Errorf("failed to create domain book: %w", err)
	}

	return domainStudy, nil
}

func (s StudyRepo) GetStudyByPatient(ctx context.Context, patient domain.PatientFilter) (domain.Study, error) {

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

	domainStudy, err := studyToDomain(study)
	if err != nil {
		return domain.Study{}, fmt.Errorf("failed to create domain book: %w", err)
	}

	return domainStudy, nil
}

func (s StudyRepo) CreateStudy(ctx context.Context, study domain.Study) (domain.Study, error) {

	studyParams := domainToStudyParams(study)

	insertedStudy, err := s.query.CreateStudy(ctx, studyParams)
	if err != nil {
		return domain.Study{}, fmt.Errorf("failed to insert a book: %w", err)
	}

	domainBook, err := studyToDomain(insertedStudy)
	if err != nil {
		return domain.Study{}, fmt.Errorf("failed to create domain book: %w", err)
	}

	return domainBook, nil

}

func (s StudyRepo) UpdateStudyDicomLink(ctx context.Context, study domain.Study) (domain.Study, error) {

	dicomLinkParams := domainToDicomLinkParams(study)

	updatedStudy, err := s.query.UpdateStudyDicomLink(ctx, dicomLinkParams)
	if err != nil {
		return domain.Study{}, fmt.Errorf("failed to update a book: %w", err)
	}

	domainBook, err := studyToDomain(updatedStudy)
	if err != nil {
		return domain.Study{}, fmt.Errorf("failed to create domain book: %w", err)
	}

	return domainBook, nil
}

func (s StudyRepo) DeleteStudy(ctx context.Context, id uuid.UUID) error {

	if id == uuid.Nil {
		return fmt.Errorf("%w: id", domain.ErrRequired)
	}

	err := s.query.DeleteStudy(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to delete a study: %w", err)
	}

	return nil
}
