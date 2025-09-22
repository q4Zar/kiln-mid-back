package testutil

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/q4ZAr/kiln-mid-back/tezos-delegation-service/internal/domain"
	"github.com/q4ZAr/kiln-mid-back/tezos-delegation-service/internal/infrastructure/tzkt"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// CreateTestDelegation creates a test delegation with default values
func CreateTestDelegation(t *testing.T) domain.Delegation {
	t.Helper()
	return domain.Delegation{
		ID:            uuid.New().String(),
		Timestamp:     time.Now(),
		Amount:        "1000000",
		Delegator:     "tz1a1SAaXRt9yoGMx29rh9FsBF4UzmvojdTL",
		Level:         "2338084",
		BlockHash:     "BLockHash_" + uuid.New().String()[:8],
		OperationHash: "OpHash_" + uuid.New().String(),
		CreatedAt:     time.Now(),
	}
}

// CreateTestDelegations creates multiple test delegations
func CreateTestDelegations(t *testing.T, count int) []domain.Delegation {
	t.Helper()
	delegations := make([]domain.Delegation, count)
	for i := 0; i < count; i++ {
		delegations[i] = CreateTestDelegation(t)
		delegations[i].Level = fmt.Sprintf("%d", 2338084+i)
		delegations[i].Timestamp = time.Now().Add(time.Duration(i) * time.Hour)
	}
	return delegations
}

// CreateTestTzktDelegation creates a test TzKT delegation response
func CreateTestTzktDelegation() tzkt.DelegationResponse {
	return tzkt.DelegationResponse{
		ID:        1,
		Level:     2338084,
		Timestamp: time.Now(),
		Block:     "BLockHash_test",
		Sender:    tzkt.Sender{Address: "tz1a1SAaXRt9yoGMx29rh9FsBF4UzmvojdTL"},
		Amount:    1000000,
	}
}

// AssertDelegationsEqual asserts that two delegations are equal
func AssertDelegationsEqual(t *testing.T, expected, actual domain.Delegation) {
	t.Helper()
	require.Equal(t, expected.Amount, actual.Amount)
	require.Equal(t, expected.Delegator, actual.Delegator)
	require.Equal(t, expected.Level, actual.Level)
	require.Equal(t, expected.BlockHash, actual.BlockHash)
	require.Equal(t, expected.OperationHash, actual.OperationHash)
	require.True(t, expected.Timestamp.Equal(actual.Timestamp))
}

// ValidateTezosAddress validates if a string is a valid Tezos address
func ValidateTezosAddress(address string) bool {
	if len(address) < 30 {
		return false
	}
	
	prefix := address[:3]
	validPrefixes := []string{"tz1", "tz2", "tz3", "KT1"}
	
	for _, valid := range validPrefixes {
		if prefix == valid {
			return true
		}
	}
	
	return false
}

// GenerateValidTezosAddresses generates a list of valid Tezos addresses for testing
func GenerateValidTezosAddresses(count int) []string {
	addresses := []string{
		"tz1a1SAaXRt9yoGMx29rh9FsBF4UzmvojdTL",
		"tz1VSUr8wwNhLAzempoch5d6hLRiTh8Cjcjb",
		"tz1aSkwEot3L2kmUvcoxzjMomb9mvBNuzFK6",
		"tz2BFTyPeYRzxd5aiBchbXN3WCZhh7BqbMBq",
		"tz3RDC3Jdn4j15J7bBHZd29EUee9gVB1CxD9",
		"KT1JejNYjmQYh8yw95u5kfQDRuxJcaUPjUnf",
		"KT1HBy1L43tiLe5MVJZ5RoxGy53Kx8kMgyoU",
	}
	
	if count > len(addresses) {
		// Generate more if needed
		for i := len(addresses); i < count; i++ {
			addresses = append(addresses, fmt.Sprintf("tz1test%s", uuid.New().String()[:27]))
		}
	}
	
	return addresses[:count]
}

// TimeRange represents a time range for testing
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// CreateTimeRange creates a time range for testing
func CreateTimeRange(daysAgo int, duration time.Duration) TimeRange {
	end := time.Now()
	start := end.Add(-time.Duration(daysAgo) * 24 * time.Hour)
	return TimeRange{
		Start: start,
		End:   start.Add(duration),
	}
}

// MockTimer provides a controllable time source for testing
type MockTimer struct {
	CurrentTime time.Time
}

func NewMockTimer() *MockTimer {
	return &MockTimer{
		CurrentTime: time.Now(),
	}
}

func (m *MockTimer) Now() time.Time {
	return m.CurrentTime
}

func (m *MockTimer) Advance(d time.Duration) {
	m.CurrentTime = m.CurrentTime.Add(d)
}

// WaitForCondition waits for a condition to be true with timeout
func WaitForCondition(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("Condition not met within timeout of %v", timeout)
}

// TestContext creates a test context with timeout
func TestContext(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.WithTimeout(context.Background(), 30*time.Second)
}

// MockDelegationRepository is a mock implementation of DelegationRepository
type MockDelegationRepository struct {
	mock.Mock
}

func (m *MockDelegationRepository) Save(delegation *domain.Delegation) error {
	args := m.Called(delegation)
	return args.Error(0)
}

func (m *MockDelegationRepository) SaveBatch(delegations []domain.Delegation) error {
	args := m.Called(delegations)
	return args.Error(0)
}

func (m *MockDelegationRepository) FindAll(year *int) ([]domain.Delegation, error) {
	args := m.Called(year)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.Delegation), args.Error(1)
}

func (m *MockDelegationRepository) GetLastIndexedLevel() (int64, error) {
	args := m.Called()
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockDelegationRepository) Exists(delegator string, level string) (bool, error) {
	args := m.Called(delegator, level)
	return args.Get(0).(bool), args.Error(1)
}

// MockDelegationService is a mock implementation of DelegationService
type MockDelegationService struct {
	mock.Mock
}

func (m *MockDelegationService) GetDelegations(year *int) ([]domain.Delegation, error) {
	args := m.Called(year)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.Delegation), args.Error(1)
}

func (m *MockDelegationService) IndexDelegations(fromLevel int64) error {
	args := m.Called(fromLevel)
	return args.Error(0)
}

func (m *MockDelegationService) StartPolling() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockDelegationService) StopPolling() {
	m.Called()
}

func (m *MockDelegationService) GetStats() (map[string]interface{}, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]interface{}), args.Error(1)
}