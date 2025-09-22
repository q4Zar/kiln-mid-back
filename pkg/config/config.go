package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Database Database
	Server   Server
	TzktAPI  TzktAPI
	Logging  Logging
	Metrics  Metrics
}

type Database struct {
	URL               string
	MaxConnections    int
	MaxIdleTime       time.Duration
	ConnectionTimeout time.Duration
}

type Server struct {
	Port            string
	RequestTimeout  time.Duration
	ShutdownTimeout time.Duration
}

type TzktAPI struct {
	BaseURL             string
	PollingInterval     time.Duration
	HistoricalIndexing  bool
	HistoricalStartDate string
	MaxRetries          int
	RetryDelay          time.Duration
	RequestTimeout      time.Duration
}

type Logging struct {
	Level       string
	Environment string
}

type Metrics struct {
	Port    string
	Enabled bool
}

func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("error loading .env file: %w", err)
	}

	cfg := &Config{
		Database: Database{
			URL:               getEnv("DATABASE_URL", "postgres://tezos:tezos@localhost:5432/tezos_delegations?sslmode=disable"),
			MaxConnections:    getEnvAsInt("CONNECTION_POOL_SIZE", 20),
			MaxIdleTime:       getEnvAsDuration("CONNECTION_TIMEOUT", "30s"),
			ConnectionTimeout: getEnvAsDuration("CONNECTION_TIMEOUT", "30s"),
		},
		Server: Server{
			Port:            getEnv("SERVER_PORT", "8080"),
			RequestTimeout:  getEnvAsDuration("REQUEST_TIMEOUT", "60s"),
			ShutdownTimeout: getEnvAsDuration("SHUTDOWN_TIMEOUT", "10s"),
		},
		TzktAPI: TzktAPI{
			BaseURL:             getEnv("TZKT_API_URL", "https://api.tzkt.io"),
			PollingInterval:     getEnvAsDuration("POLLING_INTERVAL", "30s"),
			HistoricalIndexing:  getEnvAsBool("HISTORICAL_INDEXING", true),
			HistoricalStartDate: getEnv("HISTORICAL_START_DATE", "2021-01-01"),
			MaxRetries:          getEnvAsInt("MAX_RETRIES", 3),
			RetryDelay:          getEnvAsDuration("RETRY_DELAY", "5s"),
			RequestTimeout:      getEnvAsDuration("REQUEST_TIMEOUT", "60s"),
		},
		Logging: Logging{
			Level:       getEnv("LOG_LEVEL", "info"),
			Environment: getEnv("ENVIRONMENT", "development"),
		},
		Metrics: Metrics{
			Port:    getEnv("METRICS_PORT", "9090"),
			Enabled: getEnvAsBool("METRICS_ENABLED", true),
		},
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return defaultValue
}

func getEnvAsBool(key string, defaultValue bool) bool {
	valueStr := os.Getenv(key)
	if value, err := strconv.ParseBool(valueStr); err == nil {
		return value
	}
	return defaultValue
}

func getEnvAsDuration(key, defaultValue string) time.Duration {
	valueStr := getEnv(key, defaultValue)
	if duration, err := time.ParseDuration(valueStr); err == nil {
		return duration
	}
	defaultDuration, _ := time.ParseDuration(defaultValue)
	return defaultDuration
}
