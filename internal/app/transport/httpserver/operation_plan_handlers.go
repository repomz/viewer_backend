package httpserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"net/http"

	"github.com/gorilla/mux"

	"github.com/repomz/viewer_backend/internal/app/common/server"
	"github.com/repomz/viewer_backend/internal/app/domain"
	"github.com/repomz/viewer_backend/internal/app/transport/httpmodels"
)

type operationPlanEntry struct {
	Patient    string `json:"patient"`
	Department string `json:"department"`
	Operation  string `json:"operation"`
	Additions  string `json:"additions,omitempty"`
}

type operationPlanResponseEntry struct {
	operationPlanEntry
	PreviousOperation *httpmodels.StudyResponse `json:"previous_operation,omitempty"`
}

type operationPlanDay struct {
	Date    string                       `json:"date"`
	Entries []operationPlanResponseEntry `json:"entries"`
}

type operationPlanResponse struct {
	WeekStart string             `json:"week_start"`
	Days      []operationPlanDay `json:"days"`
}

type operationPlanFile struct {
	Days map[string][]operationPlanEntry `json:"days"`
}

var operationPlanMu sync.RWMutex

func operationPlanPath() string {
	directory := strings.TrimSpace(os.Getenv("PLANS_DIR"))
	if directory == "" {
		directory = strings.TrimSpace(os.Getenv("REPORTS_DIR"))
	}
	if directory == "" {
		directory = "/app/reports"
	}
	return filepath.Join(directory, "operation-plan.json")
}

func loadOperationPlan() (operationPlanFile, error) {
	result := operationPlanFile{Days: make(map[string][]operationPlanEntry)}
	file, err := os.Open(operationPlanPath())
	if errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return operationPlanFile{}, err
	}
	defer file.Close()
	if err := json.NewDecoder(file).Decode(&result); err != nil {
		return operationPlanFile{}, err
	}
	if result.Days == nil {
		result.Days = make(map[string][]operationPlanEntry)
	}
	return result, nil
}

func saveOperationPlan(plan operationPlanFile) error {
	path := operationPlanPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".operation-plan-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(plan); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Chmod(0o640); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func monday(date time.Time) time.Time {
	weekday := (int(date.Weekday()) + 6) % 7
	return date.AddDate(0, 0, -weekday)
}

func parsePlanDate(value string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", value, time.Local)
}

func normalizePlanPatient(value string) string {
	value = strings.ToLower(strings.ReplaceAll(value, "ё", "е"))
	value = strings.NewReplacer(".", " ", ",", " ", ";", " ", "-", " ").Replace(value)
	return strings.Join(strings.Fields(value), " ")
}

func planPatientMatches(planPatient, protocolPatient string) bool {
	planValue := normalizePlanPatient(planPatient)
	protocolValue := normalizePlanPatient(protocolPatient)
	if planValue == "" || protocolValue == "" {
		return false
	}
	planParts := strings.Fields(planValue)
	protocolParts := strings.Fields(protocolValue)
	if len(planParts) == 1 {
		return len(protocolParts) > 0 && planParts[0] == protocolParts[0]
	}
	return planValue == protocolValue
}

func latestPlanProtocol(entry operationPlanEntry, studies []domain.Study, year int) *httpmodels.StudyResponse {
	var latest *domain.Study
	for index := range studies {
		study := &studies[index]
		if !isProtocolStudy(*study) || !planPatientMatches(entry.Patient, study.Patient()) {
			continue
		}
		beginning := study.TimeBeginning()
		if !beginning.Valid || beginning.Time.In(time.Local).Year() != year {
			continue
		}
		if latest == nil || beginning.Time.After(latest.TimeBeginning().Time) {
			latest = study
		}
	}
	if latest == nil {
		return nil
	}
	response := toResponseStudy(*latest)
	return &response
}

func responsePlanEntries(entries []operationPlanEntry, studies []domain.Study, year int) []operationPlanResponseEntry {
	entries = append([]operationPlanEntry(nil), entries...)
	sortOperationPlanEntries(entries)
	result := make([]operationPlanResponseEntry, 0, len(entries))
	for _, entry := range entries {
		result = append(result, operationPlanResponseEntry{
			operationPlanEntry: entry,
			PreviousOperation:  latestPlanProtocol(entry, studies, year),
		})
	}
	return result
}

func operationPlanDepartmentRank(value string) int {
	value = strings.ToLower(strings.TrimSpace(value))
	if strings.HasPrefix(value, "кардио") {
		return 0
	}
	if value == "сосуды" {
		return 1
	}
	if value == "рсц" || value == "неврология" || value == "нейро/х" {
		return 2
	}
	return 3
}

func sortOperationPlanEntries(entries []operationPlanEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		leftRank := operationPlanDepartmentRank(entries[i].Department)
		rightRank := operationPlanDepartmentRank(entries[j].Department)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		leftDepartment := strings.ToLower(entries[i].Department)
		rightDepartment := strings.ToLower(entries[j].Department)
		if leftDepartment != rightDepartment {
			return leftDepartment < rightDepartment
		}
		return strings.ToLower(entries[i].Patient) < strings.ToLower(entries[j].Patient)
	})
}

func (h HttpServer) GetOperationPlan(w http.ResponseWriter, r *http.Request) {
	start := monday(time.Now())
	if value := strings.TrimSpace(r.URL.Query().Get("week_start")); value != "" {
		parsed, err := parsePlanDate(value)
		if err != nil {
			server.BadRequest("invalid-week-start", err, w, r)
			return
		}
		start = monday(parsed)
	}
	operationPlanMu.RLock()
	plan, err := loadOperationPlan()
	operationPlanMu.RUnlock()
	if err != nil {
		server.RespondWithError(fmt.Errorf("load operation plan: %w", err), w, r)
		return
	}
	studies := []domain.Study{}
	if h.studyService != nil {
		studies, err = loadStudiesForAnalysis(r.Context(), h.studyService)
		if err != nil {
			server.RespondWithError(fmt.Errorf("load protocols for operation plan: %w", err), w, r)
			return
		}
	}
	currentYear := time.Now().In(time.Local).Year()
	response := operationPlanResponse{
		WeekStart: start.Format("2006-01-02"),
		Days:      make([]operationPlanDay, 0, 5),
	}
	for offset := 0; offset < 5; offset++ {
		date := start.AddDate(0, 0, offset).Format("2006-01-02")
		entries := plan.Days[date]
		if entries == nil {
			entries = make([]operationPlanEntry, 0)
		}
		response.Days = append(response.Days, operationPlanDay{
			Date:    date,
			Entries: responsePlanEntries(entries, studies, currentYear),
		})
	}
	server.RespondOK(response, w, r)
}

func (h HttpServer) PutOperationPlanDay(w http.ResponseWriter, r *http.Request) {
	date := mux.Vars(r)["date"]
	if _, err := parsePlanDate(date); err != nil {
		server.BadRequest("invalid-plan-date", err, w, r)
		return
	}
	var request struct {
		Entries []operationPlanEntry `json:"entries"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		server.BadRequest("invalid-json", err, w, r)
		return
	}
	if len(request.Entries) > 20 {
		server.BadRequest(
			"too-many-plan-entries",
			errors.New("a plan day cannot contain more than 20 entries"),
			w,
			r,
		)
		return
	}
	entries := make([]operationPlanEntry, 0, len(request.Entries))
	for _, entry := range request.Entries {
		entry.Patient = strings.TrimSpace(entry.Patient)
		entry.Department = strings.TrimSpace(entry.Department)
		entry.Operation = strings.TrimSpace(entry.Operation)
		entry.Additions = strings.TrimSpace(entry.Additions)
		if entry.Patient == "" && entry.Department == "" && entry.Operation == "" && entry.Additions == "" {
			continue
		}
		if len(entry.Patient) > 200 ||
			len(entry.Department) > 100 ||
			len(entry.Operation) > 300 ||
			len(entry.Additions) > 1000 {
			server.BadRequest(
				"invalid-plan-entry",
				errors.New("operation plan entry is too long"),
				w,
				r,
			)
			return
		}
		entries = append(entries, entry)
	}
	sortOperationPlanEntries(entries)
	operationPlanMu.Lock()
	plan, err := loadOperationPlan()
	if err == nil {
		if len(entries) == 0 {
			delete(plan.Days, date)
		} else {
			plan.Days[date] = entries
		}
		err = saveOperationPlan(plan)
	}
	operationPlanMu.Unlock()
	if err != nil {
		server.RespondWithError(fmt.Errorf("save operation plan: %w", err), w, r)
		return
	}
	server.RespondOK(operationPlanDay{Date: date, Entries: responsePlanEntries(entries, nil, time.Now().Year())}, w, r)
}
