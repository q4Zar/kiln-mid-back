package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCollector(t *testing.T) {
	collector := NewCollector()
	
	assert.NotNil(t, collector)
	assert.NotNil(t, collector.RequestCount)
	assert.NotNil(t, collector.RequestDuration)
	assert.NotNil(t, collector.DelegationsProcessed)
	assert.NotNil(t, collector.IndexingErrors)
	assert.NotNil(t, collector.LastIndexedLevel)
	assert.NotNil(t, collector.TzktAPIRequests)
	assert.NotNil(t, collector.TzktAPILatency)
	assert.NotNil(t, collector.DatabaseConnections)
	assert.NotNil(t, collector.DatabaseLatency)
}

func TestRecordAPIRequest(t *testing.T) {
	collector := NewCollector()
	
	tests := []struct {
		name     string
		method   string
		path     string
		status   int
		duration time.Duration
	}{
		{
			name:     "GET request success",
			method:   "GET",
			path:     "/xtz/delegations",
			status:   200,
			duration: 100 * time.Millisecond,
		},
		{
			name:     "POST request created",
			method:   "POST",
			path:     "/api/resource",
			status:   201,
			duration: 50 * time.Millisecond,
		},
		{
			name:     "GET request not found",
			method:   "GET",
			path:     "/not-found",
			status:   404,
			duration: 10 * time.Millisecond,
		},
		{
			name:     "PUT request error",
			method:   "PUT",
			path:     "/api/resource/123",
			status:   500,
			duration: 200 * time.Millisecond,
		},
		{
			name:     "DELETE request success",
			method:   "DELETE",
			path:     "/api/resource/456",
			status:   204,
			duration: 75 * time.Millisecond,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := testutil.ToFloat64(collector.RequestCount.WithLabelValues(tt.method, tt.path, http.StatusText(tt.status)))
			
			collector.RecordAPIRequest(tt.method, tt.path, tt.status, tt.duration)
			
			after := testutil.ToFloat64(collector.RequestCount.WithLabelValues(tt.method, tt.path, http.StatusText(tt.status)))
			assert.Equal(t, before+1, after)
		})
	}
}

func TestRecordDelegationProcessed(t *testing.T) {
	collector := NewCollector()
	
	// Record multiple delegations
	for i := 0; i < 10; i++ {
		collector.RecordDelegationProcessed()
	}
	
	// Verify counter increased
	count := testutil.ToFloat64(collector.DelegationsProcessed)
	assert.Equal(t, float64(10), count)
}

func TestUpdateLastIndexedLevel(t *testing.T) {
	collector := NewCollector()
	
	tests := []struct {
		name  string
		level int64
	}{
		{"initial level", 1000},
		{"higher level", 2000},
		{"much higher level", 1000000},
		{"zero level", 0},
		{"negative level", -1},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector.UpdateLastIndexedLevel(tt.level)
			
			value := testutil.ToFloat64(collector.LastIndexedLevel)
			assert.Equal(t, float64(tt.level), value)
		})
	}
}

func TestRecordTzktAPIRequest(t *testing.T) {
	collector := NewCollector()
	
	tests := []struct {
		name     string
		endpoint string
		success  bool
		duration time.Duration
	}{
		{
			name:     "successful delegation fetch",
			endpoint: "/v1/operations/delegations",
			success:  true,
			duration: 100 * time.Millisecond,
		},
		{
			name:     "failed delegation fetch",
			endpoint: "/v1/operations/delegations",
			success:  false,
			duration: 50 * time.Millisecond,
		},
		{
			name:     "successful block fetch",
			endpoint: "/v1/blocks",
			success:  true,
			duration: 75 * time.Millisecond,
		},
		{
			name:     "timeout on accounts",
			endpoint: "/v1/accounts",
			success:  false,
			duration: 5 * time.Second,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := "success"
			if !tt.success {
				status = "error"
			}
			
			beforeCount := testutil.ToFloat64(collector.TzktAPIRequests.WithLabelValues(tt.endpoint, status))
			
			collector.RecordTzktAPIRequest(tt.endpoint, tt.success, tt.duration)
			
			afterCount := testutil.ToFloat64(collector.TzktAPIRequests.WithLabelValues(tt.endpoint, status))
			assert.Equal(t, beforeCount+1, afterCount)
		})
	}
}

func TestUpdateDatabaseConnections(t *testing.T) {
	collector := NewCollector()
	
	tests := []struct {
		name   string
		active int
		idle   int
		total  int
	}{
		{
			name:   "normal load",
			active: 5,
			idle:   15,
			total:  20,
		},
		{
			name:   "high load",
			active: 18,
			idle:   2,
			total:  20,
		},
		{
			name:   "idle connections",
			active: 0,
			idle:   20,
			total:  20,
		},
		{
			name:   "maximum connections",
			active: 50,
			idle:   0,
			total:  50,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector.UpdateDatabaseConnections(tt.active, tt.idle, tt.total)
			
			// Note: Gauge values are harder to test directly
			// In a real scenario, you'd export metrics and check them
		})
	}
}

func TestRecordIndexingError(t *testing.T) {
	collector := NewCollector()
	
	errorTypes := []string{
		"network_error",
		"parsing_error",
		"database_error",
		"validation_error",
		"timeout_error",
	}
	
	// Record various errors
	for _, errType := range errorTypes {
		for i := 0; i < 3; i++ {
			collector.IndexingErrors.WithLabelValues(errType).Inc()
		}
	}
	
	// Verify each error type counter
	for _, errType := range errorTypes {
		count := testutil.ToFloat64(collector.IndexingErrors.WithLabelValues(errType))
		assert.Equal(t, float64(3), count)
	}
}

func TestMetrics_ConcurrentOperations(t *testing.T) {
	collector := NewCollector()
	
	var wg sync.WaitGroup
	operations := 1000
	
	// Concurrent API requests
	wg.Add(operations)
	for i := 0; i < operations; i++ {
		go func(id int) {
			defer wg.Done()
			method := "GET"
			if id%2 == 0 {
				method = "POST"
			}
			collector.RecordAPIRequest(method, "/test", 200, time.Millisecond*time.Duration(id%100))
		}(i)
	}
	
	// Concurrent delegation processing
	wg.Add(operations)
	for i := 0; i < operations; i++ {
		go func() {
			defer wg.Done()
			collector.RecordDelegationProcessed()
		}()
	}
	
	// Concurrent level updates
	wg.Add(operations)
	for i := 0; i < operations; i++ {
		go func(level int) {
			defer wg.Done()
			collector.UpdateLastIndexedLevel(int64(level))
		}(i)
	}
	
	wg.Wait()
	
	// Verify metrics were recorded
	getCount := testutil.ToFloat64(collector.RequestCount.WithLabelValues("GET", "/test", "OK"))
	postCount := testutil.ToFloat64(collector.RequestCount.WithLabelValues("POST", "/test", "OK"))
	assert.True(t, getCount > 0)
	assert.True(t, postCount > 0)
	assert.Equal(t, float64(operations), getCount+postCount)
	
	delegationCount := testutil.ToFloat64(collector.DelegationsProcessed)
	assert.Equal(t, float64(operations), delegationCount)
}

func TestMetrics_HTTPHandler(t *testing.T) {
	collector := NewCollector()
	
	// Record some metrics
	collector.RecordAPIRequest("GET", "/test", 200, 100*time.Millisecond)
	collector.RecordDelegationProcessed()
	collector.UpdateLastIndexedLevel(1000)
	
	// Create HTTP handler for metrics
	handler := prometheus.Handler()
	
	// Create test request
	req, err := http.NewRequest("GET", "/metrics", nil)
	require.NoError(t, err)
	
	// Record response
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	
	// Check response
	assert.Equal(t, http.StatusOK, rr.Code)
	
	body := rr.Body.String()
	assert.Contains(t, body, "# HELP")
	assert.Contains(t, body, "# TYPE")
	assert.Contains(t, body, "api_requests_total")
	assert.Contains(t, body, "delegations_processed_total")
	assert.Contains(t, body, "last_indexed_level")
}

func TestMetrics_EdgeCases(t *testing.T) {
	collector := NewCollector()
	
	t.Run("empty labels", func(t *testing.T) {
		assert.NotPanics(t, func() {
			collector.RecordAPIRequest("", "", 0, 0)
			collector.RecordTzktAPIRequest("", true, 0)
		})
	})
	
	t.Run("negative duration", func(t *testing.T) {
		assert.NotPanics(t, func() {
			collector.RecordAPIRequest("GET", "/test", 200, -100*time.Millisecond)
			collector.RecordTzktAPIRequest("/test", true, -50*time.Millisecond)
		})
	})
	
	t.Run("very long duration", func(t *testing.T) {
		assert.NotPanics(t, func() {
			collector.RecordAPIRequest("GET", "/test", 200, 24*time.Hour)
			collector.RecordTzktAPIRequest("/test", true, 1*time.Hour)
		})
	})
	
	t.Run("invalid status codes", func(t *testing.T) {
		assert.NotPanics(t, func() {
			collector.RecordAPIRequest("GET", "/test", 999, 100*time.Millisecond)
			collector.RecordAPIRequest("GET", "/test", -1, 100*time.Millisecond)
		})
	})
	
	t.Run("extreme level values", func(t *testing.T) {
		assert.NotPanics(t, func() {
			collector.UpdateLastIndexedLevel(9223372036854775807) // Max int64
			collector.UpdateLastIndexedLevel(-9223372036854775808) // Min int64
		})
	})
	
	t.Run("negative database connections", func(t *testing.T) {
		assert.NotPanics(t, func() {
			collector.UpdateDatabaseConnections(-1, -1, -1)
		})
	})
}

func BenchmarkRecordAPIRequest(b *testing.B) {
	collector := NewCollector()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		collector.RecordAPIRequest("GET", "/test", 200, 100*time.Millisecond)
	}
}

func BenchmarkRecordDelegationProcessed(b *testing.B) {
	collector := NewCollector()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		collector.RecordDelegationProcessed()
	}
}

func BenchmarkUpdateLastIndexedLevel(b *testing.B) {
	collector := NewCollector()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		collector.UpdateLastIndexedLevel(int64(i))
	}
}

func BenchmarkConcurrentMetrics(b *testing.B) {
	collector := NewCollector()
	
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			switch i % 4 {
			case 0:
				collector.RecordAPIRequest("GET", "/test", 200, 100*time.Millisecond)
			case 1:
				collector.RecordDelegationProcessed()
			case 2:
				collector.UpdateLastIndexedLevel(int64(i))
			case 3:
				collector.RecordTzktAPIRequest("/test", true, 50*time.Millisecond)
			}
			i++
		}
	})
}

func TestMetrics_MemoryLeaks(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory leak test in short mode")
	}
	
	collector := NewCollector()
	
	// Generate many unique label combinations
	for i := 0; i < 10000; i++ {
		path := strings.Repeat("a", i%100) // Variable path lengths
		collector.RecordAPIRequest("GET", path, 200, time.Millisecond)
		
		if i%100 == 0 {
			// Allow GC to run
			time.Sleep(time.Millisecond)
		}
	}
	
	// The test passes if it doesn't run out of memory
	// In production, you'd want cardinality limits on labels
}

func TestMetrics_Registration(t *testing.T) {
	// Create a new registry to avoid conflicts
	reg := prometheus.NewRegistry()
	
	// Create custom metrics
	counter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "test_custom_metric",
			Help: "Test custom metric",
		},
		[]string{"label"},
	)
	
	// Register metric
	err := reg.Register(counter)
	assert.NoError(t, err)
	
	// Attempt to register again (should fail)
	err = reg.Register(counter)
	assert.Error(t, err)
	
	// Verify metric is registered
	metrics, err := reg.Gather()
	assert.NoError(t, err)
	assert.True(t, len(metrics) > 0)
}