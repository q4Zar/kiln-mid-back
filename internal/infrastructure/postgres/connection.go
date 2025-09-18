package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/q4ZAr/kiln-mid-back/tezos-delegation-service/pkg/config"
	"github.com/q4ZAr/kiln-mid-back/tezos-delegation-service/pkg/logger"
)

func NewConnection(cfg *config.Database, logger *logger.Logger) (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ConnectionTimeout)
	defer cancel()

	poolConfig, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	poolConfig.MaxConns = int32(cfg.MaxConnections)
	poolConfig.MaxConnIdleTime = cfg.MaxIdleTime
	poolConfig.ConnConfig.ConnectTimeout = cfg.ConnectionTimeout

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Info("Successfully connected to PostgreSQL database")

	return pool, nil
}

func RunMigrations(pool *pgxpool.Pool, logger *logger.Logger) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	migrations := []string{
		`CREATE TABLE IF NOT EXISTS delegations (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
			amount TEXT NOT NULL,
			delegator TEXT NOT NULL,
			level TEXT NOT NULL,
			block_hash TEXT NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			UNIQUE(delegator, level)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_delegations_timestamp ON delegations(timestamp DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_delegations_delegator ON delegations(delegator)`,
		`CREATE INDEX IF NOT EXISTS idx_delegations_level ON delegations(level)`,
		`CREATE INDEX IF NOT EXISTS idx_delegations_created_at ON delegations(created_at DESC)`,
		`CREATE TABLE IF NOT EXISTS indexing_metadata (
			id SERIAL PRIMARY KEY,
			last_indexed_level BIGINT NOT NULL DEFAULT 0,
			last_indexed_timestamp TIMESTAMP WITH TIME ZONE,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,
		`INSERT INTO indexing_metadata (id, last_indexed_level, last_indexed_timestamp)
		VALUES (1, 0, NULL)
		ON CONFLICT (id) DO NOTHING`,
	}

	for i, migration := range migrations {
		if _, err := pool.Exec(ctx, migration); err != nil {
			return fmt.Errorf("failed to run migration %d: %w", i+1, err)
		}
	}

	logger.Info("Successfully ran database migrations")
	return nil
}
