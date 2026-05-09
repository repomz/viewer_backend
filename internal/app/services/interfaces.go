package services

import (
	"context"

	"github.com/repomz/viewer_backend/internal/app/domain"
)

type StudyRepository interface {
	GetStudies(ctx context.Context, categoryIDs []int, limit, offset int) ([]domain.Study, error)
	// 	GetStudy(ctx context.Context, id int) (domain.Study, error)
	// 	CreateStudy(ctx context.Context, study domain.Study) (domain.Study, error)
	// 	UpdateStudy(ctx context.Context, study domain.Study) (domain.Study, error)
	// 	DeleteStudy(ctx context.Context, id int) error
}
