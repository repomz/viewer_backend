package httpserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/repomz/viewer_backend/internal/app/common/server"
)

type dutyScheduleStaff struct {
	ID     string            `json:"id"`
	Name   string            `json:"name"`
	Role   string            `json:"role,omitempty"`
	Shifts map[string]string `json:"shifts"`
	Review map[string]bool   `json:"review,omitempty"`
}

type dutyScheduleGroup struct {
	ID    string              `json:"id"`
	Label string              `json:"label"`
	Staff []dutyScheduleStaff `json:"staff"`
}

type dutyScheduleDocument struct {
	SourceUploadedAt *time.Time          `json:"source_uploaded_at,omitempty"`
	Published        bool                `json:"published"`
	Month            string              `json:"month"`
	Holidays         []int               `json:"holidays"`
	Groups           []dutyScheduleGroup `json:"groups"`
	UpdatedAt        time.Time           `json:"updated_at"`
}

var dutyScheduleMu sync.RWMutex

func scheduleReady(document dutyScheduleDocument) bool {
	if document.SourceUploadedAt == nil || len(document.Groups) == 0 {
		return false
	}
	for _, group := range document.Groups {
		for _, staff := range group.Staff {
			for _, review := range staff.Review {
				if review {
					return false
				}
			}
		}
	}
	return true
}

func excludedScheduleSurgeon(name string) bool {
	parts := strings.Fields(strings.ToLower(strings.ReplaceAll(name, "ё", "е")))
	return len(parts) > 0 && parts[0] == "терешин"
}

func filterScheduleStaff(document *dutyScheduleDocument) {
	for index := range document.Groups {
		group := &document.Groups[index]
		if group.ID != "surgeons" {
			continue
		}
		staff := group.Staff[:0]
		for _, person := range group.Staff {
			if !excludedScheduleSurgeon(person.Name) {
				staff = append(staff, person)
			}
		}
		group.Staff = staff
	}
}

func dutySchedulePath(month string) string {
	directory := strings.TrimSpace(os.Getenv("PLANS_DIR"))
	if directory == "" {
		directory = strings.TrimSpace(os.Getenv("REPORTS_DIR"))
	}
	if directory == "" {
		directory = "/app/reports"
	}
	return filepath.Join(directory, "duty-schedules", month+".json")
}

func defaultDutySchedule(month string) dutyScheduleDocument {
	return dutyScheduleDocument{Month: month, Holidays: []int{}, Groups: []dutyScheduleGroup{
		{ID: "surgeons", Label: "Хирурги", Staff: []dutyScheduleStaff{
			{ID: "kirgizov", Name: "Киргизов", Shifts: map[string]string{}},
			{ID: "idrisov", Name: "Идрисов", Shifts: map[string]string{}},
			{ID: "starkov", Name: "Старков", Shifts: map[string]string{}},
			{ID: "shpilevoy", Name: "Шпилевой", Shifts: map[string]string{}},
			{ID: "suslov", Name: "Суслов", Role: "duty_only", Shifts: map[string]string{}},
		}},
	}, UpdatedAt: time.Now().UTC()}
}

func normalizeDutySchedule(document *dutyScheduleDocument, month string) error {
	filterScheduleStaff(document)
	date, err := time.Parse("2006-01", month)
	if err != nil {
		return errors.New("month must use YYYY-MM format")
	}
	days := date.AddDate(0, 1, -1).Day()
	document.Month = month
	if len(document.Groups) == 0 || len(document.Groups) > 12 {
		return errors.New("schedule groups have an invalid size")
	}
	holidaySet := map[int]bool{}
	for _, day := range document.Holidays {
		if day < 1 || day > days {
			return fmt.Errorf("invalid holiday day: %d", day)
		}
		holidaySet[day] = true
	}
	document.Holidays = document.Holidays[:0]
	for day := range holidaySet {
		document.Holidays = append(document.Holidays, day)
	}
	sort.Ints(document.Holidays)
	groupIDs := map[string]bool{}
	for groupIndex := range document.Groups {
		group := &document.Groups[groupIndex]
		group.ID = strings.TrimSpace(group.ID)
		group.Label = strings.TrimSpace(group.Label)
		if group.ID == "" || group.Label == "" || len(group.Staff) > 100 || groupIDs[group.ID] {
			return errors.New("invalid schedule group")
		}
		groupIDs[group.ID] = true
		staffIDs := map[string]bool{}
		for staffIndex := range group.Staff {
			staff := &group.Staff[staffIndex]
			staff.ID = strings.TrimSpace(staff.ID)
			staff.Name = strings.TrimSpace(staff.Name)
			staff.Role = strings.TrimSpace(staff.Role)
			if staff.ID == "" || staff.Name == "" || staffIDs[staff.ID] || len(staff.Name) > 200 {
				return errors.New("invalid schedule staff member")
			}
			staffIDs[staff.ID] = true
			if staff.Role != "" && staff.Role != "day_only" && staff.Role != "duty_only" {
				return errors.New("invalid shift role")
			}
			if staff.Shifts == nil {
				staff.Shifts = map[string]string{}
			}
			for day, value := range staff.Shifts {
				parts := strings.Split(day, ":")
				number, err := strconv.Atoi(parts[0])
				if err != nil || number < 1 || number > days {
					return fmt.Errorf("invalid shift day: %s", day)
				}
				if len(parts) > 2 || (len(parts) == 2 && parts[1] != "day" && parts[1] != "duty") {
					return fmt.Errorf("invalid shift row: %s", day)
				}
				value = strings.TrimSpace(value)
				if len(value) > 12 {
					return errors.New("shift value is too long")
				}
				if value == "" {
					delete(staff.Shifts, day)
				} else {
					staff.Shifts[day] = value
				}
			}
			for key := range staff.Review {
				parts := strings.Split(key, ":")
				number, err := strconv.Atoi(parts[0])
				if err != nil || number < 1 || number > days || len(parts) != 2 || (parts[1] != "day" && parts[1] != "duty") {
					return errors.New("invalid review cell")
				}
			}
		}
	}
	document.UpdatedAt = time.Now().UTC()
	return nil
}

func loadDutySchedule(month string) (dutyScheduleDocument, error) {
	file, err := os.Open(dutySchedulePath(month))
	if errors.Is(err, os.ErrNotExist) {
		return defaultDutySchedule(month), nil
	}
	if err != nil {
		return dutyScheduleDocument{}, err
	}
	defer file.Close()
	var document dutyScheduleDocument
	if err := json.NewDecoder(file).Decode(&document); err != nil {
		return dutyScheduleDocument{}, err
	}
	filterScheduleStaff(&document)
	document.Published = scheduleReady(document)
	return document, nil
}

func saveDutySchedule(document dutyScheduleDocument) error {
	path := dutySchedulePath(document.Month)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".duty-schedule-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(document); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Chmod(0o640); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func (h HttpServer) GetDutySchedule(w http.ResponseWriter, r *http.Request) {
	month := mux.Vars(r)["month"]
	if _, err := time.Parse("2006-01", month); err != nil {
		server.BadRequest("invalid-month", err, w, r)
		return
	}
	dutyScheduleMu.RLock()
	document, err := loadDutySchedule(month)
	dutyScheduleMu.RUnlock()
	if err != nil {
		server.InternalError("duty-schedule", err, w, r)
		return
	}
	user, _ := currentAuthenticatedUser(r)
	if user.Role != "admin" && !document.Published {
		document.Groups = []dutyScheduleGroup{}
		document.Holidays = []int{}
	}
	w.Header().Set("Cache-Control", "no-store")
	server.RespondOK(document, w, r)
}

func (h HttpServer) PutDutySchedule(w http.ResponseWriter, r *http.Request) {
	month := mux.Vars(r)["month"]
	var document dutyScheduleDocument
	if err := decodeJSON(w, r, &document); err != nil {
		server.BadRequest("invalid-json", err, w, r)
		return
	}
	if err := normalizeDutySchedule(&document, month); err != nil {
		server.BadRequest("invalid-duty-schedule", err, w, r)
		return
	}
	dutyScheduleMu.Lock()
	previous, err := loadDutySchedule(month)
	if err == nil {
		// Only the recognition endpoint can establish upload provenance.
		document.SourceUploadedAt = previous.SourceUploadedAt
		document.Published = scheduleReady(document)
		err = saveDutySchedule(document)
	}
	dutyScheduleMu.Unlock()
	if err != nil {
		server.InternalError("duty-schedule", err, w, r)
		return
	}
	server.RespondOK(document, w, r)
}
