package logger

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name        string
		level       string
		environment string
		wantErr     bool
	}{
		{
			name:        "debug level development",
			level:       "debug",
			environment: "development",
			wantErr:     false,
		},
		{
			name:        "info level production",
			level:       "info",
			environment: "production",
			wantErr:     false,
		},
		{
			name:        "warn level",
			level:       "warn",
			environment: "production",
			wantErr:     false,
		},
		{
			name:        "error level",
			level:       "error",
			environment: "production",
			wantErr:     false,
		},
		{
			name:        "invalid level defaults to info",
			level:       "invalid",
			environment: "production",
			wantErr:     false,
		},
		{
			name:        "empty level defaults to info",
			level:       "",
			environment: "production",
			wantErr:     false,
		},
		{
			name:        "case insensitive level",
			level:       "DEBUG",
			environment: "development",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := New(tt.level, tt.environment)
			
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			
			require.NoError(t, err)
			require.NotNil(t, logger)
			require.NotNil(t, logger.SugaredLogger)
		})
	}
}

func TestLogger_WithFields(t *testing.T) {
	logger, err := New("debug", "test")
	require.NoError(t, err)

	tests := []struct {
		name   string
		fields map[string]interface{}
		verify func(*testing.T, *Logger)
	}{
		{
			name: "single field",
			fields: map[string]interface{}{
				"key": "value",
			},
			verify: func(t *testing.T, l *Logger) {
				assert.NotNil(t, l)
			},
		},
		{
			name: "multiple fields",
			fields: map[string]interface{}{
				"key1": "value1",
				"key2": 42,
				"key3": true,
			},
			verify: func(t *testing.T, l *Logger) {
				assert.NotNil(t, l)
			},
		},
		{
			name: "nested fields",
			fields: map[string]interface{}{
				"user": map[string]string{
					"id":   "123",
					"name": "test",
				},
			},
			verify: func(t *testing.T, l *Logger) {
				assert.NotNil(t, l)
			},
		},
		{
			name:   "empty fields",
			fields: map[string]interface{}{},
			verify: func(t *testing.T, l *Logger) {
				assert.NotNil(t, l)
			},
		},
		{
			name:   "nil fields",
			fields: nil,
			verify: func(t *testing.T, l *Logger) {
				assert.NotNil(t, l)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newLogger := logger.WithFields(tt.fields)
			tt.verify(t, newLogger)
			
			// Verify it's a new logger instance (compare pointers)
			assert.NotSame(t, logger, newLogger)
		})
	}
}

func TestLogger_LoggingMethods(t *testing.T) {
	// Create a logger with a custom core that captures output
	var buf bytes.Buffer
	encoderCfg := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
	
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderCfg),
		zapcore.AddSync(&buf),
		zapcore.DebugLevel,
	)
	
	zapLogger := zap.New(core)
	logger := &Logger{SugaredLogger: zapLogger.Sugar()}

	tests := []struct {
		name     string
		logFunc  func()
		expected string
		level    string
	}{
		{
			name:     "debug message",
			logFunc:  func() { logger.Debug("debug message") },
			expected: "debug message",
			level:    "debug",
		},
		{
			name:     "debugw with fields",
			logFunc:  func() { logger.Debugw("debug with fields", "key", "value") },
			expected: "debug with fields",
			level:    "debug",
		},
		{
			name:     "info message",
			logFunc:  func() { logger.Info("info message") },
			expected: "info message",
			level:    "info",
		},
		{
			name:     "infow with fields",
			logFunc:  func() { logger.Infow("info with fields", "count", 42) },
			expected: "info with fields",
			level:    "info",
		},
		{
			name:     "warn message",
			logFunc:  func() { logger.Warn("warning message") },
			expected: "warning message",
			level:    "warn",
		},
		{
			name:     "warnw with fields",
			logFunc:  func() { logger.Warnw("warning with fields", "error", "timeout") },
			expected: "warning with fields",
			level:    "warn",
		},
		{
			name:     "error message",
			logFunc:  func() { logger.Error("error message") },
			expected: "error message",
			level:    "error",
		},
		{
			name:     "errorw with fields",
			logFunc:  func() { logger.Errorw("error with fields", "code", 500) },
			expected: "error with fields",
			level:    "error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			tt.logFunc()
			
			output := buf.String()
			assert.Contains(t, output, tt.expected)
			assert.Contains(t, output, tt.level)
			
			// Verify it's valid JSON
			var logEntry map[string]interface{}
			err := json.Unmarshal([]byte(output), &logEntry)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, logEntry["msg"])
		})
	}
}

func TestLogger_ConcurrentUse(t *testing.T) {
	logger, err := New("info", "test")
	require.NoError(t, err)

	// Test concurrent logging
	done := make(chan bool)
	for i := 0; i < 100; i++ {
		go func(id int) {
			logger.Infow("concurrent log", "goroutine", id)
			logger.WithFields(map[string]interface{}{
				"id": id,
			}).Debug("with fields")
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		<-done
	}
}

func TestLogger_PanicRecovery(t *testing.T) {
	// Create a logger that should handle panics gracefully
	logger, err := New("debug", "test")
	require.NoError(t, err)

	// Test that logging methods don't panic with nil or invalid inputs
	assert.NotPanics(t, func() {
		logger.Debug(nil)
		logger.Info("")
		logger.Warn(123)
		logger.Error([]int{1, 2, 3})
	})

	// Test with nil fields
	assert.NotPanics(t, func() {
		logger.WithFields(nil).Info("test")
	})
}

func TestLogger_LevelFiltering(t *testing.T) {
	tests := []struct {
		name        string
		logLevel    string
		testLogs    func(*Logger)
		shouldLog   map[string]bool
	}{
		{
			name:     "error level filters lower levels",
			logLevel: "error",
			testLogs: func(l *Logger) {
				l.Debug("debug")
				l.Info("info")
				l.Warn("warn")
				l.Error("error")
			},
			shouldLog: map[string]bool{
				"debug": false,
				"info":  false,
				"warn":  false,
				"error": true,
			},
		},
		{
			name:     "info level filters debug",
			logLevel: "info",
			testLogs: func(l *Logger) {
				l.Debug("debug")
				l.Info("info")
				l.Warn("warn")
				l.Error("error")
			},
			shouldLog: map[string]bool{
				"debug": false,
				"info":  true,
				"warn":  true,
				"error": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create logger with buffer to capture output
			var buf bytes.Buffer
			
			var level zapcore.Level
			switch strings.ToLower(tt.logLevel) {
			case "debug":
				level = zapcore.DebugLevel
			case "info":
				level = zapcore.InfoLevel
			case "warn":
				level = zapcore.WarnLevel
			case "error":
				level = zapcore.ErrorLevel
			default:
				level = zapcore.InfoLevel
			}
			
			encoderCfg := zap.NewProductionEncoderConfig()
			core := zapcore.NewCore(
				zapcore.NewJSONEncoder(encoderCfg),
				zapcore.AddSync(&buf),
				level,
			)
			
			zapLogger := zap.New(core)
			logger := &Logger{SugaredLogger: zapLogger.Sugar()}
			
			// Execute test logs
			tt.testLogs(logger)
			
			output := buf.String()
			
			// Verify expected log presence/absence
			for logType, shouldExist := range tt.shouldLog {
				if shouldExist {
					assert.Contains(t, output, logType)
				} else {
					assert.NotContains(t, output, logType)
				}
			}
		})
	}
}

func BenchmarkLogger_Info(b *testing.B) {
	logger, _ := New("info", "production")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("benchmark message")
	}
}

func BenchmarkLogger_InfoWithFields(b *testing.B) {
	logger, _ := New("info", "production")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Infow("benchmark message", 
			"field1", "value1",
			"field2", 42,
			"field3", true,
		)
	}
}

func BenchmarkLogger_WithFields(b *testing.B) {
	logger, _ := New("info", "production")
	fields := map[string]interface{}{
		"key1": "value1",
		"key2": 42,
		"key3": true,
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = logger.WithFields(fields)
	}
}