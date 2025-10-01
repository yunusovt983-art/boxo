package monitoring

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"testing"
	"time"
)

// Mock output for testing
type mockLogOutput struct {
	name    string
	entries []LogEntry
	mu      sync.RWMutex
}

func newMockLogOutput(name string) *mockLogOutput {
	return &mockLogOutput{
		name:    name,
		entries: make([]LogEntry, 0),
	}
}

func (mlo *mockLogOutput) Write(entry LogEntry) error {
	mlo.mu.Lock()
	defer mlo.mu.Unlock()
	
	mlo.entries = append(mlo.entries, entry)
	return nil
}

func (mlo *mockLogOutput) Flush() error {
	return nil
}

func (mlo *mockLogOutput) Close() error {
	return nil
}

func (mlo *mockLogOutput) GetName() string {
	return mlo.name
}

func (mlo *mockLogOutput) getEntries() []LogEntry {
	mlo.mu.RLock()
	defer mlo.mu.RUnlock()
	
	return append([]LogEntry(nil), mlo.entries...)
}

func (mlo *mockLogOutput) clear() {
	mlo.mu.Lock()
	defer mlo.mu.Unlock()
	
	mlo.entries = mlo.entries[:0]
}

// Mock hook for testing
type mockLogHook struct {
	name      string
	processed []LogEntry
	mu        sync.RWMutex
}

func newMockLogHook(name string) *mockLogHook {
	return &mockLogHook{
		name:      name,
		processed: make([]LogEntry, 0),
	}
}

func (mlh *mockLogHook) Process(entry LogEntry) (LogEntry, error) {
	mlh.mu.Lock()
	defer mlh.mu.Unlock()
	
	// Add a field to show the hook processed it
	if entry.Fields == nil {
		entry.Fields = make(map[string]interface{})
	}
	entry.Fields["processed_by"] = mlh.name
	
	mlh.processed = append(mlh.processed, entry)
	return entry, nil
}

func (mlh *mockLogHook) GetName() string {
	return mlh.name
}

func (mlh *mockLogHook) ShouldProcess(level slog.Level) bool {
	return true
}

func (mlh *mockLogHook) getProcessed() []LogEntry {
	mlh.mu.RLock()
	defer mlh.mu.RUnlock()
	
	return append([]LogEntry(nil), mlh.processed...)
}

func TestStructuredLogger_BasicLogging(t *testing.T) {
	mockOutput := newMockLogOutput("test")
	logger := NewStructuredLogger(
		WithLevel(slog.LevelDebug),
		WithOutput("test", mockOutput),
	)
	
	ctx := context.Background()
	
	t.Run("InfoLogging", func(t *testing.T) {
		mockOutput.clear()
		
		logger.InfoWithContext(ctx, "Test info message",
			slog.String("key1", "value1"),
			slog.Int("key2", 42),
		)
		
		entries := mockOutput.getEntries()
		if len(entries) != 1 {
			t.Fatalf("Expected 1 log entry, got %d", len(entries))
		}
		
		entry := entries[0]
		if entry.Level != slog.LevelInfo {
			t.Errorf("Expected level Info, got %v", entry.Level)
		}
		if entry.Message != "Test info message" {
			t.Errorf("Expected message 'Test info message', got '%s'", entry.Message)
		}
		if entry.Fields["key1"] != "value1" {
			t.Errorf("Expected field key1='value1', got %v", entry.Fields["key1"])
		}
		if entry.Fields["key2"] != 42 {
			t.Errorf("Expected field key2=42, got %v", entry.Fields["key2"])
		}
	})
	
	t.Run("ErrorLogging", func(t *testing.T) {
		mockOutput.clear()
		
		testErr := &testError{message: "test error"}
		logger.ErrorWithContext(ctx, "Test error message",
			slog.Any("error", testErr),
		)
		
		entries := mockOutput.getEntries()
		if len(entries) != 1 {
			t.Fatalf("Expected 1 log entry, got %d", len(entries))
		}
		
		entry := entries[0]
		if entry.Level != slog.LevelError {
			t.Errorf("Expected level Error, got %v", entry.Level)
		}
		if entry.Error != "test error" {
			t.Errorf("Expected error 'test error', got '%s'", entry.Error)
		}
		if entry.Stack == "" {
			t.Error("Expected stack trace to be populated")
		}
	})
	
	t.Run("LevelFiltering", func(t *testing.T) {
		mockOutput.clear()
		
		// Set level to Warn, debug messages should be filtered
		logger.SetLevel(slog.LevelWarn)
		
		logger.DebugWithContext(ctx, "Debug message")
		logger.InfoWithContext(ctx, "Info message")
		logger.WarnWithContext(ctx, "Warn message")
		logger.ErrorWithContext(ctx, "Error message")
		
		entries := mockOutput.getEntries()
		if len(entries) != 2 {
			t.Fatalf("Expected 2 log entries (Warn and Error), got %d", len(entries))
		}
		
		if entries[0].Level != slog.LevelWarn {
			t.Errorf("Expected first entry to be Warn, got %v", entries[0].Level)
		}
		if entries[1].Level != slog.LevelError {
			t.Errorf("Expected second entry to be Error, got %v", entries[1].Level)
		}
	})
}

type testError struct {
	message string
}

func (te *testError) Error() string {
	return te.message
}

func TestStructuredLogger_ContextHandling(t *testing.T) {
	mockOutput := newMockLogOutput("test")
	logger := NewStructuredLogger(
		WithLevel(slog.LevelDebug),
		WithOutput("test", mockOutput),
	)
	
	t.Run("LogContext", func(t *testing.T) {
		mockOutput.clear()
		
		logCtx := &LogContext{
			TraceID:   "trace-123",
			SpanID:    "span-456",
			RequestID: "req-789",
			UserID:    "user-abc",
			Component: "test-component",
			Operation: "test-operation",
			StartTime: time.Now().Add(-100 * time.Millisecond),
			Fields: map[string]interface{}{
				"custom_field": "custom_value",
			},
		}
		
		ctx := WithLogContext(context.Background(), logCtx)
		
		logger.InfoWithContext(ctx, "Test message with context")
		
		entries := mockOutput.getEntries()
		if len(entries) != 1 {
			t.Fatalf("Expected 1 log entry, got %d", len(entries))
		}
		
		entry := entries[0]
		if entry.TraceID != "trace-123" {
			t.Errorf("Expected TraceID 'trace-123', got '%s'", entry.TraceID)
		}
		if entry.SpanID != "span-456" {
			t.Errorf("Expected SpanID 'span-456', got '%s'", entry.SpanID)
		}
		if entry.RequestID != "req-789" {
			t.Errorf("Expected RequestID 'req-789', got '%s'", entry.RequestID)
		}
		if entry.UserID != "user-abc" {
			t.Errorf("Expected UserID 'user-abc', got '%s'", entry.UserID)
		}
		if entry.Component != "test-component" {
			t.Errorf("Expected Component 'test-component', got '%s'", entry.Component)
		}
		if entry.Operation != "test-operation" {
			t.Errorf("Expected Operation 'test-operation', got '%s'", entry.Operation)
		}
		if entry.Fields["custom_field"] != "custom_value" {
			t.Errorf("Expected custom_field 'custom_value', got %v", entry.Fields["custom_field"])
		}
		if entry.Duration == nil {
			t.Error("Expected duration to be calculated")
		}
	})
	
	t.Run("RequestTracing", func(t *testing.T) {
		mockOutput.clear()
		
		ctx := context.Background()
		
		// Start request
		ctxWithRequest := logger.LogRequestStart(ctx, "test_operation", map[string]interface{}{
			"param1": "value1",
			"param2": 42,
		})
		
		// Simulate some work
		time.Sleep(10 * time.Millisecond)
		
		// End request
		logger.LogRequestEnd(ctxWithRequest, "test_operation", 10*time.Millisecond, nil)
		
		entries := mockOutput.getEntries()
		if len(entries) != 2 {
			t.Fatalf("Expected 2 log entries (start and end), got %d", len(entries))
		}
		
		startEntry := entries[0]
		endEntry := entries[1]
		
		if startEntry.Operation != "test_operation" {
			t.Errorf("Expected start operation 'test_operation', got '%s'", startEntry.Operation)
		}
		if startEntry.Fields["param1"] != "value1" {
			t.Errorf("Expected param1 'value1', got %v", startEntry.Fields["param1"])
		}
		
		if endEntry.Operation != "test_operation" {
			t.Errorf("Expected end operation 'test_operation', got '%s'", endEntry.Operation)
		}
		if endEntry.Duration == nil {
			t.Error("Expected duration in end entry")
		}
		
		// Both entries should have the same trace and request IDs
		if startEntry.TraceID != endEntry.TraceID {
			t.Error("Start and end entries should have the same trace ID")
		}
		if startEntry.RequestID != endEntry.RequestID {
			t.Error("Start and end entries should have the same request ID")
		}
	})
}

func TestStructuredLogger_Hooks(t *testing.T) {
	mockOutput := newMockLogOutput("test")
	mockHook := newMockLogHook("test-hook")
	
	logger := NewStructuredLogger(
		WithLevel(slog.LevelDebug),
		WithOutput("test", mockOutput),
		WithHook(mockHook),
	)
	
	t.Run("HookProcessing", func(t *testing.T) {
		mockOutput.clear()
		
		ctx := context.Background()
		logger.InfoWithContext(ctx, "Test message")
		
		entries := mockOutput.getEntries()
		if len(entries) != 1 {
			t.Fatalf("Expected 1 log entry, got %d", len(entries))
		}
		
		entry := entries[0]
		if entry.Fields["processed_by"] != "test-hook" {
			t.Errorf("Expected hook to add processed_by field, got %v", entry.Fields["processed_by"])
		}
		
		processed := mockHook.getProcessed()
		if len(processed) != 1 {
			t.Errorf("Expected hook to process 1 entry, got %d", len(processed))
		}
	})
	
	t.Run("AddRemoveHook", func(t *testing.T) {
		newHook := newMockLogHook("new-hook")
		
		err := logger.AddHook(newHook)
		if err != nil {
			t.Errorf("Expected no error adding hook, got %v", err)
		}
		
		err = logger.RemoveHook("test-hook")
		if err != nil {
			t.Errorf("Expected no error removing hook, got %v", err)
		}
	})
}

func TestStructuredLogger_PerformanceLogging(t *testing.T) {
	mockOutput := newMockLogOutput("test")
	logger := NewStructuredLogger(
		WithLevel(slog.LevelDebug),
		WithOutput("test", mockOutput),
	)
	
	ctx := context.Background()
	
	t.Run("PerformanceMetrics", func(t *testing.T) {
		mockOutput.clear()
		
		metrics := map[string]interface{}{
			"cpu_usage":    75.5,
			"memory_usage": 1024 * 1024 * 512, // 512MB
			"requests_per_second": 100.0,
		}
		
		logger.LogPerformanceMetrics(ctx, "test-component", metrics)
		
		entries := mockOutput.getEntries()
		if len(entries) != 1 {
			t.Fatalf("Expected 1 log entry, got %d", len(entries))
		}
		
		entry := entries[0]
		if entry.Fields["type"] != "performance_metrics" {
			t.Errorf("Expected type 'performance_metrics', got %v", entry.Fields["type"])
		}
		if entry.Fields["component"] != "test-component" {
			t.Errorf("Expected component 'test-component', got %v", entry.Fields["component"])
		}
		if entry.Fields["cpu_usage"] != 75.5 {
			t.Errorf("Expected cpu_usage 75.5, got %v", entry.Fields["cpu_usage"])
		}
	})
	
	t.Run("SlowOperation", func(t *testing.T) {
		mockOutput.clear()
		
		logger.LogSlowOperation(ctx, "slow_db_query", 500*time.Millisecond, 100*time.Millisecond)
		
		entries := mockOutput.getEntries()
		if len(entries) != 1 {
			t.Fatalf("Expected 1 log entry, got %d", len(entries))
		}
		
		entry := entries[0]
		if entry.Level != slog.LevelWarn {
			t.Errorf("Expected level Warn, got %v", entry.Level)
		}
		if entry.Fields["type"] != "slow_operation" {
			t.Errorf("Expected type 'slow_operation', got %v", entry.Fields["type"])
		}
		if entry.Fields["operation"] != "slow_db_query" {
			t.Errorf("Expected operation 'slow_db_query', got %v", entry.Fields["operation"])
		}
	})
	
	t.Run("BottleneckLogging", func(t *testing.T) {
		mockOutput.clear()
		
		bottleneck := Bottleneck{
			ID:          "bottleneck-1",
			Component:   "bitswap",
			Type:        BottleneckTypeLatency,
			Severity:    SeverityHigh,
			Description: "High latency detected",
			DetectedAt:  time.Now(),
			Metrics: map[string]interface{}{
				"latency": 200 * time.Millisecond,
			},
		}
		
		logger.LogBottleneck(ctx, bottleneck)
		
		entries := mockOutput.getEntries()
		if len(entries) != 1 {
			t.Fatalf("Expected 1 log entry, got %d", len(entries))
		}
		
		entry := entries[0]
		if entry.Level != slog.LevelWarn {
			t.Errorf("Expected level Warn, got %v", entry.Level)
		}
		if entry.Fields["type"] != "bottleneck" {
			t.Errorf("Expected type 'bottleneck', got %v", entry.Fields["type"])
		}
		if entry.Fields["bottleneck_id"] != "bottleneck-1" {
			t.Errorf("Expected bottleneck_id 'bottleneck-1', got %v", entry.Fields["bottleneck_id"])
		}
	})
	
	t.Run("AlertLogging", func(t *testing.T) {
		mockOutput.clear()
		
		alert := Alert{
			ID:          "alert-1",
			RuleID:      "rule-1",
			Title:       "High CPU Usage",
			Component:   "resource",
			Metric:      "cpu_usage",
			Severity:    SeverityCritical,
			State:       AlertStateFiring,
			Value:       95.0,
			Threshold:   80.0,
			FireCount:   3,
			CreatedAt:   time.Now(),
			TraceID:     "trace-123",
			Labels: map[string]string{
				"environment": "production",
			},
		}
		
		logger.LogAlert(ctx, alert)
		
		entries := mockOutput.getEntries()
		if len(entries) != 1 {
			t.Fatalf("Expected 1 log entry, got %d", len(entries))
		}
		
		entry := entries[0]
		if entry.Level != slog.LevelError { // Critical severity maps to Error level
			t.Errorf("Expected level Error, got %v", entry.Level)
		}
		if entry.Fields["type"] != "alert" {
			t.Errorf("Expected type 'alert', got %v", entry.Fields["type"])
		}
		if entry.Fields["alert_id"] != "alert-1" {
			t.Errorf("Expected alert_id 'alert-1', got %v", entry.Fields["alert_id"])
		}
		if entry.Fields["trace_id"] != "trace-123" {
			t.Errorf("Expected trace_id 'trace-123', got %v", entry.Fields["trace_id"])
		}
	})
}

func TestStructuredLogger_WithMethods(t *testing.T) {
	mockOutput := newMockLogOutput("test")
	logger := NewStructuredLogger(
		WithLevel(slog.LevelDebug),
		WithOutput("test", mockOutput),
	)
	
	t.Run("WithFields", func(t *testing.T) {
		mockOutput.clear()
		
		fieldsLogger := logger.WithFields(map[string]interface{}{
			"service": "test-service",
			"version": "1.0.0",
		})
		
		ctx := context.Background()
		fieldsLogger.InfoWithContext(ctx, "Test message")
		
		entries := mockOutput.getEntries()
		if len(entries) != 1 {
			t.Fatalf("Expected 1 log entry, got %d", len(entries))
		}
		
		entry := entries[0]
		if entry.Fields["service"] != "test-service" {
			t.Errorf("Expected service 'test-service', got %v", entry.Fields["service"])
		}
		if entry.Fields["version"] != "1.0.0" {
			t.Errorf("Expected version '1.0.0', got %v", entry.Fields["version"])
		}
	})
	
	t.Run("WithComponent", func(t *testing.T) {
		mockOutput.clear()
		
		componentLogger := logger.WithComponent("bitswap")
		
		ctx := context.Background()
		componentLogger.InfoWithContext(ctx, "Test message")
		
		entries := mockOutput.getEntries()
		if len(entries) != 1 {
			t.Fatalf("Expected 1 log entry, got %d", len(entries))
		}
		
		entry := entries[0]
		if entry.Component != "bitswap" {
			t.Errorf("Expected component 'bitswap', got '%s'", entry.Component)
		}
	})
}

func TestConsoleOutput(t *testing.T) {
	output := NewConsoleOutput()
	
	t.Run("BasicWrite", func(t *testing.T) {
		entry := LogEntry{
			Timestamp: time.Now(),
			Level:     slog.LevelInfo,
			Message:   "Test message",
			Component: "test",
			Fields: map[string]interface{}{
				"key": "value",
			},
		}
		
		err := output.Write(entry)
		if err != nil {
			t.Errorf("Expected no error writing to console, got %v", err)
		}
	})
	
	t.Run("FlushAndClose", func(t *testing.T) {
		err := output.Flush()
		if err != nil {
			t.Errorf("Expected no error flushing console, got %v", err)
		}
		
		err = output.Close()
		if err != nil {
			t.Errorf("Expected no error closing console, got %v", err)
		}
	})
	
	t.Run("GetName", func(t *testing.T) {
		name := output.GetName()
		if name != "console" {
			t.Errorf("Expected name 'console', got '%s'", name)
		}
	})
}

func TestContextHelpers(t *testing.T) {
	t.Run("LogContext", func(t *testing.T) {
		ctx := context.Background()
		
		logCtx := &LogContext{
			TraceID:   "trace-123",
			RequestID: "req-456",
		}
		
		ctxWithLog := WithLogContext(ctx, logCtx)
		retrievedCtx := GetLogContext(ctxWithLog)
		
		if retrievedCtx == nil {
			t.Fatal("Expected log context to be retrieved")
		}
		if retrievedCtx.TraceID != "trace-123" {
			t.Errorf("Expected TraceID 'trace-123', got '%s'", retrievedCtx.TraceID)
		}
		if retrievedCtx.RequestID != "req-456" {
			t.Errorf("Expected RequestID 'req-456', got '%s'", retrievedCtx.RequestID)
		}
	})
	
	t.Run("TraceID", func(t *testing.T) {
		ctx := context.Background()
		ctxWithTrace := WithTraceID(ctx, "trace-789")
		
		// This would be used internally by the logger
		if traceID := ctxWithTrace.Value(traceIDKey); traceID != "trace-789" {
			t.Errorf("Expected trace ID 'trace-789', got %v", traceID)
		}
	})
	
	t.Run("RequestID", func(t *testing.T) {
		ctx := context.Background()
		ctxWithRequest := WithRequestID(ctx, "req-789")
		
		if requestID := ctxWithRequest.Value(requestIDKey); requestID != "req-789" {
			t.Errorf("Expected request ID 'req-789', got %v", requestID)
		}
	})
	
	t.Run("UserID", func(t *testing.T) {
		ctx := context.Background()
		ctxWithUser := WithUserID(ctx, "user-123")
		
		if userID := ctxWithUser.Value(userIDKey); userID != "user-123" {
			t.Errorf("Expected user ID 'user-123', got %v", userID)
		}
	})
}

// Benchmark tests

func BenchmarkStructuredLogger_InfoLogging(b *testing.B) {
	mockOutput := newMockLogOutput("test")
	logger := NewStructuredLogger(
		WithLevel(slog.LevelInfo),
		WithOutput("test", mockOutput),
	)
	
	ctx := context.Background()
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		logger.InfoWithContext(ctx, "Benchmark message",
			slog.String("key1", "value1"),
			slog.Int("key2", i),
		)
	}
}

func BenchmarkStructuredLogger_WithContext(b *testing.B) {
	mockOutput := newMockLogOutput("test")
	logger := NewStructuredLogger(
		WithLevel(slog.LevelInfo),
		WithOutput("test", mockOutput),
	)
	
	logCtx := &LogContext{
		TraceID:   "trace-123",
		RequestID: "req-456",
		Component: "test-component",
		Fields: map[string]interface{}{
			"field1": "value1",
			"field2": 42,
		},
	}
	
	ctx := WithLogContext(context.Background(), logCtx)
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		logger.InfoWithContext(ctx, "Benchmark message with context")
	}
}

func BenchmarkStructuredLogger_JSONSerialization(b *testing.B) {
	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     slog.LevelInfo,
		Message:   "Test message",
		Component: "test-component",
		TraceID:   "trace-123",
		RequestID: "req-456",
		Fields: map[string]interface{}{
			"key1": "value1",
			"key2": 42,
			"key3": 3.14,
			"key4": true,
		},
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_, err := json.Marshal(entry)
		if err != nil {
			b.Fatal(err)
		}
	}
}