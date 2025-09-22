package domain

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDelegation_Creation(t *testing.T) {
	now := time.Now()
	delegation := Delegation{
		ID:            uuid.New().String(),
		Timestamp:     now,
		Amount:        "1000000",
		Delegator:     "tz1abc123",
		Level:         "2338084",
		BlockHash:     "BlockHash1",
		OperationHash: "OpHash1",
		CreatedAt:     now,
	}

	assert.NotEmpty(t, delegation.ID)
	assert.Equal(t, "1000000", delegation.Amount)
	assert.Equal(t, "tz1abc123", delegation.Delegator)
	assert.Equal(t, "2338084", delegation.Level)
	assert.Equal(t, "BlockHash1", delegation.BlockHash)
	assert.Equal(t, "OpHash1", delegation.OperationHash)
	assert.True(t, delegation.Timestamp.Equal(now))
	assert.True(t, delegation.CreatedAt.Equal(now))
}

func TestDelegation_JSONMarshaling(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	delegation := Delegation{
		ID:            uuid.New().String(),
		Timestamp:     now,
		Amount:        "1000000",
		Delegator:     "tz1abc123",
		Level:         "2338084",
		BlockHash:     "BlockHash1",
		OperationHash: "OpHash1",
		CreatedAt:     now,
	}

	data, err := json.Marshal(delegation)
	require.NoError(t, err)

	var unmarshaled map[string]interface{}
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	// Check that fields with json:"-" tag are not included
	assert.NotContains(t, unmarshaled, "id")
	assert.NotContains(t, unmarshaled, "created_at")

	// Check that visible fields are included
	assert.Contains(t, unmarshaled, "timestamp")
	assert.Contains(t, unmarshaled, "amount")
	assert.Contains(t, unmarshaled, "delegator")
	assert.Contains(t, unmarshaled, "level")
	assert.Contains(t, unmarshaled, "operation_hash")

	// Verify values
	assert.Equal(t, "1000000", unmarshaled["amount"])
	assert.Equal(t, "tz1abc123", unmarshaled["delegator"])
	assert.Equal(t, "2338084", unmarshaled["level"])
	assert.Equal(t, "OpHash1", unmarshaled["operation_hash"])
}

func TestDelegationResponse_JSONMarshaling(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	delegations := []Delegation{
		{
			ID:            uuid.New().String(),
			Timestamp:     now,
			Amount:        "1000000",
			Delegator:     "tz1abc123",
			Level:         "2338084",
			BlockHash:     "BlockHash1",
			OperationHash: "OpHash1",
			CreatedAt:     now,
		},
		{
			ID:            uuid.New().String(),
			Timestamp:     now.Add(time.Hour),
			Amount:        "2000000",
			Delegator:     "tz1def456",
			Level:         "2338085",
			BlockHash:     "BlockHash2",
			OperationHash: "OpHash2",
			CreatedAt:     now,
		},
	}

	response := DelegationResponse{
		Data: delegations,
	}

	data, err := json.Marshal(response)
	require.NoError(t, err)

	var unmarshaled DelegationResponse
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Len(t, unmarshaled.Data, 2)
	assert.Equal(t, "1000000", unmarshaled.Data[0].Amount)
	assert.Equal(t, "tz1abc123", unmarshaled.Data[0].Delegator)
	assert.Equal(t, "2000000", unmarshaled.Data[1].Amount)
	assert.Equal(t, "tz1def456", unmarshaled.Data[1].Delegator)
}

func TestDelegation_ValidateDelegatorAddress(t *testing.T) {
	testCases := []struct {
		name      string
		delegator string
		valid     bool
	}{
		{"Valid tz1 address", "tz1a1SAaXRt9yoGMx29rh9FsBF4UzmvojdTL", true},
		{"Valid tz2 address", "tz2BFTyPeYRzxd5aiBchbXN3WCZhh7BqbMBq", true},
		{"Valid tz3 address", "tz3RDC3Jdn4j15J7bBHZd29EUee9gVB1CxD9", true},
		{"Valid KT1 address", "KT1JejNYjmQYh8yw95u5kfQDRuxJcaUPjUnf", true},
		{"Empty address", "", false},
		{"Invalid prefix", "abc123", false},
		{"Too short", "tz1", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			delegation := Delegation{
				Delegator: tc.delegator,
			}
			
			isValid := isValidTezosAddress(delegation.Delegator)
			assert.Equal(t, tc.valid, isValid)
		})
	}
}

func isValidTezosAddress(address string) bool {
	if len(address) < 3 {
		return false
	}
	
	prefix := address[:3]
	validPrefixes := []string{"tz1", "tz2", "tz3", "KT1"}
	
	for _, valid := range validPrefixes {
		if prefix == valid {
			return len(address) >= 30 // Basic length check
		}
	}
	
	return false
}

func TestDelegation_CompareAmounts(t *testing.T) {
	d1 := Delegation{Amount: "1000000"}
	d2 := Delegation{Amount: "2000000"}
	d3 := Delegation{Amount: "1000000"}

	// Simple string comparison for amounts stored as strings
	assert.True(t, d1.Amount < d2.Amount)
	assert.True(t, d1.Amount == d3.Amount)
	assert.False(t, d2.Amount < d1.Amount)
}

func TestDelegation_EmptyValues(t *testing.T) {
	delegation := Delegation{}

	assert.Empty(t, delegation.ID)
	assert.Empty(t, delegation.Amount)
	assert.Empty(t, delegation.Delegator)
	assert.Empty(t, delegation.Level)
	assert.Empty(t, delegation.BlockHash)
	assert.Empty(t, delegation.OperationHash)
	assert.True(t, delegation.Timestamp.IsZero())
	assert.True(t, delegation.CreatedAt.IsZero())
}

func TestDelegationRepository_Interface(t *testing.T) {
	// This test ensures the interface methods are properly defined
	var _ DelegationRepository = (*mockRepo)(nil)
}

func TestDelegationService_Interface(t *testing.T) {
	// This test ensures the interface methods are properly defined
	var _ DelegationService = (*mockService)(nil)
}

// Mock implementations for interface testing
type mockRepo struct{}

func (m *mockRepo) Save(delegation *Delegation) error                   { return nil }
func (m *mockRepo) SaveBatch(delegations []Delegation) error           { return nil }
func (m *mockRepo) FindAll(year *int) ([]Delegation, error)            { return nil, nil }
func (m *mockRepo) GetLastIndexedLevel() (int64, error)                { return 0, nil }
func (m *mockRepo) Exists(delegator string, level string) (bool, error) { return false, nil }

type mockService struct{}

func (m *mockService) GetDelegations(year *int) ([]Delegation, error) { return nil, nil }
func (m *mockService) IndexDelegations(fromLevel int64) error         { return nil }
func (m *mockService) StartPolling() error                            { return nil }
func (m *mockService) StopPolling()                                   {}