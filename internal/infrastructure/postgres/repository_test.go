package postgres

import (
	"testing"
)

// Repository tests are better suited as integration tests
// See internal/integration_test.go for comprehensive database testing

func TestRepository_Integration(t *testing.T) {
	t.Skip("Repository tests are implemented as integration tests - run with 'make test-integration'")
}

func TestRepository_Save(t *testing.T) {
	t.Skip("See integration tests for database testing")
}

func TestRepository_SaveBatch(t *testing.T) {
	t.Skip("See integration tests for database testing")
}

func TestRepository_FindAll(t *testing.T) {
	t.Skip("See integration tests for database testing")
}

func TestRepository_GetLastIndexedLevel(t *testing.T) {
	t.Skip("See integration tests for database testing")
}

func TestRepository_Exists(t *testing.T) {
	t.Skip("See integration tests for database testing")
}

func TestRepository_GetDelegationsByTimeRange(t *testing.T) {
	t.Skip("See integration tests for database testing")
}

func TestRepository_GetStats(t *testing.T) {
	t.Skip("See integration tests for database testing")
}

func TestRepository_UpdateIndexingMetadata(t *testing.T) {
	t.Skip("See integration tests for database testing")
}

func TestRepository_GetIndexingMetadata(t *testing.T) {
	t.Skip("See integration tests for database testing")
}