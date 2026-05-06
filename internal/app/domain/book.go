package domain

import (
	"time"

	"github.com/google/uuid"
)

// Book is a domain book.
type Book struct {
	id        uuid.UUID
	createdAt time.Time
	updatedAt time.Time
	body      string
}

type NewBookData struct {
	ID        uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
	Body      string
}

// NewBook creates a new book.
func NewBook(data NewBookData) (Book, error) {
	return Book{
		id:        data.ID,
		createdAt: data.CreatedAt,
		updatedAt: data.UpdatedAt,
		body:      data.Body,
	}, nil
}

// ID returns the book ID.
func (b Book) ID() uuid.UUID {
	return b.id
}

// Title returns the book title.
func (b Book) CreatedAt() time.Time {
	return b.createdAt
}

// Year returns the book year.
func (b Book) UpdatedAt() time.Time {
	return b.updatedAt
}

// Author returns the book author.
func (b Book) Body() string {
	return b.body
}
