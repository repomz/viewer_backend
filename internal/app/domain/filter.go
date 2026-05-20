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
