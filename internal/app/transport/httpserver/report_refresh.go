package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Refresh existing documents only. Keep filenames and creation times stable so
// late/edited protocols do not create extra report cards or extend retention.
func (h HttpServer) refreshStoredReports(ctx context.Context) error {
	entries, err := os.ReadDir(reportsDirectory())
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(reportsDirectory(), entry.Name())
		body, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var document reportRequest
		if json.Unmarshal(body, &document) != nil {
			continue
		}
		start, e1 := time.ParseInLocation("02.01.2006 15:04", fmt.Sprint(document.Report["period_start"]), time.Local)
		end, e2 := time.ParseInLocation("02.01.2006 15:04", fmt.Sprint(document.Report["period_end"]), time.Local)
		if e1 != nil || e2 != nil || !end.After(start) || end.Sub(start) > 366*24*time.Hour {
			continue
		}
		updated, err := h.buildOperationsReport(ctx, start, end, int(end.Sub(start).Hours()/24))
		if err != nil {
			return err
		}
		oldJSON, _ := json.Marshal(document.Report)
		newJSON, _ := json.Marshal(updated)
		if string(oldJSON) == string(newJSON) {
			continue
		}
		document.Report = updated
		replacement, err := json.MarshalIndent(document, "", "  ")
		if err != nil {
			return err
		}
		temporary, err := os.CreateTemp(reportsDirectory(), ".refresh-*.tmp")
		if err != nil {
			return err
		}
		_, err = temporary.Write(replacement)
		if err == nil {
			err = temporary.Sync()
		}
		closeErr := temporary.Close()
		if err == nil {
			err = closeErr
		}
		// A concurrently replaced/deleted report must not be resurrected.
		reportStorageMu.Lock()
		if current, readErr := os.ReadFile(path); err == nil && readErr == nil && string(current) == string(body) {
			err = os.Rename(temporary.Name(), path)
		}
		reportStorageMu.Unlock()
		os.Remove(temporary.Name())
		if err != nil {
			return err
		}
	}
	return nil
}
