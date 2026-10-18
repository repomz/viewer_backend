package httpserver

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"github.com/repomz/viewer_backend/internal/app/domain"
)

func TestAgentFileDeliveryIsScopedAndIdempotent(t *testing.T) {
	h := sourceTestServer(t)
	_, err := h.sqlDB.Exec(`CREATE TABLE app_users (id bigint PRIMARY KEY, role text, quota_bytes bigint);
 INSERT INTO app_users VALUES (1,'admin',1048576),(2,'user',1048576);
 CREATE TABLE drive_files(id uuid PRIMARY KEY,user_id bigint,stored_name text,original_name text,content_type text,size_bytes bigint,created_at timestamptz DEFAULT NOW());`)
	if err != nil {
		t.Fatal(err)
	}
	puts := 0
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PUT" {
			puts++
			body, _ := io.ReadAll(r.Body)
			if string(body) != "original" {
				t.Error("original changed")
			}
		}
		w.Header().Set("Content-Length", strconv.Itoa(len("original")))
		w.WriteHeader(200)
	}))
	defer cloud.Close()
	h.driveCloud = &yandexArchive{endpoint: cloud.URL, bucket: "test", accessKey: "test", secretKey: "test", region: "test", client: cloud.Client()}
	id := uuid.New()
	stub := &userRequestServiceStub{request: domain.UserRequest{ID: id, Command: "send_protocol_file", Status: "in_progress", CreatedAt: time.Now(), Payload: json.RawMessage(`{"path":"C:\\Протокол.docx","upload_token":"capability","drive_owner_id":1}`)}}
	h.userRequestService = stub
	send := func(token string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]string{"token": token, "content": base64.StdEncoding.EncodeToString([]byte("original"))})
		r := mux.SetURLVars(httptest.NewRequest("POST", "/", strings.NewReader(string(body))), map[string]string{"request_id": id.String()})
		w := httptest.NewRecorder()
		h.ReceiveAgentFile(w, r)
		return w
	}
	if w := send("wrong"); w.Code != 403 {
		t.Fatal(w.Code)
	}
	for i := 0; i < 2; i++ {
		if w := send("capability"); w.Code != 200 && w.Code != 201 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if puts != 1 {
		t.Fatalf("duplicate upload: %d", puts)
	}
	var count int
	h.sqlDB.QueryRow(`SELECT COUNT(*) FROM drive_files WHERE user_id=1`).Scan(&count)
	if count != 1 {
		t.Fatal("wrong drive owner/count", count)
	}
	stub.request.CreatedAt = time.Now().Add(-25 * time.Hour)
	if w := send("capability"); w.Code != 403 {
		t.Fatal("expired capability accepted")
	}
}

func TestAgentFileAdminOnly(t *testing.T) {
	w := httptest.NewRecorder()
	HttpServer{}.RequestAgentFile(w, httptest.NewRequest("POST", "/agent_files", strings.NewReader(`{"agent_id":1,"path":"C:\\test.docx"}`)))
	if w.Code != 403 {
		t.Fatal(w.Code)
	}
	w = httptest.NewRecorder()
	HttpServer{}.CreateUserRequest(w, httptest.NewRequest("POST", "/user_requests", strings.NewReader(`{"user_id":"admin","agent_id":1,"command":"send_protocol_file"}`)))
	if w.Code != 403 {
		t.Fatal(w.Code)
	}
}

func TestAgentFileCapabilitySurvivesConfigurationSanitization(t *testing.T) {
	raw, err := safeAgentConfiguration(json.RawMessage(`{"version":"0.2.21","remote_protocol_files":true,"password":"secret"}`))
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err = json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if result["remote_protocol_files"] != true || result["password"] != nil {
		t.Fatal("capability lost or credentials leaked")
	}
}

func TestAgentFileCapabilityNotInPublicResponse(t *testing.T) {
	job := domain.UserRequest{Command: "send_protocol_file", Payload: json.RawMessage(`{"path":"x.docx","upload_token":"secret","drive_owner_id":1}`)}
	response := toUserRequestResponse(job)
	if strings.Contains(string(response.Payload), "secret") || strings.Contains(string(response.Payload), "drive_owner_id") {
		t.Fatal(string(response.Payload))
	}
	if toAgentCommandResponse(job)["upload_token"] != "secret" {
		t.Fatal("agent must receive capability")
	}
}

func TestProtocolFilename(t *testing.T) {
	for input, want := range map[string]string{`C:\Folder\Комаров.docx`: "Комаров.docx", "/tmp/a.PDF": "a.PDF", "/tmp/env.txt": "", "/tmp/x.docx\n": "x.docx", "bad\x00.docx": ""} {
		if got := protocolFilename(input); got != want {
			t.Errorf("%q: %q != %q", input, got, want)
		}
	}
}
