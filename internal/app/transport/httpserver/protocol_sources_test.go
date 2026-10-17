package httpserver

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/repomz/viewer_backend/internal/app/transport/httpmodels"
)

func TestProtocolSourceRegistrationDoesNotChangeClinicalText(t *testing.T) {
	h := sourceTestServer(t)
	p := httpmodels.StudyRequest{StudyID: "42", Patient: "Тест", Room: 1, NameOperation: "Manually corrected title", StudyType: "каг", Department: "к/о 1", TimeBeginning: time.Now().UTC().Truncate(time.Second)}
	study, err := h.upsertProtocol(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	source := &httpmodels.ProtocolSource{AgentID: 1, FileID: uuid.NewString(), Root: "root", Path: "root/file.docx"}
	body, _ := json.Marshal(map[string]any{"year": time.Now().Year(), "items": []protocolProbe{{StudyID: p.StudyID, Patient: p.Patient, Room: 1, Time: p.TimeBeginning, Source: source}}})
	response := httptest.NewRecorder()
	h.ReconcileProtocolBatch(response, httptest.NewRequest("POST", "/agent/protocols/reconcile", bytes.NewReader(body)))
	if response.Code != 200 || !strings.Contains(response.Body.String(), "present") {
		t.Fatal(response.Code, response.Body.String())
	}
	var title string
	h.sqlDB.QueryRow(`SELECT name_operation FROM studies WHERE id=$1`, study.ID()).Scan(&title)
	if title != p.NameOperation {
		t.Fatal("registration changed clinical text")
	}
	var count int
	h.sqlDB.QueryRow(`SELECT COUNT(*) FROM protocol_sources WHERE study_id=$1`, study.ID()).Scan(&count)
	if count != 1 {
		t.Fatal("source was not registered")
	}
}

func sourceTestServer(t *testing.T) HttpServer {
	t.Helper()
	dsn := os.Getenv("VIEWER_TEST_DSN")
	if dsn == "" {
		t.Skip("set VIEWER_TEST_DSN to an isolated PostgreSQL test database")
	}
	database, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)
	schema := "source_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err = database.Exec(`CREATE SCHEMA ` + schema + `; SET search_path TO ` + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Exec(`DROP SCHEMA ` + schema + ` CASCADE`); database.Close() })
	_, err = database.Exec(`CREATE TABLE studies (
        id uuid PRIMARY KEY DEFAULT gen_random_uuid(), created_at timestamptz NOT NULL DEFAULT NOW(), updated_at timestamptz NOT NULL DEFAULT NOW(),
        study_id text NOT NULL, patient text NOT NULL, age integer, department text NOT NULL, name_operation text NOT NULL,
        study_type text NOT NULL, descr_operation text NOT NULL DEFAULT '', description text NOT NULL DEFAULT '', room integer NOT NULL DEFAULT 0,
        recommendation text NOT NULL DEFAULT '', birth_date date, options text NOT NULL DEFAULT '',payment text NOT NULL DEFAULT 'oms',
        payment_pending boolean NOT NULL DEFAULT false,payment_override text NOT NULL DEFAULT '', time_beginning timestamp,
        time_duration integer,surgeon text NOT NULL DEFAULT '',dicom_link text,deleted boolean NOT NULL DEFAULT false);
        CREATE UNIQUE INDEX study_identity ON studies(lower(btrim(patient)),time_beginning,lower(btrim(name_operation)))
        WHERE deleted=false AND lower(btrim(study_type)) NOT IN ('xa','ct') AND time_beginning IS NOT NULL;`)
	if err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile("../../../sql/migrations/20261017000000_protocol_sources.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(strings.Split(string(migration), "-- +goose Down")[0]); err != nil {
		t.Fatal(err)
	}
	return HttpServer{sqlDB: database}
}

func TestProtocolSourceEditMoveAndDeletion(t *testing.T) {
	h := sourceTestServer(t)
	ctx := context.Background()
	p := httpmodels.StudyRequest{StudyID: "1706", Patient: "Тестовый Пациент", Room: 1, NameOperation: "КАГ стент 2", StudyType: "стент_кор", Department: "к/о 1", TimeBeginning: time.Date(2026, 10, 15, 5, 20, 0, 0, time.UTC), DicomLink: "preserved-xa"}
	first, err := h.upsertProtocol(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	p.NameOperation = "КАГ стент 1"
	p.DicomLink = ""
	p.Source = &httpmodels.ProtocolSource{AgentID: 1, FileID: uuid.NewString(), Root: "C:/protocols", Path: "C:/protocols/old.docx"}
	second, err := h.upsertProtocol(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID() != second.ID() || second.DicomLink().String != "preserved-xa" {
		t.Fatal("title edit duplicated study or lost XA")
	}
	p.Source.Path = "C:/protocols/new.docx"
	p.TimeBeginning = p.TimeBeginning.Add(time.Minute)
	p.NameOperation = "КАГ corrected"
	moved, err := h.upsertProtocol(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	if moved.ID() != first.ID() {
		t.Fatal("moving and editing must preserve ID")
	}
	snapshot := sourceSnapshot{AgentID: 1, Root: "C:/protocols", ScanID: uuid.NewString(), FileIDs: []string{}}
	n, err := h.applySourceSnapshot(ctx, snapshot)
	if err != nil || n != 0 {
		t.Fatalf("first absence: %d %v", n, err)
	}
	h.sqlDB.Exec(`UPDATE protocol_sources SET missing_since=NOW()-INTERVAL '11 minutes'`)
	n, err = h.applySourceSnapshot(ctx, snapshot)
	if err != nil || n != 0 {
		t.Fatal("replayed scan must not delete", n, err)
	}
	snapshot.ScanID = uuid.NewString()
	snapshot.FileIDs = []string{p.Source.FileID}
	n, err = h.applySourceSnapshot(ctx, snapshot)
	if err != nil || n != 0 {
		t.Fatal("returned source must not be deleted", n, err)
	}
	snapshot.ScanID = uuid.NewString()
	snapshot.FileIDs = []string{}
	h.applySourceSnapshot(ctx, snapshot)
	h.sqlDB.Exec(`UPDATE protocol_sources SET missing_since=NOW()-INTERVAL '11 minutes'`)
	snapshot.ScanID = uuid.NewString()
	n, err = h.applySourceSnapshot(ctx, snapshot)
	if err != nil || n != 1 {
		t.Fatal("confirmed absence should soft-delete", n, err)
	}
	var deleted bool
	h.sqlDB.QueryRow(`SELECT deleted FROM studies WHERE id=$1`, first.ID()).Scan(&deleted)
	if !deleted {
		t.Fatal("not deleted")
	}
	restored, err := h.upsertProtocol(ctx, p)
	if err != nil || restored.ID() != first.ID() {
		t.Fatal("source reappearance must restore same protocol", err)
	}
}

func TestProtocolSourceProtectsOtherCopiesAndUntrackedStudies(t *testing.T) {
	h := sourceTestServer(t)
	ctx := context.Background()
	p := httpmodels.StudyRequest{StudyID: "100", Patient: "Тест", Room: 1, NameOperation: "КАГ", StudyType: "каг", Department: "к/о 1", TimeBeginning: time.Now().UTC().Truncate(time.Second)}
	p.Source = &httpmodels.ProtocolSource{AgentID: 1, FileID: uuid.NewString(), Root: "root", Path: "root/a.docx"}
	first, err := h.upsertProtocol(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	firstFile := p.Source.FileID
	p.Source.FileID = uuid.NewString()
	p.Source.Path = "root/b.docx"
	copied, err := h.upsertProtocol(ctx, p)
	if err != nil || copied.ID() != first.ID() {
		t.Fatal("copy must not duplicate study", err)
	}
	p.Source = nil
	p.StudyID = "101"
	p.NameOperation = "ВАБК"
	separate, err := h.upsertProtocol(ctx, p)
	if err != nil || separate.ID() == first.ID() {
		t.Fatal("different operation number must remain separate", err)
	}
	h.sqlDB.Exec(`UPDATE protocol_sources SET missing_since=NOW()-INTERVAL '11 minutes' WHERE file_id=$1`, firstFile)
	n, err := h.applySourceSnapshot(ctx, sourceSnapshot{AgentID: 1, Root: "root", ScanID: uuid.NewString(), FileIDs: []string{copiedSourceID(t, h, firstFile)}})
	if err != nil || n != 0 {
		t.Fatal("surviving copy protects protocol", n, err)
	}
	var count int
	h.sqlDB.QueryRow(`SELECT COUNT(*) FROM studies WHERE NOT deleted`).Scan(&count)
	if count != 2 {
		t.Fatal("untracked or separate study affected")
	}
}

func copiedSourceID(t *testing.T, h HttpServer, excluded string) string {
	t.Helper()
	var id string
	if err := h.sqlDB.QueryRow(`SELECT file_id::text FROM protocol_sources WHERE file_id<>$1`, excluded).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}
