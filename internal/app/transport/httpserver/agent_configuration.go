package httpserver

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/repomz/viewer_backend/internal/app/common/server"
	"github.com/repomz/viewer_backend/internal/app/domain"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Only operational options are stored; credentials and storage keys are excluded.
func safeAgentConfiguration(raw json.RawMessage) (json.RawMessage, error) {
	var config struct {
		Version             string          `json:"version"`
		RemoteConfiguration bool            `json:"remote_configuration"`
		Description         string          `json:"description"`
		LogDir              string          `json:"log_dir"`
		StateFile           string          `json:"state_file"`
		PACSConfigPath      string          `json:"pacs_config_path"`
		Heartbeat           float64         `json:"heartbeat_interval_min"`
		Study               pollingSnapshot `json:"study_polling"`
		XA                  pollingSnapshot `json:"xa_polling"`
		CT                  pollingSnapshot `json:"ct_polling"`
		Requests            pollingSnapshot `json:"user_requests_polling"`
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

type editableAgentConfiguration struct {
	Description string          `json:"description"`
	Heartbeat   float64         `json:"heartbeat_interval_min"`
	Study       pollingSnapshot `json:"study_polling"`
	XA          pollingSnapshot `json:"xa_polling"`
	CT          pollingSnapshot `json:"ct_polling"`
	Requests    pollingSnapshot `json:"user_requests_polling"`
}

func (c editableAgentConfiguration) validate() error {
	if len(c.Description) > 200 || c.Heartbeat < 1 || c.Heartbeat > 60 {
		return fmt.Errorf("heartbeat: от 1 до 60 минут; описание: до 200 символов")
	}
	for _, p := range []pollingSnapshot{c.Study, c.XA, c.CT, c.Requests} {
		if p.Interval < 0.25 || p.Interval > 1440 {
			return fmt.Errorf("интервал: от 0,25 до 1440 минут")
		}
	}
	if !c.Requests.State {
		return fmt.Errorf("опрос команд должен оставаться включённым")
	}
	if len(c.Study.Directories) > 20 || (c.Study.State && len(c.Study.Directories) == 0) {
		return fmt.Errorf("укажите папки протоколов (до 20)")
	}
	for _, d := range c.Study.Directories {
		if strings.TrimSpace(d) == "" || len(d) > 1024 {
			return fmt.Errorf("некорректный путь к протоколам")
		}
	}
	if len(c.XA.Directories) > 0 || len(c.CT.Directories) > 0 || len(c.Requests.Directories) > 0 {
		return fmt.Errorf("папки доступны только для протоколов")
	}
	return nil
}

func (h HttpServer) UpdateAgentConfiguration(w http.ResponseWriter, r *http.Request) {
	user, ok := currentAuthenticatedUser(r)
	if !ok || user.Role != "admin" {
		writePlatformError(w, 403, "Только администратор")
		return
	}
	id, err := strconv.Atoi(mux.Vars(r)["agent_id"])
	if err != nil || id <= 0 || id > 2147483647 {
		writePlatformError(w, 400, "Некорректный агент")
		return
	}
	var config editableAgentConfiguration
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		writePlatformError(w, 400, "Некорректные настройки")
		return
	}
	if err := config.validate(); err != nil {
		writePlatformError(w, 400, err.Error())
		return
	}
	var supported bool
	if err := h.sqlDB.QueryRowContext(r.Context(), `SELECT COALESCE(configuration->>'remote_configuration','false')='true' FROM agent_configuration WHERE agent_id=$1`, id).Scan(&supported); err != nil {
		writePlatformError(w, 400, "Агент ещё не передал конфигурацию")
		return
	}
	if !supported {
		writePlatformError(w, 409, "Для удалённой настройки обновите агента до версии 0.2.16 или новее")
		return
	}
	// Serialize edits per agent, including concurrent administrator sessions.
	tx, err := h.sqlDB.BeginTx(r.Context(), nil)
	if err != nil {
		writePlatformError(w, 500, "Не удалось отправить настройки")
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), `SELECT pg_advisory_xact_lock(71931, $1)`, id); err != nil {
		writePlatformError(w, 500, "Не удалось заблокировать настройки")
		return
	}
	var pending bool
	if err = tx.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM user_requests WHERE agent_id=$1 AND command='update_agent_config' AND status IN ('pending','in_progress'))`, id).Scan(&pending); err != nil {
		writePlatformError(w, 500, "Не удалось проверить очередь")
		return
	}
	if pending {
		writePlatformError(w, 409, "Предыдущие настройки ещё ожидают применения агентом")
		return
	}
	payload, _ := json.Marshal(map[string]any{"configuration": config})
	created, err := h.userRequestService.Create(r.Context(), domain.NewUserRequest{UserID: strconv.FormatInt(user.ID, 10), AgentID: int32(id), RequestType: "agent_configuration", Command: "update_agent_config", Payload: payload, MaxAttempts: 3})
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	server.RespondCreated(toUserRequestResponse(created), w, r)
}

func (h HttpServer) GetAgentConfigurations(w http.ResponseWriter, r *http.Request) {
	rows, err := h.sqlDB.QueryContext(r.Context(), `SELECT agent_id, configuration, received_at, COALESCE((SELECT id::text FROM user_requests u WHERE u.agent_id=a.agent_id AND u.command='update_agent_config' AND u.status IN ('pending','in_progress') ORDER BY created_at DESC LIMIT 1),'') FROM agent_configuration a ORDER BY agent_id`)
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
		var pendingID string
		if err := rows.Scan(&id, &config, &receivedAt, &pendingID); err != nil {
			writePlatformError(w, 500, "Не удалось прочитать настройки")
			return
		}
		result = append(result, map[string]any{"agent_id": id, "configuration": config, "received_at": receivedAt, "pending_request_id": pendingID})
	}
	if rows.Err() != nil {
		writePlatformError(w, 500, "Не удалось прочитать настройки")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}
