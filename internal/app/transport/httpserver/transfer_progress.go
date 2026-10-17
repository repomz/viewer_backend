package httpserver

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/repomz/viewer_backend/internal/app/common/server"
	"github.com/repomz/viewer_backend/internal/app/domain"
)

// Serialize creation across processes: repeated clicks reuse the active delivery.
func (h HttpServer) createTransfer(ctx context.Context, r domain.NewUserRequest) (domain.UserRequest, error) {
	tx, err := h.sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return domain.UserRequest{}, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(17062026,2)`); err != nil {
		return domain.UserRequest{}, err
	}
	var id uuid.UUID
	err = tx.QueryRowContext(ctx, `SELECT id FROM user_requests WHERE agent_id=$1 AND command=$2
 AND payload->>'study_uid'=($3::jsonb)->>'study_uid' AND status IN ('pending','in_progress')
 ORDER BY created_at LIMIT 1`, r.AgentID, r.Command, string(r.Payload)).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		err = tx.QueryRowContext(ctx, `INSERT INTO user_requests(user_id,agent_id,request_type,command,payload,max_attempts)
  VALUES($1,$2,$3,$4,$5,$6) RETURNING id`, r.UserID, r.AgentID, r.RequestType, r.Command, r.Payload, r.MaxAttempts).Scan(&id)
	}
	if err != nil {
		return domain.UserRequest{}, err
	}
	if err = tx.Commit(); err != nil {
		return domain.UserRequest{}, err
	}
	return h.userRequestService.GetByID(ctx, id)
}

type transferProgress struct {
	AgentID   int    `json:"agent_id"`
	Stage     string `json:"stage"`
	Completed int64  `json:"completed"`
	Total     int64  `json:"total"`
	Bytes     int64  `json:"bytes"`
}

func (h HttpServer) RecordTransferProgress(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(mux.Vars(r)["request_id"])
	var p transferProgress
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if err != nil || json.NewDecoder(r.Body).Decode(&p) != nil || p.AgentID <= 0 || p.Completed < 0 || p.Total < 0 || p.Bytes < 0 ||
		(p.Stage != "download" && p.Stage != "upload" && p.Stage != "import") {
		http.Error(w, "invalid transfer progress", 400)
		return
	}
	body, _ := json.Marshal(p)
	result, err := h.sqlDB.ExecContext(r.Context(), `UPDATE user_requests SET
 result=jsonb_build_object('progress',$3::jsonb),updated_at=NOW(),lease_expires_at=NOW()+INTERVAL '5 minutes'
 WHERE id=$1 AND agent_id=$2 AND status='in_progress' AND command IN ('get_ct','get_xa','send_ct_to_pacs','send_xa_to_pacs')`, id, p.AgentID, string(body))
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		http.Error(w, "request is not active", 409)
		return
	}
	server.RespondOK(map[string]bool{"accepted": true}, w, r)
}
