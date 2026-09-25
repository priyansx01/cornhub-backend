// Package database provides PostgreSQL connectivity.
package database

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"

	"github.com/priyansx01/corn-hub-clone/internal/config"
)

// Connect establishes a PostgreSQL connection pool.
func Connect(cfg config.DBConfig) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("database open: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(30 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("database ping: %w", err)
	}

	log.Printf("✓ Connected to PostgreSQL (%s:%s/%s)", cfg.Host, cfg.Port, cfg.Name)
	return db, nil
}
