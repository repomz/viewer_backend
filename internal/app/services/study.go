package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/repomz/viewer_backend/internal/app/domain"
)

// StudyService is a Study service
type StudyService struct {
	repo StudyRepository
}

// NewStudyService creates a new Study service
func NewStudyService(repo StudyRepository) StudyService {
	return StudyService{
		repo: repo,
	}
}

func (s StudyService) GetStudy(ctx context.Context, id uuid.UUID) (domain.Study, error) {
	return s.repo.GetStudy(ctx, id)
}

func (s StudyService) CreateStudy(ctx context.Context, study domain.Study) (domain.Study, error) {
	return s.repo.CreateStudy(ctx, study)
}

func (s StudyService) UpdateStudyDicomLink(ctx context.Context, study domain.Study) (domain.Study, error) {
	return s.repo.UpdateStudyDicomLink(ctx, study)
}

func (s StudyService) DeleteStudy(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeleteStudy(ctx, id)
}

func (s StudyService) GetStudies(ctx context.Context, categoryIDs []int, limit, offset int) ([]domain.Study, error) {
	return s.repo.GetStudies(ctx, categoryIDs, limit, offset)
}
