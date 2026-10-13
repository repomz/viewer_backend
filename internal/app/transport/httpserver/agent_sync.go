package httpserver

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/repomz/viewer_backend/internal/app/common/server"
)

type protocolProbe struct {
	StudyID string    `json:"study_id"`
	Patient string    `json:"patient"`
	Room    int       `json:"room"`
	Time    time.Time `json:"time_beginning"`
}

func probeStatus(probe protocolProbe, records []protocolProbe) string {
	normalize := func(s string) string {
		return strings.Join(strings.Fields(strings.ReplaceAll(strings.ToLower(s), "ё", "е")), " ")
	}
	matched, conflict := 0, false
	for _, row := range records {
		if probe.Room != 0 && row.Room != 0 && probe.Room != row.Room {
			continue
		}
		samePatient := normalize(probe.Patient) == normalize(row.Patient)
		sameTime := probe.Time.Equal(row.Time)
		// Never overwrite a coinciding number with a different patient or date.
		if row.StudyID == probe.StudyID {
			if samePatient && sameTime {
				matched++
			} else {
				conflict = true
			}
		} else if samePatient && sameTime {
			conflict = true
		}
	}
	if conflict || matched > 1 {
		return "review"
	}
	if matched == 1 {
		return "present"
	}
	if probe.Room == 0 {
		return "review"
	}
	return "missing"
}

// ReconcileProtocolBatch is read-only: it never replaces manually edited fields.
func (h HttpServer) ReconcileProtocolBatch(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	var request struct {
		Year  int             `json:"year"`
		Items []protocolProbe `json:"items"`
	}
	if json.NewDecoder(r.Body).Decode(&request) != nil || len(request.Items) == 0 || len(request.Items) > 25 {
		http.Error(w, "expected 1..25 protocol identities", 400)
		return
	}
	location, _ := time.LoadLocation("Asia/Tomsk")
	if request.Year != time.Now().In(location).Year() {
		http.Error(w, "current year required", 400)
		return
	}
	from := time.Date(request.Year, 1, 1, 0, 0, 0, 0, location)
	for _, p := range request.Items {
		if strings.TrimSpace(p.Patient) == "" || strings.TrimSpace(p.StudyID) == "" || p.Room < 0 || p.Room > 2 || p.Time.Before(from) || !p.Time.Before(from.AddDate(1, 0, 0)) {
			http.Error(w, "invalid protocol identity", 400)
			return
		}
	}
	rows, err := h.sqlDB.QueryContext(r.Context(), `SELECT study_id, patient, room, time_beginning FROM studies WHERE NOT deleted AND lower(study_type) NOT IN ('xa','ct') AND time_beginning >= $1 AND time_beginning < $2`, from.UTC(), from.AddDate(1, 0, 0).UTC())
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	defer rows.Close()
	records := []protocolProbe{}
	for rows.Next() {
		var p protocolProbe
		if err = rows.Scan(&p.StudyID, &p.Patient, &p.Room, &p.Time); err != nil {
			server.RespondWithError(err, w, r)
			return
		}
		records = append(records, p)
	}
	if err = rows.Err(); err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	statuses := make([]string, len(request.Items))
	for i, p := range request.Items {
		statuses[i] = probeStatus(p, records)
	}
	server.RespondOK(map[string]any{"statuses": statuses}, w, r)
}

// Only signed public release artifacts, never configuration or signing keys.
func (h HttpServer) AgentRelease(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["artifact"]
	if !regexp.MustCompile(`^(?:default|agent-[1-9][0-9]*)\.json$|^[a-f0-9]{64}\.zip$`).MatchString(name) {
		http.NotFound(w, r)
		return
	}
	root := os.Getenv("AGENT_RELEASE_DIR")
	if root == "" {
		http.NotFound(w, r)
		return
	}
	if strings.HasSuffix(name, ".json") {
		w.Header().Set("Cache-Control", "no-store")
	}
	http.ServeFile(w, r, filepath.Join(root, name))
}
