package httpserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"net/http"

	"github.com/gorilla/mux"

	"github.com/repomz/viewer_backend/internal/app/common/server"
)

type operationPlanEntry struct {
	Patient    string `json:"patient"`
	Department string `json:"department"`
	Operation  string `json:"operation"`
}

type operationPlanDay struct {
	Date    string               `json:"date"`
	Entries []operationPlanEntry `json:"entries"`
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
	response := operationPlanResponse{
		WeekStart: start.Format("2006-01-02"),
		Days:      make([]operationPlanDay, 0, 5),
	}
	for offset := 0; offset < 5; offset++ {
		date := start.AddDate(0, 0, offset).Format("2006-01-02")
		response.Days = append(response.Days, operationPlanDay{
			Date:    date,
			Entries: plan.Days[date],
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
		if entry.Patient == "" && entry.Department == "" && entry.Operation == "" {
			continue
		}
		if len(entry.Patient) > 200 ||
			len(entry.Department) > 100 ||
			len(entry.Operation) > 300 {
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
	server.RespondOK(operationPlanDay{Date: date, Entries: entries}, w, r)
}
