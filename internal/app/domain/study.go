package domain

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/repomz/viewer_backend/internal/app/db"
)

// Study is a domain study.
type Study struct {
	id             uuid.UUID
	studyID        string
	patient        string
	age            int32
	department     string
	nameOperation  string
	descrOperation string
	timeBeginning  time.Time
	timeDuration   int32
	surgeon        string
	dicomLink      string
	createdAt      time.Time
	updatedAt      time.Time
}

type DBStudyData struct {
	ID             uuid.UUID
	StudyID        string
	Patient        string
	Age            int32
	Department     string
	NameOperation  string
	DescrOperation string
	TimeBeginning  time.Time
	TimeDuration   int32
	Surgeon        string
	DicomLink      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// NewStudy creates a new domain Study from response
func ResponseToDBStudy(data DBStudyData) Study {
	return Study{
		id:             data.ID,
		createdAt:      data.CreatedAt,
		updatedAt:      data.UpdatedAt,
		studyID:        data.StudyID,
		patient:        data.Patient,
		age:            data.Age,
		department:     data.Department,
		nameOperation:  data.NameOperation,
		descrOperation: data.DescrOperation,
		timeBeginning:  data.TimeBeginning,
		timeDuration:   data.TimeDuration,
		surgeon:        data.Surgeon,
		dicomLink:      data.DicomLink,
	}
}

// NewStudy creates a new domain Study from db
func DBToNewStudy(data db.Study) (Study, error) {
	return Study{
		id:             data.ID,
		createdAt:      data.CreatedAt,
		updatedAt:      data.UpdatedAt,
		studyID:        data.StudyID,
		patient:        data.Patient,
		age:            data.Age.Int32,
		department:     data.Department,
		nameOperation:  data.NameOperation,
		descrOperation: data.DescrOperation,
		timeBeginning:  data.TimeBeginning.Time,
		timeDuration:   data.TimeDuration.Int32,
		surgeon:        data.Surgeon,
		dicomLink:      data.DicomLink.String,
	}, nil
}

// These methods return the db.Study
func (b Study) ID() uuid.UUID {
	return b.id
}

func (b Study) CreatedAt() time.Time {
	return b.createdAt
}

func (b Study) UpdatedAt() time.Time {
	return b.updatedAt
}

func (b Study) StudyID() string {
	return b.studyID
}

func (b Study) Patient() string {
	return b.patient
}

func (b Study) Age() sql.NullInt32 {
	return sql.NullInt32{
		Int32: b.age,
		Valid: b.age != 0, // 0 трактуется как отсутствие данных (NULL)
	}
}

func (b Study) Department() string {
	return b.department
}

func (b Study) NameOperation() string {
	return b.nameOperation
}

func (b Study) Descroperation() string {
	return b.descrOperation
}

func (b Study) TimeBeginning() sql.NullTime {
	return sql.NullTime{
		Time:  b.timeBeginning,
		Valid: !b.timeBeginning.IsZero(), // Valid = true, если дата была установлена
	}
}

func (b Study) TimeDuration() sql.NullInt32 {
	return sql.NullInt32{Int32: b.timeDuration, Valid: b.timeDuration != 0}
}

func (b Study) Surgeon() string {
	return b.surgeon
}

func (b Study) DicomLink() sql.NullString {
	return sql.NullString{String: b.dicomLink, Valid: b.dicomLink != ""}
}
