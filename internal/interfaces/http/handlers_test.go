package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/q4ZAr/kiln-mid-back/tezos-delegation-service/internal/domain"
	"github.com/q4ZAr/kiln-mid-back/tezos-delegation-service/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockService struct {
	mock.Mock
}

func (m *MockService) GetDelegations(year *int) ([]domain.Delegation, error) {
	args := m.Called(year)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.Delegation), args.Error(1)
}

func (m *MockService) IndexDelegations(fromLevel int64) error {
	args := m.Called(fromLevel)
	return args.Error(0)
}

func (m *MockService) StartPolling() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockService) StopPolling() {
	m.Called()
}

func (m *MockService) GetStats() (map[string]interface{}, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]interface{}), args.Error(1)
}

func setupRouter(service domain.DelegationService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	log, _ := logger.New("debug", "test")
	handler := NewHandler(service, log)

	router := gin.New()
	router.GET("/xtz/delegations", handler.GetDelegations)
	router.GET("/health", handler.GetHealth)
	router.GET("/ready", handler.GetReadiness)
	router.GET("/stats", handler.GetStats)

	return router
}

func TestHandler_GetDelegations(t *testing.T) {
	mockService := new(MockService)
	router := setupRouter(mockService)

	expectedDelegations := []domain.Delegation{
		{
			ID:        uuid.New().String(),
			Timestamp: time.Now().Add(-24 * time.Hour),
			Amount:    "125896",
			Delegator: "tz1a1SAaXRt9yoGMx29rh9FsBF4UzmvojdTL",
			Level:     "2338084",
		},
		{
			ID:        uuid.New().String(),
			Timestamp: time.Now().Add(-12 * time.Hour),
			Amount:    "9856354",
			Delegator: "KT1JejNYjmQYh8yw95u5kfQDRuxJcaUPjUnf",
			Level:     "1461334",
		},
	}

	mockService.On("GetDelegations", (*int)(nil)).Return(expectedDelegations, nil)

	req := httptest.NewRequest(http.MethodGet, "/xtz/delegations", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response domain.DelegationResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Len(t, response.Data, 2)
	assert.Equal(t, "125896", response.Data[0].Amount)
	assert.Equal(t, "tz1a1SAaXRt9yoGMx29rh9FsBF4UzmvojdTL", response.Data[0].Delegator)
	assert.Equal(t, "2338084", response.Data[0].Level)

	mockService.AssertExpectations(t)
}

func TestHandler_GetDelegationsWithYear(t *testing.T) {
	mockService := new(MockService)
	router := setupRouter(mockService)

	year := 2022
	expectedDelegations := []domain.Delegation{
		{
			ID:        uuid.New().String(),
			Timestamp: time.Date(2022, 5, 5, 6, 29, 14, 0, time.UTC),
			Amount:    "125896",
			Delegator: "tz1a1SAaXRt9yoGMx29rh9FsBF4UzmvojdTL",
			Level:     "2338084",
		},
	}

	mockService.On("GetDelegations", &year).Return(expectedDelegations, nil)

	req := httptest.NewRequest(http.MethodGet, "/xtz/delegations?year=2022", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response domain.DelegationResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Len(t, response.Data, 1)
	assert.Equal(t, 2022, response.Data[0].Timestamp.Year())

	mockService.AssertExpectations(t)
}

func TestHandler_GetDelegationsInvalidYear(t *testing.T) {
	mockService := new(MockService)
	router := setupRouter(mockService)

	testCases := []struct {
		name     string
		year     string
		expected string
	}{
		{"Invalid format", "abc", "Invalid year parameter"},
		{"Year too early", "2017", "Year must be between 2018 and 2100"},
		{"Year too late", "2101", "Year must be between 2018 and 2100"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/xtz/delegations?year=%s", tc.year), nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)

			var response map[string]string
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Contains(t, response["error"], tc.expected)
		})
	}
}

func TestHandler_GetDelegationsEmptyResult(t *testing.T) {
	mockService := new(MockService)
	router := setupRouter(mockService)

	mockService.On("GetDelegations", (*int)(nil)).Return([]domain.Delegation{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/xtz/delegations", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response domain.DelegationResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.NotNil(t, response.Data)
	assert.Len(t, response.Data, 0)

	mockService.AssertExpectations(t)
}

func TestHandler_GetHealth(t *testing.T) {
	mockService := new(MockService)
	router := setupRouter(mockService)

	delegations := []domain.Delegation{
		{ID: "1"}, {ID: "2"}, {ID: "3"},
	}

	mockService.On("GetDelegations", (*int)(nil)).Return(delegations, nil)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "healthy", response["status"])
	assert.Equal(t, float64(3), response["total_delegations"])

	mockService.AssertExpectations(t)
}

func TestHandler_GetHealthUnhealthy(t *testing.T) {
	mockService := new(MockService)
	router := setupRouter(mockService)

	mockService.On("GetDelegations", (*int)(nil)).Return(nil, fmt.Errorf("database connection failed"))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "unhealthy", response["status"])
	assert.Contains(t, response["error"], "database connection failed")

	mockService.AssertExpectations(t)
}

func TestHandler_GetReadiness(t *testing.T) {
	mockService := new(MockService)
	router := setupRouter(mockService)

	mockService.On("GetDelegations", (*int)(nil)).Return([]domain.Delegation{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "ready", response["status"])

	mockService.AssertExpectations(t)
}

func TestHandler_GetStats(t *testing.T) {
	mockService := new(MockService)
	router := setupRouter(mockService)

	stats := map[string]interface{}{
		"total_delegations":  100,
		"unique_delegators":  50,
		"total_amount":       "1000000000",
		"last_indexed_level": int64(2500000),
	}

	mockService.On("GetStats").Return(stats, nil)

	req := httptest.NewRequest(http.MethodGet, "/stats", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, float64(100), response["total_delegations"])
	assert.Equal(t, float64(50), response["unique_delegators"])
	assert.Equal(t, "1000000000", response["total_amount"])
	assert.Equal(t, float64(2500000), response["last_indexed_level"])

	mockService.AssertExpectations(t)
}
