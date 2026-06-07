package httpserver

// HttpServer is a HTTP server for ports
type HttpServer struct {
	studyService        StudyService
	agentRecordsService AgentRecordsService
}

// NewHttpServer creates a new HTTP server for ports
func NewHttpServer(studyService StudyService, agentRecordsService AgentRecordsService) HttpServer {
	return HttpServer{
		studyService:        studyService,
		agentRecordsService: agentRecordsService,
	}
}
