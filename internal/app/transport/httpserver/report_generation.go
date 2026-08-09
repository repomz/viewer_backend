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

const dutyBoundaryHour = 8

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
	end := time.Date(now.Year(), now.Month(), now.Day(), dutyBoundaryHour, 0, 0, 0, now.Location())
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
		start := time.Date(from.Year(), from.Month(), from.Day(), dutyBoundaryHour, 0, 0, 0, time.Local)
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
	return value.In(time.Local).Add(-dutyBoundaryHour * time.Hour).Format("2006-01-02")
}

func (h HttpServer) buildOperationsReport(
	ctx context.Context,
	start time.Time,
	end time.Time,
	days int,
) (map[string]any, error) {
	studies, err := h.studyService.GetAllStudies(ctx, 5000, 0)
	if err != nil {
		return nil, err
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
	allProtocols := make([]domain.Study, 0)
	for _, study := range studies {
		if !isProtocolStudy(study) || !study.TimeBeginning().Valid {
			continue
		}
		allProtocols = append(allProtocols, study)
		value := study.TimeBeginning().Time.In(time.Local)
		if !value.Before(start) && value.Before(end) {
			periodStudies = append(periodStudies, study)
		}
	}

	plannedByDate := make(map[string]map[string]operationPlanEntry)
	for date := start; date.Before(end); date = date.AddDate(0, 0, 1) {
		entries := make(map[string]operationPlanEntry)
		for _, entry := range plan.Days[date.Format("2006-01-02")] {
			if key := reportPatientKey(entry.Patient); key != "" {
				entries[key] = entry
			}
		}
		plannedByDate[date.Format("2006-01-02")] = entries
	}

	performedPlanned := make(map[string]domain.Study)
	emergency := make([]map[string]any, 0)
	for _, study := range periodStudies {
		key := reportPatientKey(study.Patient())
		date := dutyDate(study.TimeBeginning().Time)
		if _, planned := plannedByDate[date][key]; planned {
			performedPlanned[date+"|"+key] = study
		} else {
			emergency = append(emergency, reportOperation(study))
		}
	}

	planned := make([]map[string]any, 0)
	for date := start; date.Before(end); date = date.AddDate(0, 0, 1) {
		dateKey := date.Format("2006-01-02")
		for _, entry := range plan.Days[dateKey] {
			key := reportPatientKey(entry.Patient)
			if study, ok := performedPlanned[dateKey+"|"+key]; ok {
				planned = append(planned, reportOperation(study))
				continue
			}
			planned = append(planned, map[string]any{
				"patient": entry.Patient, "age": "", "department": entry.Department,
				"operation": entry.Operation, "time_beginning": "", "time_duration": "", "surgeon": "",
			})
		}
	}

	todayDate := end.Format("2006-01-02")
	todayPlan := make([]map[string]any, 0)
	for _, entry := range plan.Days[todayDate] {
		previous := make([]map[string]any, 0)
		key := reportPatientKey(entry.Patient)
		for _, study := range allProtocols {
			if reportPatientKey(study.Patient()) != key || !study.TimeBeginning().Time.Before(end) {
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
	run := func() {
		now := time.Now()
		if now.Hour() < dutyBoundaryHour {
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
