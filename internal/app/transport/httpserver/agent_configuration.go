package httpserver

import (
	"encoding/json"
	"net/http"
	"time"
)

// Only operational options are stored; credentials and storage keys are excluded.
func safeAgentConfiguration(raw json.RawMessage) (json.RawMessage, error) {
	var config struct {
		Version        string          `json:"version"`
		Description    string          `json:"description"`
		LogDir         string          `json:"log_dir"`
		StateFile      string          `json:"state_file"`
		PACSConfigPath string          `json:"pacs_config_path"`
		Heartbeat      float64         `json:"heartbeat_interval_min"`
		Study          pollingSnapshot `json:"study_polling"`
		XA             pollingSnapshot `json:"xa_polling"`
		CT             pollingSnapshot `json:"ct_polling"`
		Requests       pollingSnapshot `json:"user_requests_polling"`
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		return nil, err
	}
	return json.Marshal(config)
}

type pollingSnapshot struct {
	State       bool     `json:"state"`
	Interval    float64  `json:"interval_min"`
	Directories []string `json:"operations_dir,omitempty"`
}

func (h HttpServer) GetAgentConfigurations(w http.ResponseWriter, r *http.Request) {
	rows, err := h.sqlDB.QueryContext(r.Context(), `SELECT agent_id, configuration, received_at FROM agent_configuration ORDER BY agent_id`)
	if err != nil {
		writePlatformError(w, 500, "Не удалось получить настройки агентов")
		return
	}
	defer rows.Close()
	result := []map[string]any{}
	for rows.Next() {
		var id int
		var config json.RawMessage
		var receivedAt time.Time
		if err := rows.Scan(&id, &config, &receivedAt); err != nil {
			writePlatformError(w, 500, "Не удалось прочитать настройки")
			return
		}
		result = append(result, map[string]any{"agent_id": id, "configuration": config, "received_at": receivedAt})
	}
	if rows.Err() != nil {
		writePlatformError(w, 500, "Не удалось прочитать настройки")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}
