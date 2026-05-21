package httpmodels

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/repomz/viewer_backend/internal/app/domain"
)

type PatientFilter struct {
	Patient string
}

// Список разрешенных фамилий хирургов в нижнем регистре
var allowedSurgeons = map[string]bool{
	"идрисов":  true,
	"шпилевой": true,
	"старков":  true,
	"киргизов": true,
}

// Список разрешенных типов исследований в нижнем регистре
var allowedStudyTypes = map[string]bool{
	"каг":             true,
	"цаг":             true,
	"стент_кор":       true,
	"стент_вса":       true,
	"стент_периферии": true,
	"бап_кор":         true,
	"бап_вса":         true,
	"бап_периферии":   true,
	"тромбаспирация":  true,
}

// Validate проверяет имя пациента отдельно от других структур
func (f *PatientFilter) Validate() error {
	if f.Patient == "" {
		return nil
	}

	runes := []rune(string(f.Patient))
	if len(runes) < 2 {
		return domain.ErrInvalidPatient
	}

	for _, r := range runes {
		if !unicode.IsLetter(r) && r != '-' && !unicode.IsSpace(r) {
			return domain.ErrInvalidPatient
		}
	}
	return nil
}

// Normalize возвращает очищенную строку (без пробелов по краям)
// func (f *PatientFilter) Normalize() string {
// 	return strings.TrimSpace(string(f.Patient))
// }

// IsEmpty проверяет, пустой ли фильтр пациента
func (f PatientFilter) IsEmpty() bool {
	return f.Patient == ""
}

// StudyFilter для фильтрации исследований по дате, хирургу или типу операции
type StudyFilter struct {
	StudyDate *time.Time
	Surgeon   string
	StudyType string
}

// Validate проверяет только параметры исследования
func (f *StudyFilter) Validate() error {
	if f.Surgeon != "" {
		surgeon := strings.ToLower(strings.TrimSpace(string(f.Surgeon)))
		if !allowedSurgeons[surgeon] {
			return domain.ErrNotFound
		}
	}

	if f.StudyType != "" {
		studyType := strings.ToLower(strings.TrimSpace(string(f.StudyType)))
		if !allowedStudyTypes[studyType] {
			return domain.ErrNotFound
		}
	}

	if f.StudyDate != nil && f.StudyDate.After(time.Now()) {
		return fmt.Errorf("study_date cannot be in the future")
	}

	return nil
}

// Normalize приводит параметры исследования к нижнему регистру и удаляет пробелы
func (f *StudyFilter) Normalize() {
	if f.Surgeon != "" {
		f.Surgeon = strings.ToLower(strings.TrimSpace(string(f.Surgeon)))
	}
	if f.StudyType != "" {
		f.StudyType = strings.ToLower(strings.TrimSpace(string(f.StudyType)))
	}
}
