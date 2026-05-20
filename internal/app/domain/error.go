package domain

import "errors"

var (
	ErrRequired        = errors.New("required value")
	ErrNotFound        = errors.New("not found")
	ErrNil             = errors.New("nil data")
	ErrNegative        = errors.New("negative value")
	ErrInvalidUserID   = errors.New("invalid user ID")
	ErrInvalidStudyIDs = errors.New("invalid study IDs")
	ErrNoUserInContext = errors.New("no user in context")

	ErrPatientNameTooShort = errors.New("patient name too short")
	ErrPatientNameInvalid  = errors.New("patient name contains invalid characters")
	ErrSurgeonNotFound     = errors.New("surgeon not found")
	ErrStudyTypeNotFound   = errors.New("study type not found")
)
