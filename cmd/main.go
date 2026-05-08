package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"

	"github.com/IdrisovMarat/viewer_backend/internal/app/config"
	"github.com/IdrisovMarat/viewer_backend/internal/app/repository/db"
	"github.com/IdrisovMarat/viewer_backend/internal/app/repository/pgrepo"
	"github.com/IdrisovMarat/viewer_backend/internal/app/services"
	"github.com/IdrisovMarat/viewer_backend/internal/app/transport/httpserver"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
	os.Exit(0)
}

func run() error {
	// read config from env
	cfg := config.Read()

	pgDB, err := db.Dial(cfg.DB_DSN)
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

	bookRepo := pgrepo.NewBookRepo(pgDB)

	bookService := services.NewBookService(bookRepo)

	// create http server with application injected
	httpServer := httpserver.NewHttpServer(bookService)

	// create http router
	router := mux.NewRouter()
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("Book Shop API v0.1"))
	}).Methods("GET")

	router.HandleFunc("/books", httpServer.GetBooks).Methods(http.MethodGet)

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
