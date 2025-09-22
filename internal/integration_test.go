// +build integration

package integration

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/lib/pq"
	"github.com/q4ZAr/kiln-mid-back/tezos-delegation-service/internal/application"
	"github.com/q4ZAr/kiln-mid-back/tezos-delegation-service/internal/domain"
	postgresRepo "github.com/q4ZAr/kiln-mid-back/tezos-delegation-service/internal/infrastructure/postgres"
	"github.com/q4ZAr/kiln-mid-back/tezos-delegation-service/internal/infrastructure/tzkt"
	"github.com/q4ZAr/kiln-mid-back/tezos-delegation-service/pkg/config"
	"github.com/q4ZAr/kiln-mid-back/tezos-delegation-service/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	postgresContainer "github.com/testcontainers/testcontainers-go/modules/postgres"
)

type TestSuite struct {
	container testcontainers.Container
	pool      *pgxpool.Pool
	repo      *postgresRepo.Repository
	service   *application.Service
	logger    *logger.Logger
}

func setupTestDB(t *testing.T) *TestSuite {
	ctx := context.Background()

	// Start PostgreSQL container
	container, err := postgresContainer.RunContainer(ctx,
		testcontainers.WithImage("docker.io/postgres:14-alpine"),
		postgresContainer.WithDatabase("testdb"),
		postgresContainer.WithUsername("testuser"),
		postgresContainer.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(postgresContainer.Wait),
	)
	require.NoError(t, err)

	// Get connection string
	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	// Connect to database
	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)

	// Run migrations
	err = runMigrations(connStr)
	require.NoError(t, err)

	// Create logger
	log, err := logger.New("debug", "test")
	require.NoError(t, err)

	// Create repository
	repo := postgresRepo.NewRepository(pool, log)

	// Create TzKT client (using mock for integration tests)
	mockTzkt := &MockTzktClient{}

	// Create service
	cfg := &config.TzktAPI{
		URL:             "https://api.tzkt.io",
		PollingInterval: 30 * time.Second,
	}
	service := application.NewService(repo, mockTzkt, cfg, log)

	return &TestSuite{
		container: container,
		pool:      pool,
		repo:      repo,
		service:   service,
		logger:    log,
	}
}

func (s *TestSuite) Cleanup(t *testing.T) {
	ctx := context.Background()
	
	if s.pool != nil {
		s.pool.Close()
	}
	
	if s.container != nil {
		err := s.container.Terminate(ctx)
		assert.NoError(t, err)
	}
}

func runMigrations(connStr string) error {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return err
	}
	defer db.Close()

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return err
	}

	migrationsPath := "file://../migrations"
	if _, err := os.Stat("../migrations"); os.IsNotExist(err) {
		// Try alternative path
		migrationsPath = "file://./migrations"
	}

	m, err := migrate.NewWithDatabaseInstance(
		migrationsPath,
		"postgres", driver)
	if err != nil {
		// Create tables manually if migrations not found
		return createTablesManually(db)
	}

	return m.Up()
}

func createTablesManually(db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS delegations (
			id UUID PRIMARY KEY,
			timestamp TIMESTAMP NOT NULL,
			amount VARCHAR(255) NOT NULL,
			delegator VARCHAR(255) NOT NULL,
			level VARCHAR(255) NOT NULL,
			block_hash VARCHAR(255) NOT NULL,
			operation_hash VARCHAR(255) UNIQUE,
			created_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_delegations_timestamp ON delegations(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_delegations_delegator ON delegations(delegator)`,
		`CREATE INDEX IF NOT EXISTS idx_delegations_level ON delegations(level)`,
		`CREATE TABLE IF NOT EXISTS indexing_metadata (
			id INTEGER PRIMARY KEY,
			last_indexed_level BIGINT DEFAULT 0,
			last_indexed_timestamp TIMESTAMP,
			updated_at TIMESTAMP DEFAULT NOW()
		)`,
		`INSERT INTO indexing_metadata (id) VALUES (1) ON CONFLICT DO NOTHING`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute query: %w", err)
		}
	}

	return nil
}

// Integration Tests

func TestIntegration_SaveAndRetrieveDelegation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	suite := setupTestDB(t)
	defer suite.Cleanup(t)

	// Create a delegation
	delegation := &domain.Delegation{
		ID:            uuid.New().String(),
		Timestamp:     time.Now(),
		Amount:        "1000000",
		Delegator:     "tz1abc123",
		Level:         "2338084",
		BlockHash:     "BlockHash1",
		OperationHash: uuid.New().String(),
		CreatedAt:     time.Now(),
	}

	// Save delegation
	err := suite.repo.Save(delegation)
	require.NoError(t, err)

	// Retrieve delegations
	delegations, err := suite.repo.FindAll(nil)
	require.NoError(t, err)
	assert.Len(t, delegations, 1)
	assert.Equal(t, delegation.Delegator, delegations[0].Delegator)
	assert.Equal(t, delegation.Amount, delegations[0].Amount)
}

func TestIntegration_SaveBatchDelegations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	suite := setupTestDB(t)
	defer suite.Cleanup(t)

	// Create multiple delegations
	delegations := []domain.Delegation{
		{
			ID:            uuid.New().String(),
			Timestamp:     time.Now(),
			Amount:        "1000000",
			Delegator:     "tz1abc123",
			Level:         "2338084",
			BlockHash:     "BlockHash1",
			OperationHash: uuid.New().String(),
		},
		{
			ID:            uuid.New().String(),
			Timestamp:     time.Now().Add(time.Hour),
			Amount:        "2000000",
			Delegator:     "tz1def456",
			Level:         "2338085",
			BlockHash:     "BlockHash2",
			OperationHash: uuid.New().String(),
		},
		{
			ID:            uuid.New().String(),
			Timestamp:     time.Now().Add(2 * time.Hour),
			Amount:        "3000000",
			Delegator:     "tz1ghi789",
			Level:         "2338086",
			BlockHash:     "BlockHash3",
			OperationHash: uuid.New().String(),
		},
	}

	// Save batch
	err := suite.repo.SaveBatch(delegations)
	require.NoError(t, err)

	// Retrieve all
	retrieved, err := suite.repo.FindAll(nil)
	require.NoError(t, err)
	assert.Len(t, retrieved, 3)
}

func TestIntegration_GetLastIndexedLevel(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	suite := setupTestDB(t)
	defer suite.Cleanup(t)

	// Initially should be 0
	level, err := suite.repo.GetLastIndexedLevel()
	require.NoError(t, err)
	assert.Equal(t, int64(0), level)

	// Add delegations
	delegations := []domain.Delegation{
		{
			ID:            uuid.New().String(),
			Timestamp:     time.Now(),
			Amount:        "1000000",
			Delegator:     "tz1abc123",
			Level:         "2338084",
			BlockHash:     "BlockHash1",
			OperationHash: uuid.New().String(),
		},
		{
			ID:            uuid.New().String(),
			Timestamp:     time.Now(),
			Amount:        "2000000",
			Delegator:     "tz1def456",
			Level:         "2338090",
			BlockHash:     "BlockHash2",
			OperationHash: uuid.New().String(),
		},
	}

	err = suite.repo.SaveBatch(delegations)
	require.NoError(t, err)

	// Should return highest level
	level, err = suite.repo.GetLastIndexedLevel()
	require.NoError(t, err)
	assert.Equal(t, int64(2338090), level)
}

func TestIntegration_GetDelegationsByTimeRange(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	suite := setupTestDB(t)
	defer suite.Cleanup(t)

	now := time.Now()
	
	// Add delegations at different times
	delegations := []domain.Delegation{
		{
			ID:            uuid.New().String(),
			Timestamp:     now.Add(-48 * time.Hour),
			Amount:        "1000000",
			Delegator:     "tz1abc123",
			Level:         "2338084",
			BlockHash:     "BlockHash1",
			OperationHash: uuid.New().String(),
		},
		{
			ID:            uuid.New().String(),
			Timestamp:     now.Add(-24 * time.Hour),
			Amount:        "2000000",
			Delegator:     "tz1def456",
			Level:         "2338085",
			BlockHash:     "BlockHash2",
			OperationHash: uuid.New().String(),
		},
		{
			ID:            uuid.New().String(),
			Timestamp:     now.Add(-12 * time.Hour),
			Amount:        "3000000",
			Delegator:     "tz1ghi789",
			Level:         "2338086",
			BlockHash:     "BlockHash3",
			OperationHash: uuid.New().String(),
		},
	}

	err := suite.repo.SaveBatch(delegations)
	require.NoError(t, err)

	// Query for last 36 hours
	start := now.Add(-36 * time.Hour)
	end := now
	
	retrieved, err := suite.repo.GetDelegationsByTimeRange(start, end)
	require.NoError(t, err)
	assert.Len(t, retrieved, 2) // Should only get the last 2 delegations
}

func TestIntegration_GetStats(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	suite := setupTestDB(t)
	defer suite.Cleanup(t)

	// Add test data
	delegations := []domain.Delegation{
		{
			ID:            uuid.New().String(),
			Timestamp:     time.Now(),
			Amount:        "1000000",
			Delegator:     "tz1abc123",
			Level:         "2338084",
			BlockHash:     "BlockHash1",
			OperationHash: uuid.New().String(),
		},
		{
			ID:            uuid.New().String(),
			Timestamp:     time.Now(),
			Amount:        "2000000",
			Delegator:     "tz1abc123", // Same delegator
			Level:         "2338085",
			BlockHash:     "BlockHash2",
			OperationHash: uuid.New().String(),
		},
		{
			ID:            uuid.New().String(),
			Timestamp:     time.Now(),
			Amount:        "3000000",
			Delegator:     "tz1def456",
			Level:         "2338086",
			BlockHash:     "BlockHash3",
			OperationHash: uuid.New().String(),
		},
	}

	err := suite.repo.SaveBatch(delegations)
	require.NoError(t, err)

	stats, err := suite.repo.GetStats()
	require.NoError(t, err)
	
	assert.Equal(t, int64(3), stats["total_delegations"])
	assert.Equal(t, int64(2), stats["unique_delegators"])
	assert.Equal(t, "6000000", stats["total_amount"])
	assert.Equal(t, int64(2338086), stats["last_indexed_level"])
}

func TestIntegration_ServiceGetDelegations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	suite := setupTestDB(t)
	defer suite.Cleanup(t)

	// Add test data
	delegations := []domain.Delegation{
		{
			ID:            uuid.New().String(),
			Timestamp:     time.Date(2023, 6, 15, 0, 0, 0, 0, time.UTC),
			Amount:        "1000000",
			Delegator:     "tz1abc123",
			Level:         "2338084",
			BlockHash:     "BlockHash1",
			OperationHash: uuid.New().String(),
		},
		{
			ID:            uuid.New().String(),
			Timestamp:     time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC),
			Amount:        "2000000",
			Delegator:     "tz1def456",
			Level:         "2338085",
			BlockHash:     "BlockHash2",
			OperationHash: uuid.New().String(),
		},
	}

	err := suite.repo.SaveBatch(delegations)
	require.NoError(t, err)

	// Test GetDelegations without year filter
	allDelegations, err := suite.service.GetDelegations(nil)
	require.NoError(t, err)
	assert.Len(t, allDelegations, 2)

	// Test GetDelegations with year filter
	year := 2023
	yearDelegations, err := suite.service.GetDelegations(&year)
	require.NoError(t, err)
	assert.Len(t, yearDelegations, 1)
	assert.Equal(t, 2023, yearDelegations[0].Timestamp.Year())
}

// Mock TzKT client for integration tests
type MockTzktClient struct{}

func (m *MockTzktClient) GetDelegations(ctx context.Context, params tzkt.QueryParams) ([]tzkt.DelegationResponse, error) {
	return []tzkt.DelegationResponse{}, nil
}

func (m *MockTzktClient) GetDelegationsSince(ctx context.Context, timestamp time.Time, limit int) ([]tzkt.DelegationResponse, error) {
	return []tzkt.DelegationResponse{}, nil
}

func (m *MockTzktClient) GetDelegationsFromLevel(ctx context.Context, level int64, limit int) ([]tzkt.DelegationResponse, error) {
	return []tzkt.DelegationResponse{}, nil
}

func (m *MockTzktClient) GetHistoricalDelegations(ctx context.Context, startDate time.Time, batchSize int) (<-chan []tzkt.DelegationResponse, <-chan error) {
	dataChan := make(chan []tzkt.DelegationResponse)
	errChan := make(chan error)
	close(dataChan)
	close(errChan)
	return dataChan, errChan
}