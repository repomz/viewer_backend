package httpserver

// HttpServer is a HTTP server for ports
type HttpServer struct {
	bookService BookService
}

// NewHttpServer creates a new HTTP server for ports
func NewHttpServer(bookService BookService) HttpServer {
	return HttpServer{
		bookService: bookService,
	}
}
