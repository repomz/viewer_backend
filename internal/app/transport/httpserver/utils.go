package httpserver

import (
	"github.com/IdrisovMarat/viewer_backend/internal/app/domain"
)

func toResponseBook(book domain.Book) BookResponse {
	return BookResponse{
		ID:        book.ID(),
		CreatedAt: book.CreatedAt(),
		UpdatedAt: book.UpdatedAt(),
		Body:      book.Body(),
	}
}

func toDomainBook(bookRequest BookRequest) (domain.Book, error) {
	return domain.NewBook(domain.NewBookData{
		ID:        bookRequest.ID,
		CreatedAt: bookRequest.CreatedAt,
		UpdatedAt: bookRequest.UpdatedAt,
		Body:      bookRequest.Body,
	})
}
