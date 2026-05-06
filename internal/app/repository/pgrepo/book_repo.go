package pgrepo

import (
	"context"
	"fmt"

	"github.com/IdrisovMarat/viewer_backend/internal/app/domain"
	"github.com/IdrisovMarat/viewer_backend/internal/app/repository/db"
)

type BookRepo struct {
	query *db.Queries
}

func NewBookRepo(qr *db.Queries) *BookRepo {
	return &BookRepo{
		query: qr,
	}
}

func (r BookRepo) GetBooks(ctx context.Context, categoryIDs []int, limit, offset int) ([]domain.Book, error) {

	books, err := r.query.GetBooks(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get books: %w", err)
	}

	domainBooks := make([]domain.Book, len(books))
	for i, book := range books {
		domainBook, err := bookToDomain(book)
		if err != nil {
			return nil, fmt.Errorf("failed to create domain book: %w", err)
		}

		domainBooks[i] = domainBook
	}

	return domainBooks, nil
}
