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
}

type dutyScheduleGroup struct {
	ID    string              `json:"id"`
	Label string              `json:"label"`
	Staff []dutyScheduleStaff `json:"staff"`
}

type dutyScheduleDocument struct {
	Month     string              `json:"month"`
	Holidays  []int               `json:"holidays"`
	Groups    []dutyScheduleGroup `json:"groups"`
	UpdatedAt time.Time           `json:"updated_at"`
}

var dutyScheduleMu sync.RWMutex

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
			{ID: "idrisov", Name: "Идрисов М.З.", Role: "хирург", Shifts: map[string]string{}},
			{ID: "protasov", Name: "Протасов М.З.", Role: "хирург", Shifts: map[string]string{}},
			{ID: "starkov", Name: "Старков М.С.", Role: "хирург", Shifts: map[string]string{}},
			{ID: "tereshin", Name: "Терещин А.А.", Role: "хирург", Shifts: map[string]string{}},
		}},
		{ID: "nurses", Label: "Медсёстры", Staff: []dutyScheduleStaff{
			{ID: "shiryaeva", Name: "Ширяева И.П.", Role: "м/с", Shifts: map[string]string{}},
			{ID: "shipiseva", Name: "Шиписева М.П.", Role: "м/с", Shifts: map[string]string{}},
			{ID: "voronova", Name: "Воронова Ж.А.", Role: "м/с", Shifts: map[string]string{}},
			{ID: "glybova", Name: "Глыбова Л.В.", Role: "м/с", Shifts: map[string]string{}},
			{ID: "ilyina", Name: "Ильина А.А.", Role: "м/с", Shifts: map[string]string{}},
			{ID: "kayrova", Name: "Кайрова С.Л.", Role: "м/с", Shifts: map[string]string{}},
			{ID: "plotnikova", Name: "Плотникова Е.В.", Role: "м/с", Shifts: map[string]string{}},
			{ID: "protasova", Name: "Протасова Е.В.", Role: "м/с", Shifts: map[string]string{}},
			{ID: "sukhanova", Name: "Суханова О.А.", Role: "м/с", Shifts: map[string]string{}},
			{ID: "shishkina", Name: "Шишкина Т.В.", Role: "м/с", Shifts: map[string]string{}},
			{ID: "shtenina", Name: "Штенина Е.В.", Role: "м/с", Shifts: map[string]string{}},
		}},
		{ID: "orderlies", Label: "Санитарки", Staff: []dutyScheduleStaff{
			{ID: "volnykh", Name: "Вольных Е.Б.", Role: "сан", Shifts: map[string]string{}},
			{ID: "islamova", Name: "Исламова Е.А.", Role: "сан", Shifts: map[string]string{}},
			{ID: "polenova", Name: "Поленова А.В.", Role: "сан", Shifts: map[string]string{}},
			{ID: "magomedova", Name: "Магомедова Н.В.", Role: "сан", Shifts: map[string]string{}},
			{ID: "pestova", Name: "Пестова Т.В.", Role: "сан", Shifts: map[string]string{}},
			{ID: "chumel", Name: "Чумель Е.Н.", Role: "сан", Shifts: map[string]string{}},
			{ID: "amurbieva", Name: "Амурбиева М.Б.", Role: "сан", Shifts: map[string]string{}},
		}},
	}, UpdatedAt: time.Now().UTC()}
}

func normalizeDutySchedule(document *dutyScheduleDocument, month string) error {
	if _, err := time.Parse("2006-01", month); err != nil {
		return errors.New("month must use YYYY-MM format")
	}
	document.Month = month
	if len(document.Groups) == 0 || len(document.Groups) > 12 {
		return errors.New("schedule groups have an invalid size")
	}
	holidaySet := map[int]bool{}
	for _, day := range document.Holidays {
		if day < 1 || day > 31 {
			return fmt.Errorf("invalid holiday day: %d", day)
		}
		holidaySet[day] = true
	}
	document.Holidays = document.Holidays[:0]
	for day := range holidaySet {
		document.Holidays = append(document.Holidays, day)
	}
	sort.Ints(document.Holidays)
	for groupIndex := range document.Groups {
		group := &document.Groups[groupIndex]
		group.ID = strings.TrimSpace(group.ID)
		group.Label = strings.TrimSpace(group.Label)
		if group.ID == "" || group.Label == "" || len(group.Staff) > 100 {
			return errors.New("invalid schedule group")
		}
		for staffIndex := range group.Staff {
			staff := &group.Staff[staffIndex]
			staff.ID = strings.TrimSpace(staff.ID)
			staff.Name = strings.TrimSpace(staff.Name)
			staff.Role = strings.TrimSpace(staff.Role)
			if staff.ID == "" || staff.Name == "" {
				return errors.New("invalid schedule staff member")
			}
			if staff.Shifts == nil {
				staff.Shifts = map[string]string{}
			}
			for day, value := range staff.Shifts {
				number, err := strconv.Atoi(day)
				if err != nil || number < 1 || number > 31 {
					return fmt.Errorf("invalid shift day: %s", day)
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
	err := saveDutySchedule(document)
	dutyScheduleMu.Unlock()
	if err != nil {
		server.InternalError("duty-schedule", err, w, r)
		return
	}
	server.RespondOK(document, w, r)
}
