package domain

import "time"

// PatientFilter для поиска исследованиz по фамилии пациента
type PatientFilter struct {
	Patient string
}

// StudyFilter для фильтрации исследований по дате, хирургу или типу операции
type StudyFilter struct {
	StudyDate *time.Time
	Surgeon   string
	StudyType string
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
