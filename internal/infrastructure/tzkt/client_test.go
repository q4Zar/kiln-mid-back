package tzkt

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/q4ZAr/kiln-mid-back/tezos-delegation-service/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_GetDelegations(t *testing.T) {
	mockResponse := []DelegationResponse{
		{
			ID:        1,
			Level:     1000,
			Timestamp: time.Now().Add(-24 * time.Hour),
			Block:     "BLockHash1",
			Sender:    Sender{Address: "tz1abc123"},
			Amount:    1000000,
		},
		{
			ID:        2,
			Level:     1001,
			Timestamp: time.Now().Add(-23 * time.Hour),
			Block:     "BlockHash2",
			Sender:    Sender{Address: "tz1def456"},
			Amount:    2000000,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/operations/delegations", r.URL.Path)
		assert.Equal(t, "applied", r.URL.Query().Get("status"))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	log, _ := logger.New("debug", "test")
	client := NewClient(server.URL, 5*time.Second, 3, time.Second, log)

	params := QueryParams{
		Limit: 10,
	}

	delegations, err := client.GetDelegations(context.Background(), params)
	require.NoError(t, err)
	assert.Len(t, delegations, 2)
	assert.Equal(t, "tz1abc123", delegations[0].Sender.Address)
	assert.Equal(t, int64(1000000), delegations[0].Amount)
}

func TestClient_GetDelegationsSince(t *testing.T) {
	mockResponse := []DelegationResponse{
		{
			ID:        1,
			Level:     1000,
			Timestamp: time.Now().Add(-12 * time.Hour),
			Block:     "BlockHash1",
			Sender:    Sender{Address: "tz1abc123"},
			Amount:    1000000,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/operations/delegations", r.URL.Path)
		assert.NotEmpty(t, r.URL.Query().Get("timestamp.ge"))
		assert.Equal(t, "id", r.URL.Query().Get("sort.asc"))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	log, _ := logger.New("debug", "test")
	client := NewClient(server.URL, 5*time.Second, 3, time.Second, log)

	since := time.Now().Add(-24 * time.Hour)
	delegations, err := client.GetDelegationsSince(context.Background(), since, 100)

	require.NoError(t, err)
	assert.Len(t, delegations, 1)
	assert.Equal(t, "tz1abc123", delegations[0].Sender.Address)
}

func TestClient_GetDelegationsFromLevel(t *testing.T) {
	mockResponse := []DelegationResponse{
		{
			ID:        1,
			Level:     2000,
			Timestamp: time.Now(),
			Block:     "BlockHash1",
			Sender:    Sender{Address: "tz1xyz789"},
			Amount:    3000000,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/operations/delegations", r.URL.Path)
		assert.Equal(t, "1999", r.URL.Query().Get("level.ge"))
		assert.Equal(t, "id", r.URL.Query().Get("sort.asc"))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	log, _ := logger.New("debug", "test")
	client := NewClient(server.URL, 5*time.Second, 3, time.Second, log)

	delegations, err := client.GetDelegationsFromLevel(context.Background(), 1999, 100)

	require.NoError(t, err)
	assert.Len(t, delegations, 1)
	assert.Equal(t, int64(2000), delegations[0].Level)
	assert.Equal(t, "tz1xyz789", delegations[0].Sender.Address)
}

func TestClient_RetryOnError(t *testing.T) {
	attemptCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		if attemptCount < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		mockResponse := []DelegationResponse{
			{
				ID:        1,
				Level:     1000,
				Timestamp: time.Now(),
				Block:     "BlockHash1",
				Sender:    Sender{Address: "tz1success"},
				Amount:    1000000,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	log, _ := logger.New("debug", "test")
	client := NewClient(server.URL, 5*time.Second, 5, 100*time.Millisecond, log)

	params := QueryParams{Limit: 10}
	delegations, err := client.GetDelegations(context.Background(), params)

	require.NoError(t, err)
	assert.Equal(t, 3, attemptCount)
	assert.Len(t, delegations, 1)
	assert.Equal(t, "tz1success", delegations[0].Sender.Address)
}

func TestClient_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]DelegationResponse{})
	}))
	defer server.Close()

	log, _ := logger.New("debug", "test")
	client := NewClient(server.URL, 5*time.Second, 3, time.Second, log)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := client.GetDelegations(ctx, QueryParams{Limit: 10})
	assert.Error(t, err)
}

func TestClient_BuildQueryParams(t *testing.T) {
	client := &Client{}

	level := int64(1000)
	timestamp := time.Now()

	params := QueryParams{
		Limit:  100,
		Offset: 50,
		Level: &LevelFilter{
			Gte: &level,
		},
		Timestamp: &TimestampFilter{
			Gte: &timestamp,
		},
		Sort:   []string{"timestamp.asc", "level.desc"},
		Select: []string{"id", "amount"},
	}

	queryParams := client.buildQueryParams(params)

	assert.Equal(t, "100", queryParams["limit"])
	assert.Equal(t, "50", queryParams["offset"])
	assert.Equal(t, "1000", queryParams["level.ge"])
	assert.NotEmpty(t, queryParams["timestamp.ge"])
	assert.Equal(t, "timestamp", queryParams["sort.asc"])
	assert.Equal(t, "level", queryParams["sort.desc"])
	assert.Contains(t, queryParams["select"], "id")
	assert.Equal(t, "applied", queryParams["status"])
}
