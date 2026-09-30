package httpserver

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/repomz/viewer_backend/internal/app/domain"
)

const dutyBoundaryHour = 7
const dutyBoundaryMinute = 45

type reportGenerateRequest struct {
	AgentID  int32  `json:"agent_id"`
	Days     int    `json:"days"`
	DateFrom string `json:"date_from"`
	DateTo   string `json:"date_to"`
}

func defaultReportAgentID() int32 {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv("REPORT_AGENT_ID")))
	if err == nil && value > 0 {
		return int32(value)
	}
	return 2
}

func lastCompletedDutyEnd(now time.Time) time.Time {
	end := time.Date(now.Year(), now.Month(), now.Day(), dutyBoundaryHour, dutyBoundaryMinute, 0, 0, now.Location())
	if now.Before(end) {
		end = end.AddDate(0, 0, -1)
	}
	return end
}

func scheduledReportDays(now time.Time) int {
	if now.Weekday() == time.Monday {
		return 3
	}
	return 1
}

func reportPeriod(input reportGenerateRequest, now time.Time) (time.Time, time.Time, int, error) {
	if input.DateFrom != "" || input.DateTo != "" {
		from, err := parsePlanDate(input.DateFrom)
		if err != nil {
			return time.Time{}, time.Time{}, 0, fmt.Errorf("invalid date_from: %w", err)
		}
		to, err := parsePlanDate(input.DateTo)
		if err != nil {
			return time.Time{}, time.Time{}, 0, fmt.Errorf("invalid date_to: %w", err)
		}
		if to.Before(from) {
			return time.Time{}, time.Time{}, 0, fmt.Errorf("date_to must not precede date_from")
		}
		days := int(to.Sub(from).Hours()/24) + 1
		if days > 366 {
			return time.Time{}, time.Time{}, 0, fmt.Errorf("calendar range cannot exceed 366 days")
		}
		start := time.Date(from.Year(), from.Month(), from.Day(), dutyBoundaryHour, dutyBoundaryMinute, 0, 0, time.Local)
		return start, start.AddDate(0, 0, days), days, nil
	}
	if input.Days < 1 || input.Days > 7 {
		return time.Time{}, time.Time{}, 0, fmt.Errorf("days must be between 1 and 7")
	}
	end := lastCompletedDutyEnd(now)
	return end.AddDate(0, 0, -input.Days), end, input.Days, nil
}

func reportPatientKey(value string) string {
	fields := strings.Fields(strings.ToLower(strings.ReplaceAll(value, "ё", "е")))
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func reportOperation(study domain.Study) map[string]any {
	age := any("")
	if value := study.Age(); value.Valid {
		age = value.Int32
	}
	duration := any("")
	if value := study.TimeDuration(); value.Valid {
		duration = value.Int32
	}
	timeValue := ""
	if value := study.TimeBeginning(); value.Valid {
		timeValue = value.Time.In(time.Local).Format("15:04")
	}
	return map[string]any{
		"patient":        study.Patient(),
		"age":            age,
		"department":     study.Department(),
		"operation":      study.NameOperation(),
		"time_beginning": timeValue,
		"time_duration":  duration,
		"surgeon":        study.Surgeon(),
	}
}

func isProtocolStudy(study domain.Study) bool {
	typeValue := strings.ToLower(strings.TrimSpace(study.StudyType()))
	return typeValue != "xa" && typeValue != "ct"
}

func dutyDate(value time.Time) string {
	return value.In(time.Local).Add(-dutyBoundaryHour*time.Hour - dutyBoundaryMinute*time.Minute).Format("2006-01-02")
}

func (h HttpServer) buildOperationsReport(
	ctx context.Context,
	start time.Time,
	end time.Time,
	days int,
) (map[string]any, error) {
	// Read only the report window, in bounded pages, rather than the latest 5000
	// rows of a database which also contains the multi-year archive.
	studies := make([]domain.Study, 0)
	for offset := 0; ; offset += 1000 {
		if offset >= studyAnalysisMaxRows {
			return nil, fmt.Errorf("report exceeds study safety limit")
		}
		page, err := h.studyService.GetProtocolStudiesSince(ctx, start, 1000, offset)
		if err != nil {
			return nil, err
		}
		studies = append(studies, page...)
		if len(page) < 1000 {
			break
		}
	}
	operationPlanMu.RLock()
	plan, err := loadOperationPlan()
	operationPlanMu.RUnlock()
	if err != nil {
		return nil, err
	}

	sort.Slice(studies, func(i, j int) bool {
		return studies[i].TimeBeginning().Time.Before(studies[j].TimeBeginning().Time)
	})
	periodStudies := make([]domain.Study, 0)
	for _, study := range studies {
		if !isProtocolStudy(study) || !study.TimeBeginning().Valid {
			continue
		}
		value := study.TimeBeginning().Time.In(time.Local)
		if !value.Before(start) && value.Before(end) {
			periodStudies = append(periodStudies, study)
		}
	}

	planned := make([]map[string]any, 0)
	emergency := make([]map[string]any, 0)
	for _, study := range periodStudies {
		date := dutyDate(study.TimeBeginning().Time)
		isPlanned := false
		for _, entry := range plan.Days[date] {
			if planPatientMatches(entry.Patient, study.Patient()) &&
				(entry.BirthDate == "" || planBirthMatches(entry.BirthDate, study)) {
				isPlanned = true
				break
			}
		}
		if isPlanned {
			planned = append(planned, reportOperation(study))
		} else {
			emergency = append(emergency, reportOperation(study))
		}
	}

	todayDate := end.Format("2006-01-02")
	todayPlan := make([]map[string]any, 0)
	for _, entry := range plan.Days[todayDate] {
		previous := make([]map[string]any, 0)
		allProtocols := []domain.Study{}
		if !parseStudyBirthDate(entry.BirthDate).IsZero() {
			allProtocols, err = h.studyService.GetProtocolStudyCandidates(ctx, reportPatientKey(entry.Patient), time.Date(1900, 1, 1, 0, 0, 0, 0, time.Local), end, 500)
			if err != nil {
				return nil, err
			}
		}
		for _, study := range allProtocols {
			if !planPatientMatches(entry.Patient, study.Patient()) || !planBirthMatches(entry.BirthDate, study) || !study.TimeBeginning().Time.Before(end) {
				continue
			}
			previous = append(previous, map[string]any{
				"date":      study.TimeBeginning().Time.In(time.Local).Format("02.01.2006"),
				"operation": study.NameOperation(), "description": study.DescrOperation(),
				"recommendation": study.Recommendation(), "surgeon": study.Surgeon(),
			})
		}
		todayPlan = append(todayPlan, map[string]any{
			"patient": entry.Patient, "age": "", "department": entry.Department,
			"operation": entry.Operation, "previous_operations": previous,
		})
	}

	return map[string]any{
		"date": end.Format("02.01.2006"), "period_days": days,
		"period_start":  start.Format("02.01.2006 15:04"),
		"period_end":    end.Format("02.01.2006 15:04"),
		"planned_count": len(planned), "emergency_total": len(emergency),
		"today_planned_count": len(todayPlan), "planned_operations": planned,
		"emergency_operations": emergency, "today_planned_operations": todayPlan,
	}, nil
}

func (h HttpServer) generateAndStoreReport(
	ctx context.Context,
	input reportGenerateRequest,
	now time.Time,
) (reportRequest, string, error) {
	start, end, days, err := reportPeriod(input, now)
	if err != nil {
		return reportRequest{}, "", err
	}
	agentID := input.AgentID
	if agentID <= 0 {
		agentID = defaultReportAgentID()
	}
	report, err := h.buildOperationsReport(ctx, start, end, days)
	if err != nil {
		return reportRequest{}, "", err
	}
	document := reportRequest{
		AgentID: agentID, Report: report, GeneratedAt: now.UTC().Format(time.RFC3339),
	}
	filename, err := storeReport(document)
	return document, filename, err
}

func (h HttpServer) StartReportScheduler(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	lastGeneratedEnd := ""
	lastRefresh := time.Time{}
	run := func() {
		location, _ := time.LoadLocation("Asia/Tomsk")
		now := time.Now().In(location)
		cleanupExpiredReports(now)
		if now.Sub(lastRefresh) >= 5*time.Minute {
			if err := h.refreshStoredReports(ctx); err == nil {
				lastRefresh = now
			}
		}
		if now.Before(time.Date(now.Year(), now.Month(), now.Day(), dutyBoundaryHour, dutyBoundaryMinute, 0, 0, location)) {
			return
		}
		endKey := lastCompletedDutyEnd(now).Format(time.RFC3339)
		if endKey == lastGeneratedEnd {
			return
		}
		_, _, err := h.generateAndStoreReport(ctx, reportGenerateRequest{
			AgentID: defaultReportAgentID(), Days: scheduledReportDays(now),
		}, now)
		if err == nil {
			lastGeneratedEnd = endKey
		}
	}
	run()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
