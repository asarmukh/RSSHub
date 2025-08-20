package db

import (
	"database/sql"
	"fmt"
	"rsshub/internal/config"
	"time"
)

func OpenDB(cfg config.Config) (*sql.DB, error) {
	pgURL := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.PGUser, cfg.PGPassword, cfg.PGHost, cfg.PGPort, cfg.PGDatabase,
	)
	dbConn, err := sql.Open("postgres", pgURL)
	if err != nil {
		return nil, err
	}
	dbConn.SetMaxOpenConns(10)
	dbConn.SetMaxIdleConns(10)
	dbConn.SetConnMaxLifetime(30 * time.Minute)
	if err := dbConn.Ping(); err != nil {
		return nil, err
	}
	return dbConn, nil
}
