package httpserver

// HttpServer is a HTTP server for ports
type HttpServer struct {
	studyService        StudyService
	agentRecordsService AgentRecordsService
	userRequestService  UserRequestService
	xaCache             *XACache
}

// NewHttpServer creates a new HTTP server for ports
func NewHttpServer(
	studyService StudyService,
	agentRecordsService AgentRecordsService,
	userRequestServices ...UserRequestService,
) HttpServer {
	server := HttpServer{
		studyService:        studyService,
		agentRecordsService: agentRecordsService,
	}
	if len(userRequestServices) > 0 {
		server.userRequestService = userRequestServices[0]
	}
	return server
}

// SetXACache enables server-side preparation of XA cine frames.
func (h *HttpServer) SetXACache(cache *XACache) {
	h.xaCache = cache
}
