package httpserver

import (
	"encoding/json"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/repomz/viewer_backend/internal/app/common/server"
	"net/http"
	"strings"
)

type paymentRule struct {
	Field string `json:"field"`
	Value string `json:"value"`
	Mode  string `json:"mode"`
}
type paymentRulesResponse struct {
	Rules      []paymentRule `json:"rules"`
	StudyTypes []string      `json:"study_types"`
}

func (h HttpServer) GetPaymentRules(w http.ResponseWriter, r *http.Request) {
	result := paymentRulesResponse{Rules: []paymentRule{}, StudyTypes: []string{}}
	rows, err := h.sqlDB.QueryContext(r.Context(), `SELECT field,value,mode FROM payment_rules ORDER BY field,value`)
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	for rows.Next() {
		var rule paymentRule
		if err = rows.Scan(&rule.Field, &rule.Value, &rule.Mode); err != nil {
			break
		}
		result.Rules = append(result.Rules, rule)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	rows, err = h.sqlDB.QueryContext(r.Context(), `SELECT DISTINCT study_type FROM studies WHERE NOT deleted AND lower(study_type) NOT IN ('xa','ct') ORDER BY study_type`)
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var value string
		if err = rows.Scan(&value); err != nil {
			server.RespondWithError(err, w, r)
			return
		}
		result.StudyTypes = append(result.StudyTypes, value)
	}
	if err = rows.Err(); err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	server.RespondOK(result, w, r)
}

func (h HttpServer) PutPaymentRules(w http.ResponseWriter, r *http.Request) {
	var input paymentRulesResponse
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&input) != nil || len(input.Rules) > 200 {
		writePlatformError(w, 400, "Некорректные правила")
		return
	}
	seen := map[string]bool{}
	for _, rule := range input.Rules {
		key := rule.Field + "|" + rule.Value
		if (rule.Field != "study_type" && rule.Field != "options") || (rule.Mode != "auto" && rule.Mode != "review") || strings.TrimSpace(rule.Value) == "" || len(rule.Value) > 200 || (rule.Field == "options" && rule.Value != "ivus" && rule.Value != "vabk" && rule.Value != "ekmo") || seen[key] {
			writePlatformError(w, 400, "Некорректное или повторное правило")
			return
		}
		seen[key] = true
	}
	tx, err := h.sqlDB.BeginTx(r.Context(), nil)
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	defer tx.Rollback()
	// Serialize rule changes with manual decisions.
	_, err = tx.ExecContext(r.Context(), `SELECT pg_advisory_xact_lock(20261003)`)
	if err == nil {
		_, err = tx.ExecContext(r.Context(), `DELETE FROM payment_rules`)
	}
	for _, rule := range input.Rules {
		if err != nil {
			break
		}
		_, err = tx.ExecContext(r.Context(), `INSERT INTO payment_rules(field,value,mode) VALUES($1,$2,$3)`, rule.Field, rule.Value, rule.Mode)
	}
	if err == nil {
		_, err = tx.ExecContext(r.Context(), `UPDATE studies SET payment=payment, updated_at=now() WHERE NOT deleted AND lower(study_type) NOT IN ('xa','ct') AND EXTRACT(YEAR FROM (time_beginning AT TIME ZONE 'UTC') AT TIME ZONE 'Asia/Tomsk')=EXTRACT(YEAR FROM now() AT TIME ZONE 'Asia/Tomsk')`)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	invalidateOperationStatisticsResponseCache()
	h.GetPaymentRules(w, r)
}

func (h HttpServer) PutStudyPayment(w http.ResponseWriter, r *http.Request) {
	user, ok := currentAuthenticatedUser(r)
	if !ok || (user.Role != "admin" && !user.CanConfirmPayment) {
		writePlatformError(w, 403, "Подтверждение доступно администратору и Артему")
		return
	}
	id, err := uuid.Parse(mux.Vars(r)["study_id"])
	if err != nil {
		writePlatformError(w, 400, "Некорректный протокол")
		return
	}
	var input struct {
		Payment string `json:"payment"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&input) != nil || (input.Payment != "oms" && input.Payment != "vmp") {
		writePlatformError(w, 400, "Выберите ОМС или ВМП")
		return
	}
	tx, err := h.sqlDB.BeginTx(r.Context(), nil)
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(r.Context(), `SELECT pg_advisory_xact_lock(20261003)`)
	if err == nil {
		var found uuid.UUID
		err = tx.QueryRowContext(r.Context(), `UPDATE studies SET payment_override=$2,updated_at=now() WHERE id=$1 AND NOT deleted AND EXTRACT(YEAR FROM (time_beginning AT TIME ZONE 'UTC') AT TIME ZONE 'Asia/Tomsk')=EXTRACT(YEAR FROM now() AT TIME ZONE 'Asia/Tomsk') RETURNING id`, id, input.Payment).Scan(&found)
	}
	if err == nil {
		_, err = tx.ExecContext(r.Context(), `INSERT INTO payment_decisions(study_id,user_id,payment) VALUES($1,$2,$3)`, id, user.ID, input.Payment)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	invalidateOperationStatisticsResponseCache()
	study, err := h.studyService.GetStudyByID(r.Context(), id)
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	server.RespondOK(toResponseStudy(study), w, r)
}
