package httpserver

import (
	"fmt"
	"time"

	"github.com/IdrisovMarat/viewer_backend/internal/app/domain"
	"github.com/google/uuid"
)

type BookRequest struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
}

func (r *BookRequest) Validate() error {
	if r.Body == "" {
		return fmt.Errorf("%w: body", domain.ErrRequired)
	}
	// if r.Year <= 0 {
	// 	return fmt.Errorf("%w: year", domain.ErrNegative)
	// }
	// if r.Author == "" {
	// 	return fmt.Errorf("%w: author", domain.ErrRequired)
	// }
	return nil
}

type BookResponse struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
}
