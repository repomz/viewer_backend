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
	os.Exit(0)
}

func Dial(dsn string) (*db.Queries, error) {
	if dsn == "" {
		return nil, errors.New("no postgres DSN provided")
	}

	dbase, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("sql.Open failed: %w", err)
	}

	dbase.SetMaxIdleConns(10)
	dbase.SetMaxIdleConns(10)
	dbase.SetConnMaxLifetime(1 * time.Minute)

	dbQueries := db.New(dbase)

	return dbQueries, nil
}

func run() error {
	// read config from env
	cfg := config.Read()

	pgDB, err := Dial(cfg.DB_DSN)
	if err != nil {
		return fmt.Errorf("pg.Dial failed: %w", err)
	}

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

	studyService := services.NewStudyService(studyRepo)
	agentRecordsService := services.NewAgentRecordsService(agentRecordRepo)

	// create http server with application injected
	httpServer := httpserver.NewHttpServer(studyService, agentRecordsService)

	// create http router
	router := mux.NewRouter()
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("DICOM viewer API v0.1"))
	}).Methods("GET")

	router.HandleFunc("/studies", httpServer.GetAllStudies).Methods(http.MethodGet)
	router.HandleFunc("/studies", httpServer.DeleteAllStudies).Methods(http.MethodDelete)
	router.HandleFunc("/study/{study_id}", httpServer.GetStudyByID).Methods(http.MethodGet)
	router.HandleFunc("/study/patient/{patient}", httpServer.GetStudyByPatient).Methods(http.MethodGet)
	router.HandleFunc("/studies/search", httpServer.GetStudiesByFilter).Methods(http.MethodGet)
	router.HandleFunc("/study", httpServer.CreateStudy).Methods(http.MethodPost)
	router.HandleFunc("/study/{study_id}", httpServer.UpdateStudy).Methods(http.MethodPatch)
	router.HandleFunc("/study/{study_id}", httpServer.DeleteStudy).Methods(http.MethodDelete)

	router.HandleFunc("/agent_status", httpServer.CreateAgentRecord).Methods(http.MethodPost)
	router.HandleFunc("/agent_status/{agent_id}", httpServer.DeleteAllAgentRecords).Methods(http.MethodDelete)
	router.HandleFunc("/agent_status/search", httpServer.GetAgentRecordsByAgentID).Methods(http.MethodGet)
	router.HandleFunc("/agent_status/search", httpServer.GetAgentRecordsByAgentIDandStatus).Methods(http.MethodGet)

	srv := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: router,
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
		log.Fatalf("HTTP server ListenAndServe Error: %v", err)
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
