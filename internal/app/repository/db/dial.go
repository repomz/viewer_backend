package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Dial creates new database connection to postgres
func Dial(dsn string) (*Queries, error) {
	if dsn == "" {
		return nil, errors.New("no postgres DSN provided")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("sql.Open failed: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(1 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("db.Ping failed: %w", err)
	}

	dbQueries := New(db)

	return dbQueries, nil
}
