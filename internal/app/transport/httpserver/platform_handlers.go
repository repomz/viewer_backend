package httpserver

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"golang.org/x/crypto/bcrypt"
)

const (
	defaultDriveQuota = int64(1 << 30)
	sessionLifetime   = 30 * 24 * time.Hour
	maxAuthBody       = 32 << 10
)

var loginPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]{3,32}$`)

type authenticatedUser struct {
	ID          int64  `json:"id"`
	DisplayName string `json:"display_name"`
	Login       string `json:"login"`
	Role        string `json:"role"`
	QuotaBytes  int64  `json:"quota_bytes"`
	TokenHash   []byte `json:"-"`
}

type userContextKey struct{}

type loginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type authResponse struct {
	Token string            `json:"token"`
	User  authenticatedUser `json:"user"`
}

type changeCredentialsRequest struct {
	CurrentPassword string `json:"current_password"`
	NewLogin        string `json:"new_login"`
	NewPassword     string `json:"new_password"`
}

type driveFileResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	ContentType string    `json:"content_type"`
	SizeBytes   int64     `json:"size_bytes"`
	CreatedAt   time.Time `json:"created_at"`
}

type driveListResponse struct {
	Files      []driveFileResponse `json:"files"`
	UsedBytes  int64               `json:"used_bytes"`
	QuotaBytes int64               `json:"quota_bytes"`
}

func writePlatformError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func bearerToken(r *http.Request) string {
	value := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(value) < 8 || !strings.EqualFold(value[:7], "Bearer ") {
		return ""
	}
	return strings.TrimSpace(value[7:])
}

func (h HttpServer) authenticate(r *http.Request) (authenticatedUser, error) {
	if h.sqlDB == nil {
		return authenticatedUser{}, errors.New("authentication storage is unavailable")
	}
	token := bearerToken(r)
	if token == "" {
		return authenticatedUser{}, errors.New("authentication required")
	}
	hash := sha256.Sum256([]byte(token))
	var user authenticatedUser
	err := h.sqlDB.QueryRowContext(r.Context(), `
		SELECT u.id, u.display_name, u.login, u.role, u.quota_bytes
		FROM auth_sessions s
		JOIN app_users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.expires_at > NOW()
	`, hash[:]).Scan(&user.ID, &user.DisplayName, &user.Login, &user.Role, &user.QuotaBytes)
	if err != nil {
		return authenticatedUser{}, err
	}
	user.TokenHash = hash[:]
	_, _ = h.sqlDB.ExecContext(r.Context(), `UPDATE auth_sessions SET last_seen_at = NOW() WHERE token_hash = $1`, hash[:])
	return user, nil
}

func currentAuthenticatedUser(r *http.Request) (authenticatedUser, bool) {
	user, ok := r.Context().Value(userContextKey{}).(authenticatedUser)
	return user, ok
}

func (h HttpServer) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := h.authenticate(r)
		if err != nil {
			writePlatformError(w, http.StatusUnauthorized, "Требуется вход в систему")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), userContextKey{}, user)))
	}
}

func (h HttpServer) RequireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return h.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		user, _ := currentAuthenticatedUser(r)
		if user.Role != "admin" {
			writePlatformError(w, http.StatusForbidden, "Доступ разрешён только администратору")
			return
		}
		next(w, r)
	})
}

func newSessionToken() (string, []byte, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256([]byte(token))
	return token, hash[:], nil
}

func (h HttpServer) createSession(ctx context.Context, tx *sql.Tx, userID int64) (string, error) {
	token, hash, err := newSessionToken()
	if err != nil {
		return "", err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO auth_sessions (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`, userID, hash, time.Now().Add(sessionLifetime))
	return token, err
}

func (h HttpServer) Login(w http.ResponseWriter, r *http.Request) {
	if h.sqlDB == nil {
		writePlatformError(w, http.StatusServiceUnavailable, "Авторизация временно недоступна")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxAuthBody)
	var request loginRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writePlatformError(w, http.StatusBadRequest, "Некорректный запрос")
		return
	}
	request.Login = strings.TrimSpace(request.Login)
	var user authenticatedUser
	var passwordHash string
	err := h.sqlDB.QueryRowContext(r.Context(), `
		SELECT id, display_name, login, role, quota_bytes, password_hash
		FROM app_users WHERE LOWER(login) = LOWER($1)
	`, request.Login).Scan(&user.ID, &user.DisplayName, &user.Login, &user.Role, &user.QuotaBytes, &passwordHash)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(request.Password)) != nil {
		writePlatformError(w, http.StatusUnauthorized, "Неверный логин или пароль")
		return
	}
	tx, err := h.sqlDB.BeginTx(r.Context(), nil)
	if err != nil {
		writePlatformError(w, http.StatusInternalServerError, "Не удалось выполнить вход")
		return
	}
	defer tx.Rollback()
	token, err := h.createSession(r.Context(), tx, user.ID)
	if err == nil {
		_, err = tx.ExecContext(r.Context(), `DELETE FROM auth_sessions WHERE expires_at <= NOW()`)
	}
	if err != nil || tx.Commit() != nil {
		writePlatformError(w, http.StatusInternalServerError, "Не удалось выполнить вход")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(authResponse{Token: token, User: user})
}

func (h HttpServer) GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	user, _ := currentAuthenticatedUser(r)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(user)
}

func (h HttpServer) Logout(w http.ResponseWriter, r *http.Request) {
	user, _ := currentAuthenticatedUser(r)
	_, _ = h.sqlDB.ExecContext(r.Context(), `DELETE FROM auth_sessions WHERE token_hash = $1`, user.TokenHash)
	w.WriteHeader(http.StatusNoContent)
}

func (h HttpServer) ChangeCredentials(w http.ResponseWriter, r *http.Request) {
	user, _ := currentAuthenticatedUser(r)
	r.Body = http.MaxBytesReader(w, r.Body, maxAuthBody)
	var request changeCredentialsRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writePlatformError(w, http.StatusBadRequest, "Некорректный запрос")
		return
	}
	request.NewLogin = strings.TrimSpace(request.NewLogin)
	if request.NewLogin == "" && request.NewPassword == "" {
		writePlatformError(w, http.StatusBadRequest, "Укажите новый логин или пароль")
		return
	}
	if request.NewLogin != "" && !loginPattern.MatchString(request.NewLogin) {
		writePlatformError(w, http.StatusBadRequest, "Логин: 3–32 латинских символа, цифры, точка, дефис или подчёркивание")
		return
	}
	if request.NewPassword != "" && (len(request.NewPassword) < 4 || len(request.NewPassword) > 128) {
		writePlatformError(w, http.StatusBadRequest, "Пароль должен содержать от 4 до 128 символов")
		return
	}
	var currentHash string
	if err := h.sqlDB.QueryRowContext(r.Context(), `SELECT password_hash FROM app_users WHERE id = $1`, user.ID).Scan(&currentHash); err != nil ||
		bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(request.CurrentPassword)) != nil {
		writePlatformError(w, http.StatusUnauthorized, "Текущий пароль указан неверно")
		return
	}
	newHash := currentHash
	if request.NewPassword != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(request.NewPassword), bcrypt.DefaultCost)
		if err != nil {
			writePlatformError(w, http.StatusInternalServerError, "Не удалось обновить пароль")
			return
		}
		newHash = string(hash)
	}
	newLogin := user.Login
	if request.NewLogin != "" {
		newLogin = request.NewLogin
	}
	tx, err := h.sqlDB.BeginTx(r.Context(), nil)
	if err != nil {
		writePlatformError(w, http.StatusInternalServerError, "Не удалось обновить профиль")
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), `UPDATE app_users SET login = $1, password_hash = $2, updated_at = NOW() WHERE id = $3`, newLogin, newHash, user.ID); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			writePlatformError(w, http.StatusConflict, "Этот логин уже занят")
		} else {
			writePlatformError(w, http.StatusInternalServerError, "Не удалось обновить профиль")
		}
		return
	}
	if _, err = tx.ExecContext(r.Context(), `DELETE FROM auth_sessions WHERE user_id = $1`, user.ID); err != nil {
		writePlatformError(w, http.StatusInternalServerError, "Не удалось обновить профиль")
		return
	}
	token, err := h.createSession(r.Context(), tx, user.ID)
	if err != nil || tx.Commit() != nil {
		writePlatformError(w, http.StatusInternalServerError, "Не удалось обновить профиль")
		return
	}
	user.Login = newLogin
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(authResponse{Token: token, User: user})
}

func driveObjectKey(userID int64, fileID string) string {
	return "viewer-drive/" + strconv.FormatInt(userID, 10) + "/" + fileID
}

func (h HttpServer) ListDriveFiles(w http.ResponseWriter, r *http.Request) {
	user, _ := currentAuthenticatedUser(r)
	rows, err := h.sqlDB.QueryContext(r.Context(), `
		SELECT id::text, original_name, content_type, size_bytes, created_at
		FROM drive_files WHERE user_id = $1 ORDER BY created_at DESC
	`, user.ID)
	if err != nil {
		writePlatformError(w, http.StatusInternalServerError, "Не удалось получить файлы")
		return
	}
	defer rows.Close()
	files := make([]driveFileResponse, 0)
	var used int64
	for rows.Next() {
		var file driveFileResponse
		if err := rows.Scan(&file.ID, &file.Name, &file.ContentType, &file.SizeBytes, &file.CreatedAt); err != nil {
			writePlatformError(w, http.StatusInternalServerError, "Не удалось получить файлы")
			return
		}
		used += file.SizeBytes
		files = append(files, file)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(driveListResponse{Files: files, UsedBytes: used, QuotaBytes: user.QuotaBytes})
}

func (h HttpServer) UploadDriveFile(w http.ResponseWriter, r *http.Request) {
	if h.driveCloud == nil {
		writePlatformError(w, http.StatusServiceUnavailable, "Облачное хранилище не настроено")
		return
	}
	user, _ := currentAuthenticatedUser(r)
	r.Body = http.MaxBytesReader(w, r.Body, user.QuotaBytes+(2<<20))
	reader, err := r.MultipartReader()
	if err != nil {
		writePlatformError(w, http.StatusBadRequest, "Не удалось прочитать файл")
		return
	}
	var source interface {
		io.Reader
		io.Closer
	}
	var filename, contentType string
	for {
		part, nextErr := reader.NextPart()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil {
			writePlatformError(w, http.StatusBadRequest, "Не удалось прочитать файл")
			return
		}
		if part.FormName() == "file" && part.FileName() != "" {
			source = part
			filename = part.FileName()
			contentType = strings.TrimSpace(part.Header.Get("Content-Type"))
			break
		}
		_ = part.Close()
	}
	if source == nil {
		writePlatformError(w, http.StatusBadRequest, "Файл не выбран")
		return
	}
	defer source.Close()
	name := strings.TrimSpace(filepath.Base(filename))
	if name == "" || name == "." || len([]rune(name)) > 255 {
		writePlatformError(w, http.StatusBadRequest, "Некорректное имя файла")
		return
	}
	id := uuid.NewString()
	storedName := driveObjectKey(user.ID, id)
	target, err := os.CreateTemp("", ".viewer-drive-*.upload")
	var tempPath string
	if err == nil {
		tempPath = target.Name()
	}
	if err != nil {
		writePlatformError(w, http.StatusInternalServerError, "Не удалось сохранить файл")
		return
	}
	written, copyErr := io.Copy(target, io.LimitReader(source, user.QuotaBytes+1))
	closeErr := target.Close()
	if copyErr != nil || closeErr != nil || written > user.QuotaBytes {
		_ = os.Remove(tempPath)
		writePlatformError(w, http.StatusRequestEntityTooLarge, "Размер файла превышает доступное место")
		return
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	tx, err := h.sqlDB.BeginTx(r.Context(), nil)
	if err != nil {
		_ = os.Remove(tempPath)
		writePlatformError(w, http.StatusInternalServerError, "Не удалось сохранить файл")
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), `SELECT pg_advisory_xact_lock($1)`, user.ID); err != nil {
		_ = os.Remove(tempPath)
		writePlatformError(w, http.StatusInternalServerError, "Не удалось сохранить файл")
		return
	}
	var used int64
	if err = tx.QueryRowContext(r.Context(), `SELECT COALESCE(SUM(size_bytes), 0) FROM drive_files WHERE user_id = $1`, user.ID).Scan(&used); err != nil {
		_ = os.Remove(tempPath)
		writePlatformError(w, http.StatusInternalServerError, "Не удалось проверить свободное место")
		return
	}
	if used+written > user.QuotaBytes {
		_ = os.Remove(tempPath)
		writePlatformError(w, http.StatusRequestEntityTooLarge, "Недостаточно места на диске пользователя")
		return
	}
	if err = h.driveCloud.putFile(r.Context(), storedName, tempPath, contentType); err != nil {
		_ = os.Remove(tempPath)
		writePlatformError(w, http.StatusBadGateway, "Облачное хранилище временно недоступно")
		return
	}
	_ = os.Remove(tempPath)
	_, err = tx.ExecContext(r.Context(), `
		INSERT INTO drive_files (id, user_id, stored_name, original_name, content_type, size_bytes)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, id, user.ID, storedName, name, contentType, written)
	if err != nil || tx.Commit() != nil {
		_ = h.driveCloud.delete(context.Background(), storedName)
		writePlatformError(w, http.StatusInternalServerError, "Не удалось сохранить файл")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(driveFileResponse{ID: id, Name: name, ContentType: contentType, SizeBytes: written, CreatedAt: time.Now()})
}

func (h HttpServer) driveFileForUser(ctx context.Context, userID int64, fileID string) (driveFileResponse, string, error) {
	if _, err := uuid.Parse(fileID); err != nil {
		return driveFileResponse{}, "", sql.ErrNoRows
	}
	var file driveFileResponse
	var storedName string
	err := h.sqlDB.QueryRowContext(ctx, `
		SELECT id::text, original_name, content_type, size_bytes, created_at, stored_name
		FROM drive_files WHERE id = $1 AND user_id = $2
	`, fileID, userID).Scan(&file.ID, &file.Name, &file.ContentType, &file.SizeBytes, &file.CreatedAt, &storedName)
	return file, storedName, err
}

func (h HttpServer) DownloadDriveFile(w http.ResponseWriter, r *http.Request) {
	if h.driveCloud == nil {
		writePlatformError(w, http.StatusServiceUnavailable, "Облачное хранилище не настроено")
		return
	}
	user, _ := currentAuthenticatedUser(r)
	file, storedName, err := h.driveFileForUser(r.Context(), user.ID, mux.Vars(r)["file_id"])
	if err != nil {
		writePlatformError(w, http.StatusNotFound, "Файл не найден")
		return
	}
	response, cloudErr := h.driveCloud.get(r.Context(), storedName)
	if cloudErr != nil {
		writePlatformError(w, http.StatusBadGateway, "Облачное хранилище временно недоступно")
		return
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		writePlatformError(w, http.StatusNotFound, "Файл не найден")
		return
	}
	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": file.Name}))
	w.Header().Set("Content-Length", strconv.FormatInt(file.SizeBytes, 10))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = io.Copy(w, response.Body)
}

func (h HttpServer) DeleteDriveFile(w http.ResponseWriter, r *http.Request) {
	if h.driveCloud == nil {
		writePlatformError(w, http.StatusServiceUnavailable, "Облачное хранилище не настроено")
		return
	}
	user, _ := currentAuthenticatedUser(r)
	_, storedName, err := h.driveFileForUser(r.Context(), user.ID, mux.Vars(r)["file_id"])
	if err != nil {
		writePlatformError(w, http.StatusNotFound, "Файл не найден")
		return
	}
	if err := h.driveCloud.delete(r.Context(), storedName); err != nil {
		writePlatformError(w, http.StatusBadGateway, "Облачное хранилище временно недоступно")
		return
	}
	result, err := h.sqlDB.ExecContext(r.Context(), `DELETE FROM drive_files WHERE id = $1 AND user_id = $2`, mux.Vars(r)["file_id"], user.ID)
	if err != nil {
		writePlatformError(w, http.StatusInternalServerError, "Не удалось удалить файл")
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		writePlatformError(w, http.StatusNotFound, "Файл не найден")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type loginMetric struct {
	UserID      int64  `json:"user_id"`
	DisplayName string `json:"display_name"`
	Login       string `json:"login"`
	Count       int64  `json:"count"`
	Total       int64  `json:"total"`
}

type platformMetricsResponse struct {
	Date          string        `json:"date"`
	TotalLogins   int64         `json:"total_logins"`
	AllTimeLogins int64         `json:"all_time_logins"`
	Logins        []loginMetric `json:"logins"`
	ProtocolCount int64         `json:"protocol_count"`
	DiskTotal     uint64        `json:"disk_total_bytes"`
	DiskUsed      uint64        `json:"disk_used_bytes"`
	DiskFree      uint64        `json:"disk_free_bytes"`
	MemoryTotal   uint64        `json:"memory_total_bytes"`
	MemoryUsed    uint64        `json:"memory_used_bytes"`
}

func diskUsage(path string) (uint64, uint64, uint64, error) {
	if err := os.MkdirAll(path, 0750); err != nil {
		return 0, 0, 0, err
	}
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0, 0, err
	}
	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bavail * uint64(stat.Bsize)
	return total, total - free, free, nil
}

func readUintFile(path string) (uint64, bool) {
	value, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	parsed, err := strconv.ParseUint(strings.TrimSpace(string(value)), 10, 64)
	return parsed, err == nil
}

func memoryUsage() (uint64, uint64) {
	if total, ok := readUintFile("/sys/fs/cgroup/memory.max"); ok {
		if used, ok := readUintFile("/sys/fs/cgroup/memory.current"); ok {
			return total, used
		}
	}
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0
	}
	defer file.Close()
	var totalKB, availableKB uint64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		value, _ := strconv.ParseUint(fields[1], 10, 64)
		switch strings.TrimSuffix(fields[0], ":") {
		case "MemTotal":
			totalKB = value
		case "MemAvailable":
			availableKB = value
		}
	}
	return totalKB * 1024, (totalKB - availableKB) * 1024
}

func (h HttpServer) GetPlatformMetrics(w http.ResponseWriter, r *http.Request) {
	location, _ := time.LoadLocation("Asia/Tomsk")
	now := time.Now().In(location)
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	end := start.AddDate(0, 0, 1)
	rows, err := h.sqlDB.QueryContext(r.Context(), `
		SELECT u.id, u.display_name, u.login, COUNT(e.id) FILTER (WHERE e.logged_at >= $1 AND e.logged_at < $2), COUNT(e.id)
		FROM app_users u
		LEFT JOIN login_events e ON e.user_id = u.id
		WHERE u.role <> 'admin'
		GROUP BY u.id, u.display_name, u.login
		ORDER BY u.display_name
	`, start, end)
	if err != nil {
		writePlatformError(w, http.StatusInternalServerError, "Не удалось получить метрики")
		return
	}
	defer rows.Close()
	response := platformMetricsResponse{Date: start.Format("2006-01-02"), Logins: make([]loginMetric, 0)}
	for rows.Next() {
		var item loginMetric
		if err := rows.Scan(&item.UserID, &item.DisplayName, &item.Login, &item.Count, &item.Total); err != nil {
			writePlatformError(w, http.StatusInternalServerError, "Не удалось получить метрики")
			return
		}
		response.TotalLogins += item.Count
		response.AllTimeLogins += item.Total
		response.Logins = append(response.Logins, item)
	}
	if err := h.sqlDB.QueryRowContext(r.Context(), `
		SELECT COUNT(*) FROM studies WHERE NOT deleted AND LOWER(BTRIM(study_type)) NOT IN ('xa', 'ct')
	`).Scan(&response.ProtocolCount); err != nil {
		writePlatformError(w, http.StatusInternalServerError, "Не удалось получить метрики")
		return
	}
	response.DiskTotal, response.DiskUsed, response.DiskFree, _ = diskUsage("/")
	response.MemoryTotal, response.MemoryUsed = memoryUsage()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(response)
}

func (u authenticatedUser) String() string {
	return fmt.Sprintf("%s (%s)", u.DisplayName, u.Login)
}
