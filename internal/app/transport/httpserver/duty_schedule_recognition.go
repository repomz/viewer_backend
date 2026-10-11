package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/repomz/viewer_backend/internal/app/common/server"
)

// Recognition and saving are one server-side operation. Failed OCR preserves the schedule.
var scheduleRecognitionSlot = make(chan struct{}, 1)

type schedulePhotoCell struct {
	Value     string `json:"value"`
	Uncertain bool   `json:"uncertain"`
}
type schedulePhotoRow struct {
	Name  string              `json:"name"`
	Group string              `json:"group"`
	Row   string              `json:"row"`
	Cells []schedulePhotoCell `json:"cells"`
}
type schedulePhotoResult struct {
	Month string             `json:"month"`
	Rows  []schedulePhotoRow `json:"rows"`
}

func photoSchedule(result schedulePhotoResult, month string) (dutyScheduleDocument, error) {
	date, err := time.Parse("2006-01", month)
	if err != nil || result.Month != month {
		return dutyScheduleDocument{}, errors.New("Месяц на фотографии не совпадает с выбранным месяцем")
	}
	days := date.AddDate(0, 1, -1).Day()
	doc := dutyScheduleDocument{Month: month, Holidays: []int{}, Groups: []dutyScheduleGroup{{ID: "surgeons", Label: "Хирурги", Staff: []dutyScheduleStaff{}}, {ID: "nurses", Label: "Медсёстры", Staff: []dutyScheduleStaff{}}}}
	if len(result.Rows) == 0 || len(result.Rows) > 80 {
		return doc, errors.New("Не удалось распознать строки графика")
	}
	seen := map[string]bool{}
	for _, row := range result.Rows {
		if row.Group == "surgeons" && excludedScheduleSurgeon(row.Name) {
			continue
		}
		if row.Group != "surgeons" && row.Group != "nurses" {
			return doc, errors.New("Некорректная группа сотрудников")
		}
		if row.Row != "day" && row.Row != "duty" {
			return doc, errors.New("Некорректная строка смен")
		}
		name := strings.Join(strings.Fields(row.Name), " ")
		if name == "" || len(name) > 200 || len(row.Cells) != days {
			return doc, errors.New("Распознаны не все даты или фамилии. Выберите более чёткую фотографию")
		}
		identity := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(name, " ", ""), ".", ""))
		key := row.Group + ":" + identity + ":" + row.Row
		if seen[key] {
			return doc, errors.New("Распознаватель продублировал строку сотрудника")
		}
		seen[key] = true
		group := &doc.Groups[0]
		if row.Group == "nurses" {
			group = &doc.Groups[1]
		}
		index := -1
		for i, s := range group.Staff {
			if s.ID == identity {
				index = i
				break
			}
		}
		if index < 0 {
			group.Staff = append(group.Staff, dutyScheduleStaff{ID: identity, Name: name, Role: row.Row + "_only", Shifts: map[string]string{}, Review: map[string]bool{}})
			index = len(group.Staff) - 1
		} else {
			group.Staff[index].Role = ""
		}
		staff := &group.Staff[index]
		for i, cell := range row.Cells {
			shiftKey := strconv.Itoa(i+1) + ":" + row.Row
			value := strings.ToUpper(strings.TrimSpace(cell.Value))
			if value == "O" {
				value = "О"
			}
			switch value {
			case "", "6", "8", "16", "18", "24", "О", "Б":
			default:
				value = ""
				cell.Uncertain = true
			}
			if value != "" {
				staff.Shifts[shiftKey] = value
			}
			if cell.Uncertain {
				staff.Review[shiftKey] = true
			}
		}
	}
	if len(doc.Groups[0].Staff) == 0 || len(doc.Groups[1].Staff) == 0 {
		return doc, errors.New("На фото должны быть распознаны хирурги и медсёстры")
	}
	err = normalizeDutySchedule(&doc, month)
	return doc, err
}

func (h HttpServer) RecognizeDutySchedule(w http.ResponseWriter, r *http.Request) {
	month := mux.Vars(r)["month"]
	if _, err := time.Parse("2006-01", month); err != nil {
		writePlatformError(w, 400, "Некорректный месяц")
		return
	}
	select {
	case scheduleRecognitionSlot <- struct{}{}:
		defer func() { <-scheduleRecognitionSlot }()
	default:
		writePlatformError(w, 429, "Уже распознаётся другой график. Повторите позже")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 12<<20)
	data, err := io.ReadAll(r.Body)
	if err != nil {
		writePlatformError(w, 413, "Фотография должна быть не больше 12 МБ")
		return
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "jpeg" && format != "png") || config.Width < 300 || config.Height < 300 || int64(config.Width)*int64(config.Height) > 50_000_000 {
		writePlatformError(w, 400, "Выберите чёткую фотографию JPEG или PNG")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 180*time.Second)
	defer cancel()
	python := os.Getenv("SCHEDULE_OCR_PYTHON")
	if python == "" {
		python = "python3"
	}
	script := os.Getenv("SCHEDULE_OCR_SCRIPT")
	if script == "" {
		script = "/app/scripts/schedule_ocr.py"
	}
	cmd := exec.CommandContext(ctx, python, script, month)
	cmd.Stdin = bytes.NewReader(data)
	cmd.Env = append(os.Environ(), "OMP_THREAD_LIMIT=1", "OPENBLAS_NUM_THREADS=1")
	var output, diagnostic bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &diagnostic
	if err := cmd.Run(); err != nil {
		writePlatformError(w, 422, "Не удалось однозначно распознать сетку или месяц. Снимите весь лист ровно, без обрезанных краёв")
		return
	}
	var recognized schedulePhotoResult
	if err := json.Unmarshal(output.Bytes(), &recognized); err != nil {
		writePlatformError(w, 502, "Ошибка формата распознанного графика")
		return
	}
	result, err := photoSchedule(recognized, month)
	if err != nil {
		writePlatformError(w, 422, err.Error())
		return
	}
	// Save immediately, but never expose unresolved schedules to regular users.
	dutyScheduleMu.Lock()
	previous, err := loadDutySchedule(month)
	if err == nil {
		result.Holidays = previous.Holidays
		now := time.Now().UTC()
		result.SourceUploadedAt = &now
		result.Published = scheduleReady(result)
		err = saveDutySchedule(result)
	}
	dutyScheduleMu.Unlock()
	if err != nil {
		server.InternalError("duty-schedule", err, w, r)
		return
	}
	server.RespondOK(result, w, r)
}
