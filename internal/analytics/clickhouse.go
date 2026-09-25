package analytics

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/priyansx01/corn-hub-clone/internal/config"
)

// ConnectClickHouse establishes a connection to ClickHouse and initializes the schema.
func ConnectClickHouse(cfg config.ClickHouseConfig) (driver.Conn, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{cfg.Addr},
		Auth: clickhouse.Auth{
			Database: "default", // Connect to default first to create our DB
			Username: cfg.User,
			Password: cfg.Password,
		},
		DialTimeout:     5 * time.Second,
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: time.Hour,
	})
	if err != nil {
		return nil, fmt.Errorf("clickhouse open: %w", err)
	}

	ctx := context.Background()
	if err := conn.Ping(ctx); err != nil {
		return nil, fmt.Errorf("clickhouse ping: %w", err)
	}

	// Create Database
	if err := conn.Exec(ctx, fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", cfg.Database)); err != nil {
		return nil, fmt.Errorf("create clickhouse database: %w", err)
	}

	// Reconnect to the specific database
	conn, err = clickhouse.Open(&clickhouse.Options{
		Addr: []string{cfg.Addr},
		Auth: clickhouse.Auth{
			Database: cfg.Database,
			Username: cfg.User,
			Password: cfg.Password,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("reconnect to specific clickhouse db: %w", err)
	}

	// Initialize tables
	if err := initSchema(ctx, conn); err != nil {
		return nil, fmt.Errorf("init clickhouse schema: %w", err)
	}

	log.Printf("✓ Connected to ClickHouse (%s/%s)", cfg.Addr, cfg.Database)
	return conn, nil
}

func initSchema(ctx context.Context, conn driver.Conn) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS video_watched (
			user_id UUID,
			course_id UUID,
			module_id UUID,
			watch_pct Float32,
			timestamp DateTime
		) ENGINE = MergeTree()
		ORDER BY (course_id, module_id, timestamp)`,

		`CREATE TABLE IF NOT EXISTS quiz_attempted (
			user_id UUID,
			course_id UUID,
			score Int32,
			passed UInt8,
			timestamp DateTime
		) ENGINE = MergeTree()
		ORDER BY (course_id, timestamp)`,

		`CREATE TABLE IF NOT EXISTS drop_off_at (
			user_id UUID,
			module_id UUID,
			seconds_watched Int32,
			timestamp DateTime
		) ENGINE = MergeTree()
		ORDER BY (module_id, timestamp)`,
	}

	for _, q := range queries {
		if err := conn.Exec(ctx, q); err != nil {
			return err
		}
	}

	return nil
}
