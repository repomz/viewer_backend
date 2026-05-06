package services

import (
	"context"

	"github.com/IdrisovMarat/viewer_backend/internal/app/domain"
)

type BookRepository interface {
	GetBooks(ctx context.Context, categoryIDs []int, limit, offset int) ([]domain.Book, error)
	// 	GetBook(ctx context.Context, id int) (domain.Book, error)
	// 	CreateBook(ctx context.Context, book domain.Book) (domain.Book, error)
	// 	UpdateBook(ctx context.Context, book domain.Book) (domain.Book, error)
	// 	DeleteBook(ctx context.Context, id int) error
}
