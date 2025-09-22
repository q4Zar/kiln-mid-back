package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/q4ZAr/kiln-mid-back/tezos-delegation-service/internal/domain"
	"github.com/q4ZAr/kiln-mid-back/tezos-delegation-service/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Complex test scenarios for HTTP handlers

func TestHandler_GetDelegations_TableDriven(t *testing.T) {
	type testCase struct {
		name               string
		queryParams        string
		mockSetup          func(*MockService)
		expectedStatus     int
		validateResponse   func(*testing.T, *httptest.ResponseRecorder)
	}
	
	tests := []testCase{
		{
			name:        "successful request with pagination hint",
			queryParams: "",
			mockSetup: func(m *MockService) {
				delegations := make([]domain.Delegation, 100)
				for i := range delegations {
					delegations[i] = domain.Delegation{
						ID:        uuid.New().String(),
						Timestamp: time.Now().Add(time.Duration(-i) * time.Hour),
						Amount:    fmt.Sprintf("%d", (i+1)*1000000),
						Delegator: fmt.Sprintf("tz1test%d", i),
						Level:     fmt.Sprintf("%d", 2000000+i),
					}
				}
				m.On("GetDelegations", (*int)(nil)).Return(delegations, nil)
			},
			expectedStatus: http.StatusOK,
			validateResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response domain.DelegationResponse
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Len(t, response.Data, 100)
			},
		},
		{
			name:        "year filter edge case - minimum year",
			queryParams: "?year=2018",
			mockSetup: func(m *MockService) {
				year := 2018
				m.On("GetDelegations", &year).Return([]domain.Delegation{}, nil)
			},
			expectedStatus: http.StatusOK,
			validateResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response domain.DelegationResponse
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Empty(t, response.Data)
			},
		},
		{
			name:        "year filter edge case - maximum year",
			queryParams: "?year=2100",
			mockSetup: func(m *MockService) {
				year := 2100
				m.On("GetDelegations", &year).Return([]domain.Delegation{}, nil)
			},
			expectedStatus: http.StatusOK,
			validateResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response domain.DelegationResponse
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.NotNil(t, response.Data)
			},
		},
		{
			name:        "year filter below minimum",
			queryParams: "?year=2017",
			mockSetup:   func(m *MockService) {},
			expectedStatus: http.StatusBadRequest,
			validateResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]string
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response["error"], "Year must be between 2018 and 2100")
			},
		},
		{
			name:        "year filter above maximum",
			queryParams: "?year=2101",
			mockSetup:   func(m *MockService) {},
			expectedStatus: http.StatusBadRequest,
			validateResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]string
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response["error"], "Year must be between 2018 and 2100")
			},
		},
		{
			name:        "multiple query parameters",
			queryParams: "?year=2023&limit=50&offset=100", // Extra params should be ignored
			mockSetup: func(m *MockService) {
				year := 2023
				m.On("GetDelegations", &year).Return([]domain.Delegation{}, nil)
			},
			expectedStatus: http.StatusOK,
			validateResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response domain.DelegationResponse
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.NotNil(t, response.Data)
			},
		},
		{
			name:        "special characters in query",
			queryParams: "?year=2023%20%27DROP%20TABLE",
			mockSetup:   func(m *MockService) {},
			expectedStatus: http.StatusBadRequest,
			validateResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]string
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response["error"], "Invalid year parameter")
			},
		},
		{
			name:        "service error handling",
			queryParams: "",
			mockSetup: func(m *MockService) {
				m.On("GetDelegations", (*int)(nil)).Return(nil, fmt.Errorf("database connection lost"))
			},
			expectedStatus: http.StatusInternalServerError,
			validateResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]string
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, "Failed to fetch delegations", response["error"])
			},
		},
		{
			name:        "empty result with year",
			queryParams: "?year=2099",
			mockSetup: func(m *MockService) {
				year := 2099
				m.On("GetDelegations", &year).Return([]domain.Delegation{}, nil)
			},
			expectedStatus: http.StatusOK,
			validateResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response domain.DelegationResponse
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.NotNil(t, response.Data)
				assert.Empty(t, response.Data)
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(MockService)
			tt.mockSetup(mockService)
			
			router := setupRouter(mockService)
			
			req := httptest.NewRequest(http.MethodGet, "/xtz/delegations"+tt.queryParams, nil)
			rec := httptest.NewRecorder()
			
			router.ServeHTTP(rec, req)
			
			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.validateResponse != nil {
				tt.validateResponse(t, rec)
			}
			
			mockService.AssertExpectations(t)
		})
	}
}

func TestHandler_ConcurrentRequests(t *testing.T) {
	mockService := new(MockService)
	
	// Setup mock to handle concurrent calls
	delegations := []domain.Delegation{
		{
			ID:        uuid.New().String(),
			Timestamp: time.Now(),
			Amount:    "1000000",
			Delegator: "tz1abc123",
			Level:     "1000",
		},
	}
	
	mockService.On("GetDelegations", (*int)(nil)).Return(delegations, nil).Maybe()
	mockService.On("GetStats").Return(map[string]interface{}{
		"total_delegations": 100,
		"unique_delegators": 50,
		"total_amount":      "1000000000",
	}, nil).Maybe()
	
	router := setupRouter(mockService)
	
	var wg sync.WaitGroup
	requestCount := 100
	results := make([]*httptest.ResponseRecorder, requestCount)
	
	// Concurrent requests to different endpoints
	for i := 0; i < requestCount; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			
			var req *http.Request
			switch index % 4 {
			case 0:
				req = httptest.NewRequest(http.MethodGet, "/xtz/delegations", nil)
			case 1:
				req = httptest.NewRequest(http.MethodGet, "/health", nil)
			case 2:
				req = httptest.NewRequest(http.MethodGet, "/ready", nil)
			case 3:
				req = httptest.NewRequest(http.MethodGet, "/stats", nil)
			}
			
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			results[index] = rec
		}(i)
	}
	
	wg.Wait()
	
	// Verify all requests succeeded
	for i, rec := range results {
		assert.True(t, rec.Code == http.StatusOK || rec.Code == http.StatusServiceUnavailable, 
			"Request %d returned unexpected status: %d", i, rec.Code)
	}
	
	mockService.AssertExpectations(t)
}

func TestHandler_HealthCheck_Detailed(t *testing.T) {
	tests := []struct {
		name           string
		setupMock      func(*MockService)
		expectedStatus int
		validateBody   func(*testing.T, map[string]interface{})
	}{
		{
			name: "healthy with data",
			setupMock: func(m *MockService) {
				delegations := make([]domain.Delegation, 1000)
				for i := range delegations {
					delegations[i] = domain.Delegation{
						ID: uuid.New().String(),
					}
				}
				m.On("GetDelegations", (*int)(nil)).Return(delegations, nil)
			},
			expectedStatus: http.StatusOK,
			validateBody: func(t *testing.T, body map[string]interface{}) {
				assert.Equal(t, "healthy", body["status"])
				assert.Equal(t, float64(1000), body["total_delegations"])
			},
		},
		{
			name: "healthy but empty",
			setupMock: func(m *MockService) {
				m.On("GetDelegations", (*int)(nil)).Return([]domain.Delegation{}, nil)
			},
			expectedStatus: http.StatusOK,
			validateBody: func(t *testing.T, body map[string]interface{}) {
				assert.Equal(t, "healthy", body["status"])
				assert.Equal(t, float64(0), body["total_delegations"])
			},
		},
		{
			name: "unhealthy - database error",
			setupMock: func(m *MockService) {
				m.On("GetDelegations", (*int)(nil)).Return(nil, fmt.Errorf("connection refused"))
			},
			expectedStatus: http.StatusServiceUnavailable,
			validateBody: func(t *testing.T, body map[string]interface{}) {
				assert.Equal(t, "unhealthy", body["status"])
				assert.Contains(t, body["error"], "connection refused")
			},
		},
		{
			name: "unhealthy - timeout",
			setupMock: func(m *MockService) {
				m.On("GetDelegations", (*int)(nil)).Return(nil, fmt.Errorf("context deadline exceeded"))
			},
			expectedStatus: http.StatusServiceUnavailable,
			validateBody: func(t *testing.T, body map[string]interface{}) {
				assert.Equal(t, "unhealthy", body["status"])
				assert.Contains(t, body["error"], "deadline exceeded")
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(MockService)
			tt.setupMock(mockService)
			
			router := setupRouter(mockService)
			
			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			rec := httptest.NewRecorder()
			
			router.ServeHTTP(rec, req)
			
			assert.Equal(t, tt.expectedStatus, rec.Code)
			
			var response map[string]interface{}
			err := json.Unmarshal(rec.Body.Bytes(), &response)
			require.NoError(t, err)
			
			if tt.validateBody != nil {
				tt.validateBody(t, response)
			}
			
			mockService.AssertExpectations(t)
		})
	}
}

func TestHandler_Stats_ComplexScenarios(t *testing.T) {
	tests := []struct {
		name           string
		setupMock      func(*MockService)
		expectedStatus int
		validateStats  func(*testing.T, map[string]interface{})
	}{
		{
			name: "complete statistics",
			setupMock: func(m *MockService) {
				stats := map[string]interface{}{
					"total_delegations":  int64(1000000),
					"unique_delegators":  int64(50000),
					"total_amount":       "999999999999999999",
					"last_indexed_level": int64(3000000),
					"latest_delegation":  time.Now(),
					"oldest_delegation":  time.Now().Add(-365 * 24 * time.Hour),
					"average_amount":     "1000000",
					"indexing_rate":      float64(100.5),
				}
				m.On("GetStats").Return(stats, nil)
			},
			expectedStatus: http.StatusOK,
			validateStats: func(t *testing.T, stats map[string]interface{}) {
				assert.Equal(t, float64(1000000), stats["total_delegations"])
				assert.Equal(t, float64(50000), stats["unique_delegators"])
				assert.Equal(t, "999999999999999999", stats["total_amount"])
				assert.Equal(t, float64(3000000), stats["last_indexed_level"])
			},
		},
		{
			name: "partial statistics",
			setupMock: func(m *MockService) {
				stats := map[string]interface{}{
					"total_delegations": int64(100),
					"unique_delegators": int64(10),
					"total_amount":      "0",
				}
				m.On("GetStats").Return(stats, nil)
			},
			expectedStatus: http.StatusOK,
			validateStats: func(t *testing.T, stats map[string]interface{}) {
				assert.Equal(t, float64(100), stats["total_delegations"])
				assert.Equal(t, float64(10), stats["unique_delegators"])
				assert.Equal(t, "0", stats["total_amount"])
			},
		},
		{
			name: "error fetching stats",
			setupMock: func(m *MockService) {
				m.On("GetStats").Return(nil, fmt.Errorf("stats calculation failed"))
			},
			expectedStatus: http.StatusInternalServerError,
			validateStats: func(t *testing.T, response map[string]interface{}) {
				assert.Equal(t, "Failed to fetch statistics", response["error"])
			},
		},
		{
			name: "empty stats",
			setupMock: func(m *MockService) {
				m.On("GetStats").Return(map[string]interface{}{}, nil)
			},
			expectedStatus: http.StatusOK,
			validateStats: func(t *testing.T, stats map[string]interface{}) {
				assert.Empty(t, stats)
			},
		},
		{
			name: "stats with nil values",
			setupMock: func(m *MockService) {
				stats := map[string]interface{}{
					"total_delegations": int64(100),
					"unique_delegators": nil,
					"total_amount":      "1000000",
					"extra_field":       nil,
				}
				m.On("GetStats").Return(stats, nil)
			},
			expectedStatus: http.StatusOK,
			validateStats: func(t *testing.T, stats map[string]interface{}) {
				assert.Equal(t, float64(100), stats["total_delegations"])
				assert.Nil(t, stats["unique_delegators"])
				assert.Equal(t, "1000000", stats["total_amount"])
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(MockService)
			tt.setupMock(mockService)
			
			router := setupRouter(mockService)
			
			req := httptest.NewRequest(http.MethodGet, "/stats", nil)
			rec := httptest.NewRecorder()
			
			router.ServeHTTP(rec, req)
			
			assert.Equal(t, tt.expectedStatus, rec.Code)
			
			var response map[string]interface{}
			err := json.Unmarshal(rec.Body.Bytes(), &response)
			require.NoError(t, err)
			
			if tt.validateStats != nil {
				tt.validateStats(t, response)
			}
			
			mockService.AssertExpectations(t)
		})
	}
}

func TestHandler_RequestHeaders(t *testing.T) {
	mockService := new(MockService)
	mockService.On("GetDelegations", (*int)(nil)).Return([]domain.Delegation{}, nil).Maybe()
	
	router := setupRouter(mockService)
	
	tests := []struct {
		name        string
		headers     map[string]string
		endpoint    string
		expectCode  int
	}{
		{
			name: "with user agent",
			headers: map[string]string{
				"User-Agent": "TestClient/1.0",
			},
			endpoint:   "/xtz/delegations",
			expectCode: http.StatusOK,
		},
		{
			name: "with accept header",
			headers: map[string]string{
				"Accept": "application/json",
			},
			endpoint:   "/xtz/delegations",
			expectCode: http.StatusOK,
		},
		{
			name: "with authorization header (should be ignored)",
			headers: map[string]string{
				"Authorization": "Bearer token123",
			},
			endpoint:   "/xtz/delegations",
			expectCode: http.StatusOK,
		},
		{
			name: "with custom headers",
			headers: map[string]string{
				"X-Request-ID":   "test-123",
				"X-Forwarded-For": "192.168.1.1",
			},
			endpoint:   "/health",
			expectCode: http.StatusOK,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.endpoint, nil)
			
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}
			
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			
			assert.Equal(t, tt.expectCode, rec.Code)
		})
	}
	
	mockService.AssertExpectations(t)
}

func TestHandler_MethodNotAllowed(t *testing.T) {
	mockService := new(MockService)
	router := setupRouter(mockService)
	
	methods := []string{
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
		http.MethodOptions,
	}
	
	endpoints := []string{
		"/xtz/delegations",
		"/health",
		"/ready",
		"/stats",
	}
	
	for _, method := range methods {
		for _, endpoint := range endpoints {
			t.Run(fmt.Sprintf("%s %s", method, endpoint), func(t *testing.T) {
				req := httptest.NewRequest(method, endpoint, nil)
				rec := httptest.NewRecorder()
				
				router.ServeHTTP(rec, req)
				
				// Gin returns 404 for unmatched routes by default
				assert.Equal(t, http.StatusNotFound, rec.Code)
			})
		}
	}
}

// Benchmark tests
func BenchmarkHandler_GetDelegations(b *testing.B) {
	mockService := new(MockService)
	
	delegations := make([]domain.Delegation, 1000)
	for i := range delegations {
		delegations[i] = domain.Delegation{
			ID:        uuid.New().String(),
			Timestamp: time.Now(),
			Amount:    fmt.Sprintf("%d", i*1000000),
			Delegator: fmt.Sprintf("tz1test%d", i),
			Level:     fmt.Sprintf("%d", 2000000+i),
		}
	}
	
	mockService.On("GetDelegations", (*int)(nil)).Return(delegations, nil).Maybe()
	
	router := setupRouter(mockService)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/xtz/delegations", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
	}
}

func BenchmarkHandler_ConcurrentRequests(b *testing.B) {
	mockService := new(MockService)
	
	mockService.On("GetDelegations", (*int)(nil)).Return([]domain.Delegation{}, nil).Maybe()
	mockService.On("GetStats").Return(map[string]interface{}{
		"total_delegations": 100,
	}, nil).Maybe()
	
	router := setupRouter(mockService)
	
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			var endpoint string
			if i%2 == 0 {
				endpoint = "/xtz/delegations"
			} else {
				endpoint = "/stats"
			}
			
			req := httptest.NewRequest(http.MethodGet, endpoint, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			i++
		}
	})
}

// Helper function for setting up router with stats endpoint
func setupRouterWithStats(service domain.DelegationService) *gin.Engine {
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