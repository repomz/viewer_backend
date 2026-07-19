package domain

import "errors"

var (
	ErrRequired         = errors.New("required value")
	ErrNotFound         = errors.New("not found")
	ErrNil              = errors.New("nil data")
	ErrNegative         = errors.New("negative value")
	ErrInvalidUserID    = errors.New("invalid user ID")
	ErrInvalidStudyIDs  = errors.New("invalid study IDs")
	ErrNoUserInContext  = errors.New("no user in context")
	ErrInvalidPatient   = errors.New("invalid patient")
	ErrInvalidAgentID   = errors.New("invalid agent ID")
	ErrInvalidSurgeon   = errors.New("invalid surgeon")
	ErrInvalidStudyType = errors.New("invalid study type")
	ErrInvalidStatus    = errors.New("invalid status")
	ErrInvalidCommand   = errors.New("invalid command")
	ErrInvalidRequest   = errors.New("invalid request")
	ErrConflict         = errors.New("conflict")
)
