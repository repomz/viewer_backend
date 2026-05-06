package pgrepo

import (
	"github.com/IdrisovMarat/viewer_backend/internal/app/domain"
	"github.com/IdrisovMarat/viewer_backend/internal/app/repository/db"
)

func domainToBook(book domain.Book) db.Book {
	return db.Book{
		ID:        book.ID(),
		CreatedAt: book.CreatedAt(),
		UpdatedAt: book.UpdatedAt(),
		Body:      book.Body(),
	}
}

func bookToDomain(book db.Book) (domain.Book, error) {
	return domain.NewBook(domain.NewBookData{
		ID:        book.ID,
		CreatedAt: book.CreatedAt,
		UpdatedAt: book.UpdatedAt,
		Body:      book.Body,
	})
}
