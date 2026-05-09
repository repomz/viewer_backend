package domain

import (
	"time"

	"github.com/google/uuid"
)

// Study is a domain study.
type Study struct {
	id             uuid.UUID
	studyID        string
	patient        string
	age            int
	department     string
	nameOperation  string
	descrOperation string
	timeBeginning  time.Time
	timeDuration   int
	surgeon        string
	dicomLink      string
	createdAt      time.Time
	updatedAt      time.Time
}

type NewStudyData struct {
	ID             uuid.UUID
	StudyID        string
	Patient        string
	Age            int
	Department     string
	NameOperation  string
	DescrOperation string
	TimeBeginning  time.Time
	TimeDuration   int
	Surgeon        string
	DicomLink      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// NewStudy creates a new Study.
func NewStudy(data NewStudyData) (Study, error) {
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
	}, nil
}

// ID returns the Study ID.
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

func (b Study) Age() int {
	return b.age
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

func (b Study) TimeBeginning() time.Time {
	return b.timeBeginning
}

func (b Study) TimeDuration() int {
	return b.timeDuration
}

func (b Study) Surgeon() string {
	return b.surgeon
}

func (b Study) DicomLink() string {
	return b.dicomLink
}
