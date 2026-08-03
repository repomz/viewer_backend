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

func reportIdentity(agentID int32, report map[string]any) string {
	periodStart := strings.TrimSpace(fmt.Sprint(report["period_start"]))
	periodEnd := strings.TrimSpace(fmt.Sprint(report["period_end"]))
	if periodStart != "" && periodStart != "<nil>" && periodEnd != "" && periodEnd != "<nil>" {
		return fmt.Sprintf("%d|%s|%s", agentID, periodStart, periodEnd)
	}
	date := strings.TrimSpace(fmt.Sprint(report["date"]))
	if date != "" && date != "<nil>" {
		return fmt.Sprintf("%d|%s", agentID, date)
	}
	return ""
}

func storedReportIdentity(body []byte) string {
	var stored reportRequest
	if json.Unmarshal(body, &stored) != nil {
		return ""
	}
	return reportIdentity(stored.AgentID, stored.Report)
}

func storedReportAgentID(report any) int {
	object, ok := report.(map[string]any)
	if !ok {
		return 0
	}
	value, _ := strconv.Atoi(strings.TrimSuffix(fmt.Sprint(object["agent_id"]), ".0"))
	return value
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
	if request.AgentID <= 0 || len(request.Report) == 0 {
		server.BadRequest("invalid-report", fmt.Errorf("agent_id and report are required"), w, r)
		return
	}
	if _, err := time.Parse(time.RFC3339, request.GeneratedAt); err != nil {
		server.BadRequest(
			"invalid-report",
			fmt.Errorf("generated_at must be RFC3339: %w", err),
			w,
			r,
		)
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
	identity := reportIdentity(request.AgentID, request.Report)
	replacedReports := make([]string, 0)
	if identity != "" {
		entries, _ := os.ReadDir(directory)
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			stored, readErr := os.ReadFile(filepath.Join(directory, entry.Name()))
			if readErr == nil && storedReportIdentity(stored) == identity {
				replacedReports = append(replacedReports, entry.Name())
			}
		}
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
	for _, replaced := range replacedReports {
		if replaced != filename {
			_ = os.Remove(filepath.Join(directory, replaced))
		}
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
	agentID, _ := strconv.Atoi(r.URL.Query().Get("agent_id"))
	reports := make([]any, 0, min(limit, len(entries)))
	seen := make(map[string]bool)
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
				identity := storedReportIdentity(body)
				if identity != "" && seen[identity] {
					continue
				}
				if identity != "" {
					seen[identity] = true
				}
				object["filename"] = entry.Name()
			} else {
				report = map[string]any{
					"filename": entry.Name(),
					"report":   report,
				}
			}
			if agentID > 0 && storedReportAgentID(report) != agentID {
				continue
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

func (h HttpServer) DeleteReport(w http.ResponseWriter, r *http.Request) {
	filename := mux.Vars(r)["filename"]
	if filename != filepath.Base(filename) || !strings.HasSuffix(filename, ".json") {
		server.BadRequest("invalid-report-name", fmt.Errorf("invalid report filename"), w, r)
		return
	}
	if err := os.Remove(filepath.Join(reportsDirectory(), filename)); err != nil {
		if os.IsNotExist(err) {
			server.NotFound("report-not-found", err, w, r)
			return
		}
		server.InternalError("report-storage", err, w, r)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
