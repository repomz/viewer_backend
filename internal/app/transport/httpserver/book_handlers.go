package httpserver

import (
	"net/http"
	"strconv"

	"github.com/IdrisovMarat/viewer_backend/internal/app/common/server"
)

func (h HttpServer) GetBooks(w http.ResponseWriter, r *http.Request) {
	// filter by category IDs
	queryCategoryIDs := r.URL.Query()["category_id"]
	var categoryIDs []int
	for _, id := range queryCategoryIDs {
		categoryID, err := strconv.Atoi(id)
		if err != nil {
			server.BadRequest("invalid-category-id", err, w, r)
			return
		}
		categoryIDs = append(categoryIDs, categoryID)
	}
	// page
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil {
		page = 1
	}
	var limit, offset int
	if page > 0 {
		limit = 10
		offset = (page - 1) * limit
	}

	books, err := h.bookService.GetBooks(r.Context(), categoryIDs, limit, offset)
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}

	response := make([]BookResponse, 0, len(books))
	for _, book := range books {
		response = append(response, toResponseBook(book))
	}

	server.RespondOK(response, w, r)
}
