package httpserver

// HttpServer is a HTTP server for ports
type HttpServer struct {
	studyService StudyService
}

// NewHttpServer creates a new HTTP server for ports
func NewHttpServer(studyService StudyService) HttpServer {
	return HttpServer{
		studyService: studyService,
	}
}
