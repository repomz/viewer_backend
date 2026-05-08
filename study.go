package viewerbackend

import "time"

type StudyModel struct {
	ID            string    `db:"id" gorm:"primaryKey"`
	StudyID       string    `db:"study_id"`
	Patient       string    `db:"patient"`
	Age           int       `db:"age"`
	Department    string    `db:"department"`
	Operation     string    `db:"operation"`
	TimeBeginning time.Time `db:"time_beginning"`
	TimeDuration  int       `db:"time_duration"`
	Surgeon       string    `db:"surgeon"`
	DicomLink     string    `db:"dicom_link"`
	CreatedAt     time.Time `db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type Study struct {
	ID            string    `json:"id"`
	StudyID       string    `json:"study_id"`
	Patient       string    `json:"patient"`
	Age           int       `json:"age"`
	Department    string    `json:"department"`
	Operation     string    `json:"operation"`
	TimeBeginning time.Time `json:"time_beginning"`
	TimeDuration  int       `json:"time_duration"`
	Surgeon       string    `json:"surgeon"`
	DicomLink     string    `json:"dicom_link"`
	CreatedAt     time.Time `db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
