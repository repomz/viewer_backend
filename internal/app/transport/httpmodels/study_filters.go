package httpmodels

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/repomz/viewer_backend/internal/app/domain"
)

// Самостоятельные типы фильтров на основе строк
type PatientFilter string
type SurgeonFilter string
type StudyTypeFilter string

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
func (f PatientFilter) Validate() error {
	if f == "" {
		return nil
	}

	runes := []rune(string(f))
	if len(runes) < 2 {
		return domain.ErrPatientNameTooShort
	}

	for _, r := range runes {
		if !unicode.IsLetter(r) && r != '-' && !unicode.IsSpace(r) {
			return domain.ErrPatientNameInvalid
		}
	}
	return nil
}

// Normalize возвращает очищенную строку (без пробелов по краям)
func (f PatientFilter) Normalize() PatientFilter {
	return PatientFilter(strings.TrimSpace(string(f)))
}

// IsEmpty проверяет, пустой ли фильтр пациента
func (f PatientFilter) IsEmpty() bool {
	return f == ""
}

// StudyFilter для фильтрации исследований по дате, хирургу или типу операции
type StudyFilter struct {
	StudyDate *time.Time
	Surgeon   SurgeonFilter
	StudyType StudyTypeFilter
}

// Validate проверяет только параметры исследования
func (f *StudyFilter) Validate() error {
	if f.Surgeon != "" {
		surgeon := strings.ToLower(strings.TrimSpace(string(f.Surgeon)))
		if !allowedSurgeons[surgeon] {
			return domain.ErrSurgeonNotFound
		}
	}

	if f.StudyType != "" {
		studyType := strings.ToLower(strings.TrimSpace(string(f.StudyType)))
		if !allowedStudyTypes[studyType] {
			return domain.ErrStudyTypeNotFound
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
		f.Surgeon = SurgeonFilter(strings.ToLower(strings.TrimSpace(string(f.Surgeon))))
	}
	if f.StudyType != "" {
		f.StudyType = StudyTypeFilter(strings.ToLower(strings.TrimSpace(string(f.StudyType))))
	}
}

// IsEmpty - проверка что фильтр пустой
func (f *StudyFilter) IsEmpty() bool {
	return f.StudyDate == nil && f.Surgeon == "" && f.StudyType == ""
}

// HasDateAndSurgeon - комбинация дата + хирург
func (f *StudyFilter) HasDateAndSurgeon() bool {
	return f.StudyDate != nil && f.Surgeon != "" && f.StudyType == ""
}

// HasDateAndType - комбинация дата + тип
func (f *StudyFilter) HasDateAndType() bool {
	return f.StudyDate != nil && f.StudyType != "" && f.Surgeon == ""
}

// HasSurgeonAndType - комбинация хирург + тип
func (f *StudyFilter) HasSurgeonAndType() bool {
	return f.Surgeon != "" && f.StudyType != "" && f.StudyDate == nil
}

// HasAll - все три фильтра
func (f *StudyFilter) HasAll() bool {
	return f.StudyDate != nil && f.Surgeon != "" && f.StudyType != ""
}

// HasOnlyDate - только дата
func (f *StudyFilter) HasOnlyDate() bool {
	return f.StudyDate != nil && f.Surgeon == "" && f.StudyType == ""
}

// HasOnlySurgeon - только хирург
func (f *StudyFilter) HasOnlySurgeon() bool {
	return f.Surgeon != "" && f.StudyDate == nil && f.StudyType == ""
}

// HasOnlyType - только тип
func (f *StudyFilter) HasOnlyType() bool {
	return f.StudyType != "" && f.StudyDate == nil && f.Surgeon == ""
}
