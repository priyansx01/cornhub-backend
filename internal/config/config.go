// Package config loads application configuration from environment variables.
// All services read their config through this package — single source of truth.
package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all application configuration.
type Config struct {
	App        AppConfig
	DB         DBConfig
	Redis      RedisConfig
	JWT        JWTConfig
	MinIO      MinIOConfig
	Kafka      KafkaConfig
	ClickHouse ClickHouseConfig
	CORS       CORSConfig
}

type ClickHouseConfig struct {
	Addr     string
	User     string
	Password string
	Database string
}

type AppConfig struct {
	Name string
	Env  string
	Port string
}

type DBConfig struct {
	Host         string
	Port         string
	User         string
	Password     string
	Name         string
	SSLMode      string
	MaxOpenConns int
	MaxIdleConns int
}

// DSN returns a PostgreSQL connection string.
func (c DBConfig) DSN() string {
	return "host=" + c.Host +
		" port=" + c.Port +
		" user=" + c.User +
		" password=" + c.Password +
		" dbname=" + c.Name +
		" sslmode=" + c.SSLMode
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type JWTConfig struct {
	Secret     string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
}

type MinIOConfig struct {
	Endpoint         string
	AccessKey        string
	SecretKey        string
	UseSSL           bool
	RawBucket        string
	HLSBucket        string
	ThumbnailsBucket string
	// PublicURL is the base URL players use to fetch HLS/thumbnail objects
	// (e.g. https://cdn.example.com); it may differ from the internal Endpoint.
	PublicURL string
}

type KafkaConfig struct {
	Brokers string
}

type CORSConfig struct {
	Origins string
}

// Load reads all config from environment variables with sensible defaults.
func Load() *Config {
	return &Config{
		App: AppConfig{
			Name: getEnv("APP_NAME", "smartfm-lms"),
			Env:  getEnv("APP_ENV", "development"),
			Port: getEnv("APP_PORT", "8080"),
		},
		DB: DBConfig{
			Host:         getEnv("DB_HOST", "localhost"),
			Port:         getEnv("DB_PORT", "5432"),
			User:         getEnv("DB_USER", "lms"),
			Password:     getEnv("DB_PASSWORD", "lms_secret"),
			Name:         getEnv("DB_NAME", "smartfm_lms"),
			SSLMode:      getEnv("DB_SSL_MODE", "disable"),
			MaxOpenConns: getEnvInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns: getEnvInt("DB_MAX_IDLE_CONNS", 5),
		},
		Redis: RedisConfig{
			Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 0),
		},
		JWT: JWTConfig{
			Secret:     getEnv("JWT_SECRET", "change-me-in-production"),
			AccessTTL:  getEnvDuration("JWT_ACCESS_TTL", 24*time.Hour),
			RefreshTTL: getEnvDuration("JWT_REFRESH_TTL", 30*24*time.Hour),
		},
		MinIO: MinIOConfig{
			Endpoint:         getEnv("MINIO_ENDPOINT", "localhost:9000"),
			AccessKey:        getEnv("MINIO_ACCESS_KEY", "minioadmin"),
			SecretKey:        getEnv("MINIO_SECRET_KEY", "minioadmin"),
			UseSSL:           getEnvBool("MINIO_USE_SSL", false),
			RawBucket:        getEnv("MINIO_RAW_BUCKET", "lms-raw-videos"),
			HLSBucket:        getEnv("MINIO_HLS_BUCKET", "lms-hls-videos"),
			ThumbnailsBucket: getEnv("MINIO_THUMBNAILS_BUCKET", "lms-thumbnails"),
			PublicURL:        getEnv("MINIO_PUBLIC_URL", "http://localhost:9000"),
		},
		Kafka: KafkaConfig{
			Brokers: getEnv("KAFKA_BROKERS", "localhost:9092"),
		},
		ClickHouse: ClickHouseConfig{
			Addr:     getEnv("CLICKHOUSE_ADDR", "localhost:9004"),
			User:     getEnv("CLICKHOUSE_USER", "lms"),
			Password: getEnv("CLICKHOUSE_PASSWORD", "lms_secret"),
			Database: getEnv("CLICKHOUSE_DB", "smartfm_analytics"),
		},
		CORS: CORSConfig{
			Origins: getEnv("CORS_ORIGINS", "http://localhost:3000"),
		},
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if val := os.Getenv(key); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			return b
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if val := os.Getenv(key); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
	}
	return fallback
}
