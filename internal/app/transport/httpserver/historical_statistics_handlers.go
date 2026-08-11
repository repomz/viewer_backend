package httpserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/repomz/viewer_backend/internal/app/common/server"
)

type historicalStatisticsYear struct {
	Year   int            `json:"year"`
	Counts map[string]int `json:"counts"`
	Total  int            `json:"total"`
}

type historicalStatisticsDocument struct {
	SchemaVersion  int                        `json:"schema_version"`
	Source         string                     `json:"source"`
	StartYear      int                        `json:"start_year"`
	EndYear        int                        `json:"end_year"`
	GeneratedAt    time.Time                  `json:"generated_at"`
	OperationTypes []string                   `json:"operation_types"`
	Years          []historicalStatisticsYear `json:"years"`
}

var historicalStatisticsMu sync.RWMutex

func historicalStatisticsPath() string {
	directory := strings.TrimSpace(os.Getenv("PLANS_DIR"))
	if directory == "" {
		directory = strings.TrimSpace(os.Getenv("REPORTS_DIR"))
	}
	if directory == "" {
		directory = "/app/reports"
	}
	return filepath.Join(directory, "operation-history-statistics.json")
}

func saveHistoricalStatistics(document historicalStatisticsDocument) error {
	path := historicalStatisticsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".operation-history-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(document); err != nil {
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

func loadHistoricalStatistics() (historicalStatisticsDocument, error) {
	file, err := os.Open(historicalStatisticsPath())
	if err != nil {
		return historicalStatisticsDocument{}, err
	}
	defer file.Close()
	var document historicalStatisticsDocument
	if err := json.NewDecoder(file).Decode(&document); err != nil {
		return historicalStatisticsDocument{}, err
	}
	return document, nil
}

func normalizeHistoricalStatistics(document *historicalStatisticsDocument) error {
	if document.SchemaVersion != 2 {
		return errors.New("schema_version 2 is required")
	}
	currentYear := time.Now().Year()
	if document.StartYear < 1900 || document.StartYear > currentYear+1 {
		return errors.New("start_year is outside the supported range")
	}
	if document.EndYear < document.StartYear || document.EndYear > currentYear+1 {
		return errors.New("end_year is outside the supported range")
	}
	document.Source = strings.TrimSpace(document.Source)
	if document.Source == "" {
		document.Source = "hospital-archive"
	}
	if document.GeneratedAt.IsZero() {
		document.GeneratedAt = time.Now().UTC()
	}
	seenTypes := make(map[string]bool)
	operationTypes := make([]string, 0, len(document.OperationTypes))
	for _, operationType := range document.OperationTypes {
		operationType = strings.TrimSpace(operationType)
		if operationType == "" || seenTypes[operationType] {
			continue
		}
		seenTypes[operationType] = true
		operationTypes = append(operationTypes, operationType)
	}
	document.OperationTypes = operationTypes
	if len(document.OperationTypes) == 0 || len(document.OperationTypes) > 100 || len(document.Years) > 150 {
		return errors.New("historical statistics has an invalid size")
	}
	allowedTypes := stringSet(document.OperationTypes)
	seenYears := make(map[int]bool)
	for index := range document.Years {
		row := &document.Years[index]
		if row.Year < document.StartYear || row.Year > document.EndYear || seenYears[row.Year] {
			return fmt.Errorf("invalid or duplicate statistics year: %d", row.Year)
		}
		seenYears[row.Year] = true
		if row.Counts == nil {
			row.Counts = make(map[string]int)
		}
		row.Total = 0
		for operationType, count := range row.Counts {
			if !allowedTypes[operationType] || count < 0 {
				return fmt.Errorf("invalid historical operation count: %s", operationType)
			}
			row.Total += count
		}
	}
	sort.Slice(document.Years, func(i, j int) bool { return document.Years[i].Year < document.Years[j].Year })
	return nil
}

func (h HttpServer) GetHistoricalStatistics(w http.ResponseWriter, r *http.Request) {
	historicalStatisticsMu.RLock()
	document, err := loadHistoricalStatistics()
	historicalStatisticsMu.RUnlock()
	if errors.Is(err, os.ErrNotExist) {
		server.RespondOK(historicalStatisticsDocument{
			SchemaVersion: 2, OperationTypes: []string{}, Years: []historicalStatisticsYear{},
		}, w, r)
		return
	}
	if err != nil {
		server.InternalError("historical-statistics", err, w, r)
		return
	}
	server.RespondOK(document, w, r)
}

func (h HttpServer) PutHistoricalStatistics(w http.ResponseWriter, r *http.Request) {
	var document historicalStatisticsDocument
	if err := decodeJSON(w, r, &document); err != nil {
		server.BadRequest("invalid-json", err, w, r)
		return
	}
	if err := normalizeHistoricalStatistics(&document); err != nil {
		server.BadRequest("invalid-historical-statistics", err, w, r)
		return
	}
	historicalStatisticsMu.Lock()
	err := saveHistoricalStatistics(document)
	historicalStatisticsMu.Unlock()
	if err != nil {
		server.InternalError("historical-statistics", err, w, r)
		return
	}
	server.RespondOK(document, w, r)
}
