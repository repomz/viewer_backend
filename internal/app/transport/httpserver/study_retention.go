package httpserver

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/repomz/viewer_backend/internal/app/domain"
)

const (
	studyRetentionPageSize = 1000
	studyRetentionHour     = 9
)

type studyRetentionResult struct {
	Scanned int
	Deleted int
}

// WarmCurrentXACache restores only XA studies that belong to the active
// clinical retention window. Older cloud archives remain cold until opened.
func (h HttpServer) WarmCurrentXACache(ctx context.Context) {
	if h.studyService == nil || h.xaCache == nil {
		return
	}
	now := time.Now()
	for offset := 0; ; offset += studyRetentionPageSize {
		page, err := h.studyService.GetAllStudies(ctx, studyRetentionPageSize, offset)
		if err != nil {
			log.Printf("XA hot cache warmup skipped: %v", err)
			return
		}
		for _, study := range page {
			if strings.EqualFold(strings.TrimSpace(study.StudyType()), "xa") &&
				!shouldDeleteArchivedXAStudy(study, now) {
				h.xaCache.HydrateArchived(study.StudyID())
			}
		}
		if len(page) < studyRetentionPageSize {
			return
		}
	}
}

// StartStudyRetention periodically removes expired local XA media. Protocol
// metadata stays in PostgreSQL so the archive search can address every year;
// the regular /studies endpoint applies the current-week window itself.
func (h HttpServer) StartStudyRetention(ctx context.Context) {
	lastRunDate := ""
	runOnce := func(now time.Time) {
		date := now.Format("2006-01-02")
		if date != lastRunDate && h.runStudyRetentionIfDue(ctx, now) {
			lastRunDate = date
		}
	}
	runOnce(time.Now())
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			runOnce(now)
		}
	}
}

func (h HttpServer) runStudyRetentionIfDue(ctx context.Context, now time.Time) bool {
	// Sunday still belongs to the current clinical week. Cleanup starts on
	// Monday morning; on later weekdays it also runs to recover from downtime.
	if !studyRetentionIsDue(now) {
		return false
	}
	result, err := h.runStudyRetention(ctx, now)
	if err != nil {
		log.Printf("study retention failed: %v", err)
		return false
	}
	if result.Deleted > 0 {
		log.Printf(
			"study retention completed: scanned=%d deleted=%d",
			result.Scanned,
			result.Deleted,
		)
	}
	return true
}

func studyRetentionIsDue(now time.Time) bool {
	return now.Weekday() != time.Sunday &&
		(now.Weekday() != time.Monday || now.Hour() >= studyRetentionHour)
}

func (h HttpServer) runStudyRetention(
	ctx context.Context,
	now time.Time,
) (studyRetentionResult, error) {
	if h.studyService == nil {
		return studyRetentionResult{}, nil
	}
	studies := make([]domain.Study, 0)
	for offset := 0; ; offset += studyRetentionPageSize {
		page, err := h.studyService.GetAllStudies(
			ctx,
			studyRetentionPageSize,
			offset,
		)
		if err != nil {
			return studyRetentionResult{}, fmt.Errorf("list studies: %w", err)
		}
		studies = append(studies, page...)
		if len(page) < studyRetentionPageSize {
			break
		}
	}

	result := studyRetentionResult{Scanned: len(studies)}
	for _, study := range studies {
		modality := strings.ToLower(strings.TrimSpace(study.StudyType()))
		if modality == "xa" {
			if shouldDeleteArchivedXAStudy(study, now) && h.xaCache != nil && h.xaCache.cloudArchived(study.StudyID()) {
				_ = h.xaCache.removeLocalStudy(study.StudyID())
			}
		}
	}
	return result, nil
}

func shouldDeleteArchivedXAStudy(study domain.Study, now time.Time) bool {
	if strings.ToLower(strings.TrimSpace(study.StudyType())) != "xa" {
		return false
	}
	start := study.TimeBeginning()
	if !start.Valid {
		return false
	}
	operationDate := beginningOfDay(start.Time.In(now.Location()))
	currentMonday := beginningOfDay(monday(now.In(now.Location())))
	if !operationDate.Before(currentMonday) {
		return false
	}
	previousMonday := currentMonday.AddDate(0, 0, -7)
	if !operationDate.Before(previousMonday) &&
		(operationDate.Weekday() == time.Friday ||
			operationDate.Weekday() == time.Saturday ||
			operationDate.Weekday() == time.Sunday) {
		return false
	}
	return true
}

func shouldDeleteProtocolStudy(
	study domain.Study,
	plan operationPlanFile,
	now time.Time,
) bool {
	modality := strings.ToLower(strings.TrimSpace(study.StudyType()))
	if modality == "xa" || modality == "ct" {
		return false
	}
	start := study.TimeBeginning()
	if !start.Valid {
		return false
	}
	location := now.Location()
	operationDate := beginningOfDay(start.Time.In(location))
	currentMonday := beginningOfDay(monday(now.In(location)))
	if !operationDate.Before(currentMonday) {
		return false
	}

	previousMonday := currentMonday.AddDate(0, 0, -7)
	if operationDate.Before(previousMonday) {
		return true
	}

	switch operationDate.Weekday() {
	case time.Monday, time.Tuesday, time.Wednesday, time.Thursday:
		return true
	case time.Friday:
		return operationIsPlanned(study.Patient(), operationDate, plan)
	case time.Saturday, time.Sunday:
		return false
	default:
		return false
	}
}

func operationIsPlanned(
	patient string,
	operationDate time.Time,
	plan operationPlanFile,
) bool {
	surname := normalizedSurname(patient)
	if surname == "" {
		return false
	}
	for _, entry := range plan.Days[operationDate.Format("2006-01-02")] {
		if normalizedSurname(entry.Patient) == surname {
			return true
		}
	}
	return false
}

func normalizedSurname(patient string) string {
	fields := strings.Fields(strings.ToLower(strings.ReplaceAll(patient, "ё", "е")))
	if len(fields) == 0 {
		return ""
	}
	return strings.Trim(fields[0], ".,;:()[]{}")
}

func beginningOfDay(value time.Time) time.Time {
	return time.Date(
		value.Year(),
		value.Month(),
		value.Day(),
		0,
		0,
		0,
		0,
		value.Location(),
	)
}
