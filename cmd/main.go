package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/mux"

	"github.com/repomz/viewer_backend/internal/app/config"
	"github.com/repomz/viewer_backend/internal/app/db"
	"github.com/repomz/viewer_backend/internal/app/repository/pgrepo"
	"github.com/repomz/viewer_backend/internal/app/services"
	"github.com/repomz/viewer_backend/internal/app/transport/httpserver"

	_ "github.com/lib/pq"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func Dial(ctx context.Context, dsn string) (*sql.DB, *db.Queries, error) {
	if dsn == "" {
		return nil, nil, errors.New("no postgres DSN provided")
	}

	dbase, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("sql.Open failed: %w", err)
	}

	dbase.SetMaxOpenConns(20)
	dbase.SetMaxIdleConns(10)
	dbase.SetConnMaxLifetime(5 * time.Minute)
	dbase.SetConnMaxIdleTime(1 * time.Minute)

	if err := dbase.PingContext(ctx); err != nil {
		_ = dbase.Close()
		return nil, nil, fmt.Errorf("postgres ping failed: %w", err)
	}

	return dbase, db.New(dbase), nil
}

func run() error {
	// read config from env
	cfg := config.Read()
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	dialCtx, cancelDial := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelDial()
	sqlDB, pgDB, err := Dial(dialCtx, cfg.DB_DSN)
	if err != nil {
		return fmt.Errorf("pg.Dial failed: %w", err)
	}
	defer sqlDB.Close()

	// run Postgres migrations
	// if pgDB != nil {
	// 	log.Println("Running PostgreSQL migrations")
	// 	if err := runPgMigrations(cfg.DSN, cfg.MigrationsPath); err != nil {
	// 		return fmt.Errorf("runPgMigrations failed: %w", err)
	// 	}
	// }

	// create repositories

	studyRepo := pgrepo.NewStudyRepo(pgDB)
	agentRecordRepo := pgrepo.NewAgentRecordRepo(pgDB)
	userRequestRepo := pgrepo.NewUserRequestRepo(pgDB)

	studyService := services.NewStudyService(studyRepo)
	agentRecordsService := services.NewAgentRecordsService(agentRecordRepo)
	userRequestService := services.NewUserRequestService(userRequestRepo)

	// create http server with application injected
	httpServer := httpserver.NewHttpServer(studyService, agentRecordsService, userRequestService)
	xaCache, err := httpserver.NewXACacheFromEnvironment()
	if err != nil {
		return fmt.Errorf("initialize XA cache: %w", err)
	}
	httpServer.SetXACache(xaCache)
	if strings.EqualFold(strings.TrimSpace(os.Getenv("XA_CACHE_WARM_EXISTING")), "true") {
		go xaCache.WarmExisting(context.Background())
		go httpServer.WarmCurrentXACache(context.Background())
	}
	retentionCtx, cancelRetention := context.WithCancel(context.Background())
	defer cancelRetention()
	if strings.EqualFold(strings.TrimSpace(os.Getenv("STUDY_RETENTION_ENABLED")), "true") {
		go httpServer.StartStudyRetention(retentionCtx)
	}
	go httpServer.StartReportScheduler(retentionCtx)

	// create http router
	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("DICOM viewer API v0.1"))
	}).Methods("GET")

	router.HandleFunc("/studies", httpServer.GetAllStudies).Methods(http.MethodGet)
	router.HandleFunc("/studies", httpServer.DeleteAllStudies).Methods(http.MethodDelete)
	router.HandleFunc("/studies", httpServer.CreateStudy).Methods(http.MethodPost)
	router.HandleFunc("/studies/search", httpServer.GetStudiesByFilter).Methods(http.MethodGet)
	router.HandleFunc("/studies/suggest", httpServer.SuggestProtocolStudies).Methods(http.MethodGet)
	router.HandleFunc("/studies/patient/{patient}", httpServer.GetStudyByPatient).Methods(http.MethodGet)
	router.HandleFunc("/studies/{study_id}", httpServer.GetStudyByID).Methods(http.MethodGet)
	router.HandleFunc("/studies/{study_id}/dicom-link", httpServer.UpdateStudy).Methods(http.MethodPatch)
	router.HandleFunc("/studies/{study_id}", httpServer.DeleteStudy).Methods(http.MethodDelete)
	router.HandleFunc("/ct_studies", httpServer.CreateCTStudy).Methods(http.MethodPost)
	router.HandleFunc("/xa_studies", httpServer.CreateXAStudy).Methods(http.MethodPost)
	router.HandleFunc("/xa-cache/{study_uid}/manifest", httpServer.GetXACacheManifest).Methods(http.MethodGet)
	router.HandleFunc("/xa-cache/{study_uid}/prepare", httpServer.PrepareXACache).Methods(http.MethodPost)
	router.HandleFunc("/xa-cache/{study_uid}/archive", httpServer.GetXACacheArchive).Methods(http.MethodGet)
	router.HandleFunc("/xa-cache/{study_uid}/series/{cine_id}", httpServer.GetXACacheCine).Methods(http.MethodGet)
	router.HandleFunc("/xa-cache/{study_uid}/frames/{frame_id}", httpServer.GetXACacheFrame).Methods(http.MethodGet)
	router.HandleFunc("/pacs/studies/{study_uid}", httpServer.DeletePACSStudy).Methods(http.MethodDelete)
	router.HandleFunc("/reports/generate", httpServer.GenerateReport).Methods(http.MethodPost)
	router.HandleFunc("/reports", httpServer.GetReports).Methods(http.MethodGet)
	router.HandleFunc("/reports/{filename}", httpServer.GetReport).Methods(http.MethodGet)
	router.HandleFunc("/reports/{filename}", httpServer.DeleteReport).Methods(http.MethodDelete)
	router.HandleFunc("/operation-plan", httpServer.GetOperationPlan).Methods(http.MethodGet)
	router.HandleFunc("/operation-plan/{date}", httpServer.PutOperationPlanDay).Methods(http.MethodPut)
	router.HandleFunc("/duty-schedule/{month}", httpServer.GetDutySchedule).Methods(http.MethodGet)
	router.HandleFunc("/duty-schedule/{month}", httpServer.PutDutySchedule).Methods(http.MethodPut)
	router.HandleFunc("/statistics/operations", httpServer.GetOperationStatistics).Methods(http.MethodGet)
	router.HandleFunc("/statistics/vmp", httpServer.PutVMPStatisticsConfig).Methods(http.MethodPut)
	router.HandleFunc("/statistics/history", httpServer.GetHistoricalStatistics).Methods(http.MethodGet)
	router.HandleFunc("/statistics/history", httpServer.PutHistoricalStatistics).Methods(http.MethodPut)

	// Backward-compatible singular routes.
	router.HandleFunc("/study/{study_id}", httpServer.GetStudyByID).Methods(http.MethodGet)
	router.HandleFunc("/study/patient/{patient}", httpServer.GetStudyByPatient).Methods(http.MethodGet)
	router.HandleFunc("/study", httpServer.CreateStudy).Methods(http.MethodPost)
	router.HandleFunc("/study/{study_id}", httpServer.UpdateStudy).Methods(http.MethodPatch)
	router.HandleFunc("/study/{study_id}", httpServer.DeleteStudy).Methods(http.MethodDelete)

	router.HandleFunc("/agent_status", httpServer.CreateAgentRecord).Methods(http.MethodPost)
	router.HandleFunc("/agents", httpServer.GetAgents).Methods(http.MethodGet)
	router.HandleFunc("/agent_status/{agent_id}", httpServer.DeleteAllAgentRecords).Methods(http.MethodDelete)
	router.HandleFunc("/agent_status/searchby_id", httpServer.GetAgentRecordsByAgentID).Methods(http.MethodGet)
	router.HandleFunc("/agent_status/searchby_status", httpServer.GetAgentRecordsByAgentIDandStatus).Methods(http.MethodGet)

	router.HandleFunc("/user_requests", httpServer.CreateUserRequest).Methods(http.MethodPost)
	router.HandleFunc("/user_requests", httpServer.ClaimUserRequest).Methods(http.MethodGet)
	router.HandleFunc("/user_requests/history", httpServer.ListUserRequests).Methods(http.MethodGet)
	router.HandleFunc("/user_requests/history", httpServer.DeleteAllUserRequests).Methods(http.MethodDelete)
	router.HandleFunc("/user_requests/{request_id}", httpServer.GetUserRequest).Methods(http.MethodGet)
	router.HandleFunc("/user_requests/{request_id}", httpServer.DeleteUserRequest).Methods(http.MethodDelete)
	router.HandleFunc("/user_requests/{request_id}/result", httpServer.RecordUserRequestResult).Methods(http.MethodPost)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Minute,
		IdleTimeout:       60 * time.Second,
	}

	// listen to OS signals and gracefully shutdown HTTP server
	stopped := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
		<-sigint
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("HTTP Server Shutdown Error: %v", err)
		}
		close(stopped)
	}()

	log.Printf("Starting HTTP server on %s", cfg.HTTPAddr)

	// start HTTP server
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return fmt.Errorf("HTTP server ListenAndServe: %w", err)
	}

	<-stopped

	log.Printf("Have a nice day!")

	return nil
}

// runPgMigrations runs Postgres migrations
// func runPgMigrations(dsn, path string) error {
// 	if path == "" {
// 		return errors.New("no migrations path provided")
// 	}
// 	if dsn == "" {
// 		return errors.New("no DSN provided")
// 	}

// 	m, err := migrate.New(
// 		path,
// 		dsn,
// 	)
// 	if err != nil {
// 		return err
// 	}

// 	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
// 		return err
// 	}

// 	return nil
// }
