package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/repomz/viewer_backend/internal/app/domain"
)

type StudyRepository interface {
	GetStudies(ctx context.Context, categoryIDs []int, limit, offset int) ([]domain.Study, error)
	GetStudy(ctx context.Context, id uuid.UUID) (domain.Study, error)
	GetStudyByPatient(ctx context.Context, patient domain.PatientFilter) (domain.Study, error)
	CreateStudy(ctx context.Context, study domain.Study) (domain.Study, error)
	UpdateStudyDicomLink(ctx context.Context, study domain.Study) (domain.Study, error)
	DeleteStudy(ctx context.Context, id uuid.UUID) error
	DeleteAllStudies(ctx context.Context, id uuid.UUID) error
}
