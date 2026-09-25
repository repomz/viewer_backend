package httpserver

import "database/sql"

import (
	"os"
	"strings"
)

// HttpServer is a HTTP server for ports
type HttpServer struct {
	studyService        StudyService
	agentRecordsService AgentRecordsService
	userRequestService  UserRequestService
	agentLogService     AgentLogService
	xaCache             *XACache
	sqlDB               *sql.DB
	driveDir            string
	driveCloud          *yandexArchive
}

// SetPlatformServices enables authentication, metrics and personal file storage.
func (h *HttpServer) SetPlatformServices(database *sql.DB, driveDir string) {
	h.sqlDB = database
	h.driveDir = driveDir
	h.driveCloud = newYandexArchiveFromEnvironment()
	if h.driveCloud != nil {
		if bucket := strings.TrimSpace(os.Getenv("DRIVE_YANDEX_BUCKET")); bucket != "" {
			h.driveCloud.bucket = bucket
		}
	}
}

// SetAgentLogService enables collection and browsing of hospital-agent logs.
func (h *HttpServer) SetAgentLogService(service AgentLogService) {
	h.agentLogService = service
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
