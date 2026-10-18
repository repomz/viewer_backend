package httpserver

import (
	"bytes"
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/repomz/viewer_backend/internal/app/common/server"
	"github.com/repomz/viewer_backend/internal/app/domain"
)

type agentFilePayload struct {
	Path  string `json:"path"`
	Token string `json:"upload_token"`
	Owner int64  `json:"drive_owner_id"`
}
type agentFileIDKey struct{}

func protocolFilename(value string) string {
	value = strings.ReplaceAll(strings.TrimSpace(value), `\`, "/")
	name := path.Base(value)
	switch strings.ToLower(path.Ext(name)) {
	case ".docx", ".doc", ".rtf", ".pdf":
		if !strings.ContainsAny(value, "\x00\r\n") && len(value) <= 4096 {
			return name
		}
	}
	return ""
}

func (h HttpServer) RequestAgentFile(w http.ResponseWriter, r *http.Request) {
	user, ok := currentAuthenticatedUser(r)
	if !ok || user.Role != "admin" {
		writePlatformError(w, 403, "Только администратор")
		return
	}
	var input struct {
		AgentID int32  `json:"agent_id"`
		Path    string `json:"path"`
	}
	if decodeJSON(w, r, &input) != nil || input.AgentID <= 0 || protocolFilename(input.Path) == "" {
		writePlatformError(w, 400, "Укажите агента и полный путь к DOCX, DOC, RTF или PDF")
		return
	}
	var supported bool
	if h.sqlDB == nil {
		writePlatformError(w, 503, "База недоступна")
		return
	}
	if err := h.sqlDB.QueryRowContext(r.Context(), `SELECT COALESCE(configuration->>'remote_protocol_files','false')='true' FROM agent_configuration WHERE agent_id=$1`, input.AgentID).Scan(&supported); err != nil || !supported {
		writePlatformError(w, 409, "Обновите агента до версии с отправкой исходных протоколов и дождитесь его подключения")
		return
	}
	token, _, err := newSessionToken()
	if err != nil {
		writePlatformError(w, 500, "Не удалось создать запрос")
		return
	}
	tx, err := h.sqlDB.BeginTx(r.Context(), nil)
	if err != nil {
		writePlatformError(w, 500, "Не удалось создать запрос")
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), `SELECT pg_advisory_xact_lock(71932,$1)`, input.AgentID); err != nil {
		writePlatformError(w, 500, "Не удалось проверить очередь")
		return
	}
	var existingID uuid.UUID
	err = tx.QueryRowContext(r.Context(), `SELECT id FROM user_requests WHERE command='send_protocol_file' AND agent_id=$1 AND user_id=$2 AND payload->>'path'=$3 AND status IN ('pending','in_progress') ORDER BY created_at DESC LIMIT 1`, input.AgentID, strconv.FormatInt(user.ID, 10), strings.TrimSpace(input.Path)).Scan(&existingID)
	if err == nil {
		existing, loadErr := h.userRequestService.GetByID(r.Context(), existingID)
		if loadErr != nil {
			server.RespondWithError(loadErr, w, r)
			return
		}
		server.RespondOK(toUserRequestResponse(existing), w, r)
		return
	}
	if err != sql.ErrNoRows {
		writePlatformError(w, 500, "Не удалось проверить очередь")
		return
	}
	payload, _ := json.Marshal(agentFilePayload{strings.TrimSpace(input.Path), token, user.ID})
	created, err := h.userRequestService.Create(r.Context(), domain.NewUserRequest{
		UserID: strconv.FormatInt(user.ID, 10), AgentID: input.AgentID, RequestType: "protocol_file",
		Command: "send_protocol_file", Payload: payload, MaxAttempts: 3,
	})
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	if err = tx.Commit(); err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	server.RespondCreated(toUserRequestResponse(created), w, r)
}

func (h HttpServer) ListAgentFileRequests(w http.ResponseWriter, r *http.Request) {
	user, ok := currentAuthenticatedUser(r)
	if !ok || user.Role != "admin" {
		writePlatformError(w, 403, "Только администратор")
		return
	}
	id, err := parseAgentID(r.URL.Query().Get("agent_id"))
	if err != nil {
		writePlatformError(w, 400, "Некорректный агент")
		return
	}
	jobs, err := h.userRequestService.List(r.Context(), strconv.FormatInt(user.ID, 10), id, 100)
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	items := []any{}
	for _, job := range jobs {
		if job.Command == "send_protocol_file" {
			items = append(items, toUserRequestResponse(job))
		}
	}
	server.RespondOK(items, w, r)
}

// A request-scoped capability permits only one original document into the requesting
// administrator's drive. Agents never receive an administrator session or S3 credentials.
func (h HttpServer) ReceiveAgentFile(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(mux.Vars(r)["request_id"])
	if err != nil {
		writePlatformError(w, 400, "Некорректный запрос")
		return
	}
	var input struct {
		Token   string `json:"token"`
		Content string `json:"content"`
	}
	if decodeJSONLimit(w, r, &input, 28<<20) != nil {
		writePlatformError(w, 400, "Некорректный файл")
		return
	}
	job, err := h.userRequestService.GetByID(r.Context(), id)
	var payload agentFilePayload
	if err != nil || job.Command != "send_protocol_file" || job.Status != "in_progress" || time.Since(job.CreatedAt) > 24*time.Hour || json.Unmarshal(job.Payload, &payload) != nil || payload.Token == "" || subtle.ConstantTimeCompare([]byte(input.Token), []byte(payload.Token)) != 1 {
		writePlatformError(w, 403, "Запрос файла недействителен")
		return
	}
	content, err := base64.StdEncoding.DecodeString(input.Content)
	if err != nil || len(content) > 20<<20 || protocolFilename(payload.Path) == "" {
		writePlatformError(w, 400, "Некорректный файл")
		return
	}
	var user authenticatedUser
	err = h.sqlDB.QueryRowContext(r.Context(), `SELECT id, role, quota_bytes FROM app_users WHERE id=$1 AND role='admin'`, payload.Owner).Scan(&user.ID, &user.Role, &user.QuotaBytes)
	if err != nil {
		writePlatformError(w, 403, "Диск администратора недоступен")
		return
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", protocolFilename(payload.Path))
	if err != nil {
		writePlatformError(w, 400, "Некорректное имя файла")
		return
	}
	_, _ = part.Write(content)
	_ = writer.Close()
	ctx := context.WithValue(r.Context(), userContextKey{}, user)
	ctx = context.WithValue(ctx, agentFileIDKey{}, id.String())
	upload := r.Clone(ctx)
	upload.Body = io.NopCloser(bytes.NewReader(body.Bytes()))
	upload.Header = r.Header.Clone()
	upload.Header.Set("Content-Type", writer.FormDataContentType())
	h.UploadDriveFile(w, upload)
}
