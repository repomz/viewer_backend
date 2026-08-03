package httpserver

import (
	"context"
	"fmt"

	"github.com/repomz/viewer_backend/internal/app/domain"
)

const (
	studyAnalysisPageSize = 1000
	studyAnalysisMaxRows  = 100000
)

// loadStudiesForAnalysis reads every available study in bounded pages so yearly
// plan/statistics calculations are not silently truncated by a UI page limit.
func loadStudiesForAnalysis(ctx context.Context, service StudyService) ([]domain.Study, error) {
	result := make([]domain.Study, 0, studyAnalysisPageSize)
	for offset := 0; offset < studyAnalysisMaxRows; offset += studyAnalysisPageSize {
		page, err := service.GetAllStudies(ctx, studyAnalysisPageSize, offset)
		if err != nil {
			return nil, err
		}
		result = append(result, page...)
		if len(page) < studyAnalysisPageSize {
			return result, nil
		}
	}
	return nil, fmt.Errorf("study analysis exceeds safety limit of %d rows", studyAnalysisMaxRows)
}
