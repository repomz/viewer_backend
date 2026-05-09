package pgrepo

import (
	"context"
	"fmt"

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
