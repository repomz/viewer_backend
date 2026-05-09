package httpserver

import (
	"context"

	"github.com/repomz/viewer_backend/internal/app/domain"
)

// StudyService is a study service
type StudyService interface {
	GetStudies(ctx context.Context, categoryIDs []int, limit, offset int) ([]domain.Study, error)
	// GetStudy(ctx context.Context, id int) (domain.Study, error)
	// CreateStudy(ctx context.Context, study domain.Study) (domain.Study, error)
	// UpdateStudy(ctx context.Context, study domain.Study) (domain.Study, error)
	// DeleteStudy(ctx context.Context, id int) error
}
