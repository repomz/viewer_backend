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

type StudyRepo struct {
	query *db.Queries
}

func NewStudyRepo(qr *db.Queries) *StudyRepo {
	return &StudyRepo{
		query: qr,
	}
}

func (s StudyRepo) GetAllStudies(ctx context.Context, limit, offset int) ([]domain.Study, error) {

	studies, err := s.query.GetStudies(ctx, db.GetStudiesParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get studies: %w", err)
	}

	domainStudies := make([]domain.Study, len(studies))
	for i, study := range studies {
		domainStudy, err := dbStudyToDomain(study)
		if err != nil {
			return nil, fmt.Errorf("failed to create domain study: %w", err)
		}

		domainStudies[i] = domainStudy
	}

	return domainStudies, nil
}

func (s StudyRepo) GetStudiesByFilter(ctx context.Context, filter domain.StudyFilter) ([]domain.Study, error) {

	var studies []db.Study
	var err error

	// Извлекаем время из указателя и создаем объект NullTime заранее, чтобы не дублировать в кейсах
	sqlStudyTime := sqlNullTimefromFilter(filter)

	switch {
	case filter.HasAll():

		studies, err = s.query.GetStudiesByDateSurgeonStudyType(ctx, db.GetStudiesByDateSurgeonStudyTypeParams{
			TimeBeginning: sqlStudyTime,
			Surgeon:       filter.Surgeon,
			StudyType:     filter.StudyType,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get studies: %w", err)
		}

	case filter.HasDateAndSurgeon():
		studies, err = s.query.GetStudiesByDateAndSurgeon(ctx, db.GetStudiesByDateAndSurgeonParams{
			TimeBeginning: sqlStudyTime,
			Surgeon:       filter.Surgeon,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get studies: %w", err)
		}

	case filter.HasDateAndType():
		studies, err = s.query.GetStudiesByDateAndStudyType(ctx, db.GetStudiesByDateAndStudyTypeParams{
			TimeBeginning: sqlStudyTime,
			StudyType:     filter.StudyType,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get studies: %w", err)
		}

	case filter.HasSurgeonAndType():
		studies, err = s.query.GetStudiesBySurgeonAndStudyType(ctx, db.GetStudiesBySurgeonAndStudyTypeParams{
			Surgeon:   filter.Surgeon,
			StudyType: filter.StudyType,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get studies: %w", err)
		}

	case filter.HasOnlyDate():
		studies, err = s.query.GetStudiesByDate(ctx, sqlStudyTime)
		if err != nil {
			return nil, fmt.Errorf("failed to get studies: %w", err)
		}

	case filter.HasOnlySurgeon():
		studies, err = s.query.GetStudiesBySurgeon(ctx, filter.Surgeon)
		if err != nil {
			return nil, fmt.Errorf("failed to get studies: %w", err)
		}
	case filter.HasOnlyType():
		studies, err = s.query.GetStudiesByStudyType(ctx, filter.StudyType)
		if err != nil {
			return nil, fmt.Errorf("failed to get studies: %w", err)
		}

	default:
		studies, err = s.query.GetStudies(ctx, db.GetStudiesParams{Limit: 100})
		if err != nil {
			return nil, fmt.Errorf("failed to get studies: %w", err)
		}
	}

	domainStudies := make([]domain.Study, len(studies))
	for i, study := range studies {
		domainStudy, err := dbStudyToDomain(study)
		if err != nil {
			return nil, fmt.Errorf("failed to create domain study: %w", err)
		}

		domainStudies[i] = domainStudy
	}

	return domainStudies, nil
}

func (s StudyRepo) GetStudyByID(ctx context.Context, id uuid.UUID) (domain.Study, error) {

	if id == uuid.Nil {
		return domain.Study{}, fmt.Errorf("%w: id", domain.ErrRequired)
	}

	study, err := s.query.GetStudyByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Study{}, domain.ErrNotFound
		}
		return domain.Study{}, fmt.Errorf("failed to get a study: %w", err)
	}

	domainStudy, err := dbStudyToDomain(study)
	if err != nil {
		return domain.Study{}, fmt.Errorf("failed to create domain study: %w", err)
	}

	return domainStudy, nil
}

func (s StudyRepo) GetStudyByStudyIDAndType(
	ctx context.Context,
	studyID string,
	studyType string,
) (domain.Study, error) {
	study, err := s.query.GetStudyByStudyIDAndType(
		ctx,
		db.GetStudyByStudyIDAndTypeParams{StudyID: studyID, StudyType: studyType},
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Study{}, domain.ErrNotFound
		}
		return domain.Study{}, fmt.Errorf("get study by external ID and type: %w", err)
	}
	domainStudy, err := dbStudyToDomain(study)
	if err != nil {
		return domain.Study{}, fmt.Errorf("create domain study: %w", err)
	}
	return domainStudy, nil
}

func (s StudyRepo) GetStudyByPatient(ctx context.Context, patient domain.PatientFilter) (domain.Study, error) {

	if patient.Patient == "" {
		return domain.Study{}, fmt.Errorf("%w: patient", domain.ErrRequired)
	}

	study, err := s.query.GetStudyByPatient(ctx, patient.Patient)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Study{}, domain.ErrNotFound
		}
		return domain.Study{}, fmt.Errorf("failed to get a study: %w", err)
	}

	domainStudy, err := dbStudyToDomain(study)
	if err != nil {
		return domain.Study{}, fmt.Errorf("failed to create domain study: %w", err)
	}

	return domainStudy, nil
}

func (s StudyRepo) CreateStudy(ctx context.Context, study domain.Study) (domain.Study, error) {

	studyParams := domainToDBStudyParams(study)

	insertedStudy, err := s.query.CreateStudy(ctx, studyParams)
	if err != nil {
		return domain.Study{}, fmt.Errorf("failed to insert a study: %w", err)
	}

	domainStudy, err := dbStudyToDomain(insertedStudy)
	if err != nil {
		return domain.Study{}, fmt.Errorf("failed to create domain study: %w", err)
	}

	return domainStudy, nil

}

func (s StudyRepo) UpdateStudyDicomLink(ctx context.Context, study domain.Study) (domain.Study, error) {

	dicomLinkParams := domainToDBDicomLinkParams(study)

	updatedStudy, err := s.query.UpdateStudyDicomLink(ctx, dicomLinkParams)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Study{}, domain.ErrNotFound
		}
		return domain.Study{}, fmt.Errorf("failed to update a study: %w", err)
	}

	domainStudy, err := dbStudyToDomain(updatedStudy)
	if err != nil {
		return domain.Study{}, fmt.Errorf("failed to create domain study: %w", err)
	}

	return domainStudy, nil
}

func (s StudyRepo) DeleteStudy(ctx context.Context, id uuid.UUID) error {

	if id == uuid.Nil {
		return fmt.Errorf("%w: id", domain.ErrRequired)
	}

	err := s.query.SoftDeleteStudy(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to delete a study: %w", err)
	}

	return nil
}

func (s StudyRepo) DeleteAllStudies(ctx context.Context) error {

	err := s.query.SoftDeleteAllStudies(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete studies: %w", err)
	}

	return nil
}
