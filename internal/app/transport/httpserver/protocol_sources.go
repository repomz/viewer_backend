package httpserver

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/repomz/viewer_backend/internal/app/common/server"
	"github.com/repomz/viewer_backend/internal/app/db"
	"github.com/repomz/viewer_backend/internal/app/domain"
	"github.com/repomz/viewer_backend/internal/app/transport/httpmodels"
)

var errProtocolAmbiguous = errors.New("ambiguous protocol identity; manual review required")

func lockProtocolSources(ctx context.Context, tx *sql.Tx) error {
	// Serialize protocol writes and source snapshots, including legacy agents.
	_, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(17062026, 1)`)
	return err
}

func findProtocol(ctx context.Context, tx *sql.Tx, p protocolProbe) (uuid.UUID, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id FROM studies WHERE NOT deleted AND room=$1 AND study_id=$2
        AND lower(btrim(patient))=lower(btrim($3)) AND time_beginning=$4
        AND lower(btrim(study_type)) NOT IN ('xa','ct') LIMIT 2`, p.Room, p.StudyID, p.Patient, p.Time)
	if err != nil {
		return uuid.Nil, err
	}
	defer rows.Close()
	id, count := uuid.Nil, 0
	for rows.Next() {
		if err = rows.Scan(&id); err != nil {
			return uuid.Nil, err
		}
		count++
	}
	if err = rows.Err(); err != nil {
		return uuid.Nil, err
	}
	if count > 1 {
		return uuid.Nil, errProtocolAmbiguous
	}
	return id, nil
}

func bindProtocolSource(ctx context.Context, tx *sql.Tx, source *httpmodels.ProtocolSource, id uuid.UUID) error {
	if source == nil {
		return nil
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO protocol_sources(agent_id,file_id,study_id,root,path)
        VALUES($1,$2,$3,$4,$5) ON CONFLICT(agent_id,file_id) DO UPDATE SET
        root=EXCLUDED.root,path=EXCLUDED.path,last_seen=NOW(),missing_since=NULL
        WHERE protocol_sources.study_id=EXCLUDED.study_id`, source.AgentID, source.FileID, id, source.Root, source.Path)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n != 1 {
		return errProtocolAmbiguous
	}
	return err
}

// Source identity takes precedence over editable clinical text. For old agents,
// an unambiguous room/number/patient/time receipt also survives a title change.
func (h HttpServer) upsertProtocol(ctx context.Context, request httpmodels.StudyRequest) (domain.Study, error) {
	tx, err := h.sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return domain.Study{}, err
	}
	defer tx.Rollback()
	if err = lockProtocolSources(ctx, tx); err != nil {
		return domain.Study{}, err
	}
	id := uuid.Nil
	if request.Source != nil {
		err = tx.QueryRowContext(ctx, `SELECT study_id FROM protocol_sources WHERE agent_id=$1 AND file_id=$2`, request.Source.AgentID, request.Source.FileID).Scan(&id)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return domain.Study{}, err
		}
	}
	if id == uuid.Nil && request.Room > 0 {
		id, err = findProtocol(ctx, tx, protocolProbe{StudyID: request.StudyID, Patient: request.Patient, Room: int(request.Room), Time: request.TimeBeginning})
		if err != nil {
			return domain.Study{}, err
		}
	}
	birth := sql.NullTime{}
	if request.BirthDate != "" {
		birth.Time, err = time.Parse("2006-01-02", request.BirthDate)
		birth.Valid = err == nil
	}
	queries := db.New(tx)
	var row db.Study
	if id == uuid.Nil {
		row, err = queries.CreateStudy(ctx, db.CreateStudyParams{
			StudyID: request.StudyID, Patient: request.Patient, Age: sql.NullInt32{Int32: request.Age, Valid: true},
			Department: request.Department, NameOperation: request.NameOperation, StudyType: request.StudyType,
			DescrOperation: request.DescrOperation, Recommendation: request.Recommendation,
			TimeBeginning: sql.NullTime{Time: request.TimeBeginning, Valid: true}, TimeDuration: sql.NullInt32{Int32: request.TimeDuration, Valid: true},
			Surgeon: request.Surgeon, DicomLink: sql.NullString{String: request.DicomLink, Valid: request.DicomLink != ""},
			BirthDate: birth, Options: request.Options, Description: request.Description, Room: request.Room,
		})
		if err == nil {
			id = row.ID
		}
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE studies SET study_id=$2,patient=$3,age=$4,department=$5,
            name_operation=$6,study_type=$7,descr_operation=$8,recommendation=$9,time_beginning=$10,
            time_duration=$11,surgeon=$12,birth_date=COALESCE($13,birth_date),options=$14,description=$15,
            room=CASE WHEN $16=0 THEN room ELSE $16 END,dicom_link=COALESCE(NULLIF($17,''),dicom_link),
            deleted=false,updated_at=NOW() WHERE id=$1`, id, request.StudyID, request.Patient, request.Age,
			request.Department, request.NameOperation, request.StudyType, request.DescrOperation, request.Recommendation,
			request.TimeBeginning, request.TimeDuration, request.Surgeon, birth, request.Options, request.Description, request.Room, request.DicomLink)
		if err == nil {
			row, err = queries.GetStudyByID(ctx, id)
		}
	}
	if err != nil {
		return domain.Study{}, err
	}
	if err = bindProtocolSource(ctx, tx, request.Source, id); err != nil {
		return domain.Study{}, err
	}
	result, err := domain.DBToNewStudy(row)
	if err != nil {
		return domain.Study{}, err
	}
	if err = tx.Commit(); err != nil {
		return domain.Study{}, err
	}
	return result, nil
}

type sourceSnapshot struct {
	AgentID int      `json:"agent_id"`
	Root    string   `json:"root"`
	ScanID  string   `json:"scan_id"`
	FileIDs []string `json:"file_ids"`
}

func (h HttpServer) ProtocolSourceSnapshot(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var snapshot sourceSnapshot
	if json.NewDecoder(r.Body).Decode(&snapshot) != nil || snapshot.AgentID <= 0 || strings.TrimSpace(snapshot.Root) == "" || len(snapshot.Root) > 2048 || snapshot.FileIDs == nil || len(snapshot.FileIDs) > 10000 {
		http.Error(w, "invalid source snapshot", 400)
		return
	}
	if id, err := uuid.Parse(snapshot.ScanID); err != nil || id == uuid.Nil {
		http.Error(w, "invalid scan id", 400)
		return
	} else {
		snapshot.ScanID = id.String()
	}
	for _, v := range snapshot.FileIDs {
		if id, err := uuid.Parse(v); err != nil || id == uuid.Nil {
			http.Error(w, "invalid file id", 400)
			return
		}
	}
	count, err := h.applySourceSnapshot(r.Context(), snapshot)
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	if count > 0 {
		invalidateStudyAnalysisResponseCaches()
		log.Printf("Protocol source cleanup: agent=%d root=%q deleted=%d", snapshot.AgentID, snapshot.Root, count)
	}
	server.RespondOK(map[string]any{"deleted": count}, w, r)
}

func (h HttpServer) applySourceSnapshot(ctx context.Context, snapshot sourceSnapshot) (int64, error) {
	tx, err := h.sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if err = lockProtocolSources(ctx, tx); err != nil {
		return 0, err
	}
	var last string
	err = tx.QueryRowContext(ctx, `SELECT last_scan::text FROM protocol_source_scans WHERE agent_id=$1 AND root=$2`, snapshot.AgentID, snapshot.Root).Scan(&last)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	if last == snapshot.ScanID {
		return 0, nil
	}
	ids, _ := json.Marshal(snapshot.FileIDs)
	_, err = tx.ExecContext(ctx, `UPDATE protocol_sources SET missing_since=NULL,last_seen=NOW()
        WHERE agent_id=$1 AND root=$2 AND file_id IN (SELECT value::uuid FROM jsonb_array_elements_text($3::jsonb))`, snapshot.AgentID, snapshot.Root, string(ids))
	if err != nil {
		return 0, err
	}
	// Only explicitly registered sources, absent on repeated complete scans for
	// >=10 minutes, qualify. A surviving copy/source always protects the study.
	result, err := tx.ExecContext(ctx, `UPDATE studies s SET deleted=true,updated_at=NOW() WHERE NOT s.deleted
        AND EXISTS(SELECT 1 FROM protocol_sources p WHERE p.study_id=s.id AND p.agent_id=$1 AND p.root=$2 AND p.missing_since<=NOW()-INTERVAL '10 minutes')
        AND NOT EXISTS(SELECT 1 FROM protocol_sources p WHERE p.study_id=s.id AND (p.missing_since IS NULL OR p.missing_since>NOW()-INTERVAL '10 minutes'))`, snapshot.AgentID, snapshot.Root)
	if err != nil {
		return 0, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE protocol_sources SET missing_since=NOW() WHERE agent_id=$1 AND root=$2 AND missing_since IS NULL
        AND file_id NOT IN (SELECT value::uuid FROM jsonb_array_elements_text($3::jsonb))`, snapshot.AgentID, snapshot.Root, string(ids))
	if err != nil {
		return 0, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO protocol_source_scans(agent_id,root,last_scan) VALUES($1,$2,$3)
        ON CONFLICT(agent_id,root) DO UPDATE SET last_scan=EXCLUDED.last_scan`, snapshot.AgentID, snapshot.Root, snapshot.ScanID)
	if err != nil {
		return 0, fmt.Errorf("save source scan: %w", err)
	}
	return count, tx.Commit()
}
