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

func (s StudyService) GetStudyByID(ctx context.Context, id uuid.UUID) (domain.Study, error) {
	return s.repo.GetStudyByID(ctx, id)
}

func (s StudyService) GetStudyByStudyIDAndType(
	ctx context.Context,
	studyID string,
	studyType string,
) (domain.Study, error) {
	return s.repo.GetStudyByStudyIDAndType(ctx, studyID, studyType)
}

func (s StudyService) GetStudyByPatient(ctx context.Context, patient domain.PatientFilter) (domain.Study, error) {
	return s.repo.GetStudyByPatient(ctx, patient)
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

func (s StudyService) DeleteAllStudies(ctx context.Context) error {
	return s.repo.DeleteAllStudies(ctx)
}

func (s StudyService) GetAllStudies(ctx context.Context, limit, offset int) ([]domain.Study, error) {
	return s.repo.GetAllStudies(ctx, limit, offset)
}

func (s StudyService) GetStudiesByFilter(ctx context.Context, filter domain.StudyFilter) ([]domain.Study, error) {
	return s.repo.GetStudiesByFilter(ctx, filter)
}
