package httpserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/repomz/viewer_backend/internal/app/common/server"
)

type reportRequest struct {
	AgentID     int32          `json:"agent_id"`
	Report      map[string]any `json:"report"`
	GeneratedAt string         `json:"generated_at"`
}

func reportsDirectory() string {
	if configured := strings.TrimSpace(os.Getenv("REPORTS_DIR")); configured != "" {
		return configured
	}
	return "reports"
}

func (h HttpServer) CreateReport(w http.ResponseWriter, r *http.Request) {
	var request reportRequest
	if err := decodeJSON(w, r, &request); err != nil {
		server.BadRequest("invalid-json", err, w, r)
		return
	}
	if request.AgentID <= 0 || request.Report == nil {
		server.BadRequest("invalid-report", fmt.Errorf("agent_id and report are required"), w, r)
		return
	}
	directory := reportsDirectory()
	if err := os.MkdirAll(directory, 0o750); err != nil {
		server.InternalError("report-storage", err, w, r)
		return
	}
	filename := fmt.Sprintf(
		"report_agent_%d_%s.json",
		request.AgentID,
		time.Now().UTC().Format("20060102_150405.000000000"),
	)
	path := filepath.Join(directory, filename)
	body, err := json.MarshalIndent(request, "", "  ")
	if err != nil {
		server.BadRequest("invalid-report", err, w, r)
		return
	}
	temporary, err := os.CreateTemp(directory, ".report-*.tmp")
	if err != nil {
		server.InternalError("report-storage", err, w, r)
		return
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err = temporary.Write(body); err == nil {
		err = temporary.Sync()
	}
	closeErr := temporary.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(temporaryPath, path)
	}
	if err != nil {
		server.InternalError("report-storage", err, w, r)
		return
	}
	server.RespondCreated(map[string]any{"filename": filename}, w, r)
}

func (h HttpServer) GetReports(w http.ResponseWriter, r *http.Request) {
	entries, err := os.ReadDir(reportsDirectory())
	if err != nil {
		if os.IsNotExist(err) {
			server.RespondOK([]any{}, w, r)
			return
		}
		server.InternalError("report-storage", err, w, r)
		return
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() > entries[j].Name() })
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	reports := make([]any, 0, min(limit, len(entries)))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		body, readErr := os.ReadFile(filepath.Join(reportsDirectory(), entry.Name()))
		if readErr != nil {
			continue
		}
		var report any
		if json.Unmarshal(body, &report) == nil {
			if object, ok := report.(map[string]any); ok {
				object["filename"] = entry.Name()
			} else {
				report = map[string]any{
					"filename": entry.Name(),
					"report":   report,
				}
			}
			reports = append(reports, report)
		}
		if len(reports) == limit {
			break
		}
	}
	server.RespondOK(reports, w, r)
}

func (h HttpServer) GetReport(w http.ResponseWriter, r *http.Request) {
	filename := mux.Vars(r)["filename"]
	if filename != filepath.Base(filename) || !strings.HasSuffix(filename, ".json") {
		server.BadRequest("invalid-report-name", fmt.Errorf("invalid report filename"), w, r)
		return
	}
	body, err := os.ReadFile(filepath.Join(reportsDirectory(), filename))
	if err != nil {
		if os.IsNotExist(err) {
			server.NotFound("report-not-found", err, w, r)
			return
		}
		server.InternalError("report-storage", err, w, r)
		return
	}
	var report any
	if err := json.Unmarshal(body, &report); err != nil {
		server.InternalError("invalid-stored-report", err, w, r)
		return
	}
	server.RespondOK(report, w, r)
}
