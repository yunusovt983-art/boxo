package monitoring

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

func TestRequestTracer_BasicFunctionality(t *testing.T) {
	logger := NewStructuredLogger(WithLevel(slog.LevelDebug))
	tracer := NewRequestTracer(logger)
	
	t.Run("StartTrace", func(t *testing.T) {
		ctx := context.Background()
		
		traceCtx, err := tracer.StartTrace(ctx, "test_operation")
		if err != nil {
			t.Fatalf("Expected no error starting trace, got %v", err)
		}
		
		if traceCtx.TraceID == "" {
			t.Error("Expected trace ID to be generated")
		}
		if traceCtx.SpanID == "" {
			t.Error("Expected span ID to be generated")
		}
		if traceCtx.Operation != "test_operation" {
			t.Errorf("Expected operation 'test_operation', got '%s'", traceCtx.Operation)
		}
		if traceCtx.StartTime.IsZero() {
			t.Error("Expected start time to be set")
		}
	})
	
	t.Run("EndTrace", func(t *testing.T) {
		ctx := context.Background()
		
		traceCtx, err := tracer.StartTrace(ctx, "test_operation")
		if err != nil {
			t.Fatalf("Failed to start trace: %v", err)
		}
		
		// Simulate some work
		time.Sleep(10 * time.Millisecond)
		
		err = tracer.EndTrace(traceCtx, nil)
		if err != nil {
			t.Errorf("Expected no error ending trace, got %v", err)
		}
		
		// Trace should no longer be active
		activeTraces := tracer.GetActiveTraces()
		for _, trace := range activeTraces {
			if trace.TraceID == traceCtx.TraceID {
				t.Error("Trace should not be active after ending")
			}
		}
	})
	
	t.Run("EndTraceWithError", func(t *testing.T) {
		ctx := context.Background()
		
		traceCtx, err := tracer.StartTrace(ctx, "failing_operation")
		if err != nil {
			t.Fatalf("Failed to start trace: %v", err)
		}
		
		testErr := &testError{message: "operation failed"}
		err = tracer.EndTrace(traceCtx, testErr)
		if err != nil {
			t.Errorf("Expected no error ending trace with error, got %v", err)
		}
		
		// Check trace data
		trace, err := tracer.GetTrace(traceCtx.TraceID)
		if err != nil {
			t.Fatalf("Failed to get trace: %v", err)
		}
		
		if trace.Success {
			t.Error("Expected trace to be marked as failed")
		}
		if trace.Error != "operation failed" {
			t.Errorf("Expected error 'operation failed', got '%s'", trace.Error)
		}
	})
}

func TestRequestTracer_GetTrace(t *testing.T) {
	logger := NewStructuredLogger(WithLevel(slog.LevelDebug))
	tracer := NewRequestTracer(logger)
	
	t.Run("GetExistingTrace", func(t *testing.T) {
		ctx := context.Background()
		
		traceCtx, err := tracer.StartTrace(ctx, "test_operation")
		if err != nil {
			t.Fatalf("Failed to start trace: %v", err)
		}
		
		trace, err := tracer.GetTrace(traceCtx.TraceID)
		if err != nil {
			t.Errorf("Expected no error getting trace, got %v", err)
		}
		if trace == nil {
			t.Fatal("Expected trace to be found")
		}
		
		if trace.TraceID != traceCtx.TraceID {
			t.Errorf("Expected trace ID '%s', got '%s'", traceCtx.TraceID, trace.TraceID)
		}
		if trace.Operation != "test_operation" {
			t.Errorf("Expected operation 'test_operation', got '%s'", trace.Operation)
		}
	})
	
	t.Run("GetNonExistentTrace", func(t *testing.T) {
		trace, err := tracer.GetTrace("non-existent-trace")
		if err == nil {
			t.Error("Expected error for non-existent trace")
		}
		if trace != nil {
			t.Error("Expected nil trace for non-existent ID")
		}
	})
}

func TestRequestTracer_ActiveTraces(t *testing.T) {
	logger := NewStructuredLogger(WithLevel(slog.LevelDebug))
	tracer := NewRequestTracer(logger)
	
	t.Run("GetActiveTraces", func(t *testing.T) {
		ctx := context.Background()
		
		// Start multiple traces
		trace1, _ := tracer.StartTrace(ctx, "operation1")
		trace2, _ := tracer.StartTrace(ctx, "operation2")
		trace3, _ := tracer.StartTrace(ctx, "operation3")
		
		activeTraces := tracer.GetActiveTraces()
		if len(activeTraces) != 3 {
			t.Errorf("Expected 3 active traces, got %d", len(activeTraces))
		}
		
		// End one trace
		tracer.EndTrace(trace2, nil)
		
		activeTraces = tracer.GetActiveTraces()
		if len(activeTraces) != 2 {
			t.Errorf("Expected 2 active traces after ending one, got %d", len(activeTraces))
		}
		
		// Verify the correct traces are still active
		activeIDs := make(map[string]bool)
		for _, trace := range activeTraces {
			activeIDs[trace.TraceID] = true
		}
		
		if !activeIDs[trace1.TraceID] {
			t.Error("Expected trace1 to still be active")
		}
		if activeIDs[trace2.TraceID] {
			t.Error("Expected trace2 to not be active")
		}
		if !activeIDs[trace3.TraceID] {
			t.Error("Expected trace3 to still be active")
		}
	})
}

func TestRequestTracer_WithLogContext(t *testing.T) {
	logger := NewStructuredLogger(WithLevel(slog.LevelDebug))
	tracer := NewRequestTracer(logger)
	
	t.Run("TraceWithLogContext", func(t *testing.T) {
		logCtx := &LogContext{
			Component: "bitswap",
			RequestID: "req-123",
			UserID:    "user-456",
			Fields: map[string]interface{}{
				"custom_field": "custom_value",
			},
		}
		
		ctx := WithLogContext(context.Background(), logCtx)
		
		traceCtx, err := tracer.StartTrace(ctx, "bitswap_request")
		if err != nil {
			t.Fatalf("Failed to start trace: %v", err)
		}
		
		if traceCtx.Labels["component"] != "bitswap" {
			t.Errorf("Expected component label 'bitswap', got '%s'", traceCtx.Labels["component"])
		}
		if traceCtx.Labels["request_id"] != "req-123" {
			t.Errorf("Expected request_id label 'req-123', got '%s'", traceCtx.Labels["request_id"])
		}
		if traceCtx.Labels["user_id"] != "user-456" {
			t.Errorf("Expected user_id label 'user-456', got '%s'", traceCtx.Labels["user_id"])
		}
		if traceCtx.Attributes["custom_field"] != "custom_value" {
			t.Errorf("Expected custom_field attribute 'custom_value', got %v", traceCtx.Attributes["custom_field"])
		}
	})
}

func TestEnhancedRequestTracer_SpanManagement(t *testing.T) {
	logger := NewStructuredLogger(WithLevel(slog.LevelDebug))
	tracer := NewRequestTracer(logger)
	enhancedTracer := tracer.(EnhancedRequestTracer)
	
	t.Run("AddSpan", func(t *testing.T) {
		ctx := context.Background()
		
		traceCtx, err := tracer.StartTrace(ctx, "parent_operation")
		if err != nil {
			t.Fatalf("Failed to start trace: %v", err)
		}
		
		spanID, err := enhancedTracer.AddSpan(traceCtx.TraceID, traceCtx.SpanID, "child_operation", map[string]interface{}{
			"param1": "value1",
		})
		if err != nil {
			t.Errorf("Expected no error adding span, got %v", err)
		}
		if spanID == "" {
			t.Error("Expected span ID to be generated")
		}
		
		// Verify span was added to trace
		trace, err := tracer.GetTrace(traceCtx.TraceID)
		if err != nil {
			t.Fatalf("Failed to get trace: %v", err)
		}
		
		if len(trace.Spans) != 2 { // Initial span + added span
			t.Errorf("Expected 2 spans, got %d", len(trace.Spans))
		}
		
		// Find the added span
		var addedSpan *SpanData
		for _, span := range trace.Spans {
			if span.SpanID == spanID {
				addedSpan = &span
				break
			}
		}
		
		if addedSpan == nil {
			t.Fatal("Added span not found in trace")
		}
		if addedSpan.Operation != "child_operation" {
			t.Errorf("Expected span operation 'child_operation', got '%s'", addedSpan.Operation)
		}
		if addedSpan.ParentID != traceCtx.SpanID {
			t.Errorf("Expected parent ID '%s', got '%s'", traceCtx.SpanID, addedSpan.ParentID)
		}
		if addedSpan.Attributes["param1"] != "value1" {
			t.Errorf("Expected param1 'value1', got %v", addedSpan.Attributes["param1"])
		}
	})
	
	t.Run("EndSpan", func(t *testing.T) {
		ctx := context.Background()
		
		traceCtx, err := tracer.StartTrace(ctx, "parent_operation")
		if err != nil {
			t.Fatalf("Failed to start trace: %v", err)
		}
		
		spanID, err := enhancedTracer.AddSpan(traceCtx.TraceID, traceCtx.SpanID, "child_operation", nil)
		if err != nil {
			t.Fatalf("Failed to add span: %v", err)
		}
		
		// Simulate some work
		time.Sleep(10 * time.Millisecond)
		
		err = enhancedTracer.EndSpan(traceCtx.TraceID, spanID, nil, map[string]interface{}{
			"result": "success",
		})
		if err != nil {
			t.Errorf("Expected no error ending span, got %v", err)
		}
		
		// Verify span was updated
		trace, err := tracer.GetTrace(traceCtx.TraceID)
		if err != nil {
			t.Fatalf("Failed to get trace: %v", err)
		}
		
		var endedSpan *SpanData
		for _, span := range trace.Spans {
			if span.SpanID == spanID {
				endedSpan = &span
				break
			}
		}
		
		if endedSpan == nil {
			t.Fatal("Ended span not found in trace")
		}
		if endedSpan.EndTime == nil {
			t.Error("Expected span end time to be set")
		}
		if endedSpan.Duration == 0 {
			t.Error("Expected span duration to be calculated")
		}
		if !endedSpan.Success {
			t.Error("Expected span to be marked as successful")
		}
		if endedSpan.Attributes["result"] != "success" {
			t.Errorf("Expected result attribute 'success', got %v", endedSpan.Attributes["result"])
		}
	})
	
	t.Run("EndSpanWithError", func(t *testing.T) {
		ctx := context.Background()
		
		traceCtx, err := tracer.StartTrace(ctx, "parent_operation")
		if err != nil {
			t.Fatalf("Failed to start trace: %v", err)
		}
		
		spanID, err := enhancedTracer.AddSpan(traceCtx.TraceID, traceCtx.SpanID, "failing_operation", nil)
		if err != nil {
			t.Fatalf("Failed to add span: %v", err)
		}
		
		testErr := &testError{message: "span failed"}
		err = enhancedTracer.EndSpan(traceCtx.TraceID, spanID, testErr, nil)
		if err != nil {
			t.Errorf("Expected no error ending span with error, got %v", err)
		}
		
		// Verify span error was recorded
		trace, err := tracer.GetTrace(traceCtx.TraceID)
		if err != nil {
			t.Fatalf("Failed to get trace: %v", err)
		}
		
		var failedSpan *SpanData
		for _, span := range trace.Spans {
			if span.SpanID == spanID {
				failedSpan = &span
				break
			}
		}
		
		if failedSpan == nil {
			t.Fatal("Failed span not found in trace")
		}
		if failedSpan.Success {
			t.Error("Expected span to be marked as failed")
		}
		if failedSpan.Error != "span failed" {
			t.Errorf("Expected error 'span failed', got '%s'", failedSpan.Error)
		}
	})
}

func TestEnhancedRequestTracer_QueryMethods(t *testing.T) {
	logger := NewStructuredLogger(WithLevel(slog.LevelDebug))
	tracer := NewRequestTracer(logger)
	enhancedTracer := tracer.(EnhancedRequestTracer)
	
	ctx := context.Background()
	
	// Create test traces
	trace1, _ := tracer.StartTrace(ctx, "fast_operation")
	time.Sleep(5 * time.Millisecond)
	tracer.EndTrace(trace1, nil)
	
	trace2, _ := tracer.StartTrace(ctx, "slow_operation")
	time.Sleep(50 * time.Millisecond)
	tracer.EndTrace(trace2, nil)
	
	trace3, _ := tracer.StartTrace(ctx, "failing_operation")
	time.Sleep(10 * time.Millisecond)
	tracer.EndTrace(trace3, &testError{message: "failed"})
	
	trace4, _ := tracer.StartTrace(ctx, "fast_operation")
	time.Sleep(3 * time.Millisecond)
	tracer.EndTrace(trace4, nil)
	
	t.Run("GetTracesByOperation", func(t *testing.T) {
		since := time.Now().Add(-1 * time.Minute)
		
		fastTraces := enhancedTracer.GetTracesByOperation("fast_operation", since)
		if len(fastTraces) != 2 {
			t.Errorf("Expected 2 fast_operation traces, got %d", len(fastTraces))
		}
		
		slowTraces := enhancedTracer.GetTracesByOperation("slow_operation", since)
		if len(slowTraces) != 1 {
			t.Errorf("Expected 1 slow_operation trace, got %d", len(slowTraces))
		}
		
		failingTraces := enhancedTracer.GetTracesByOperation("failing_operation", since)
		if len(failingTraces) != 1 {
			t.Errorf("Expected 1 failing_operation trace, got %d", len(failingTraces))
		}
	})
	
	t.Run("GetSlowTraces", func(t *testing.T) {
		since := time.Now().Add(-1 * time.Minute)
		threshold := 20 * time.Millisecond
		
		slowTraces := enhancedTracer.GetSlowTraces(threshold, since)
		if len(slowTraces) != 1 {
			t.Errorf("Expected 1 slow trace, got %d", len(slowTraces))
		}
		
		if len(slowTraces) > 0 && slowTraces[0].Operation != "slow_operation" {
			t.Errorf("Expected slow trace to be 'slow_operation', got '%s'", slowTraces[0].Operation)
		}
	})
	
	t.Run("GetFailedTraces", func(t *testing.T) {
		since := time.Now().Add(-1 * time.Minute)
		
		failedTraces := enhancedTracer.GetFailedTraces(since)
		if len(failedTraces) != 1 {
			t.Errorf("Expected 1 failed trace, got %d", len(failedTraces))
		}
		
		if len(failedTraces) > 0 {
			if failedTraces[0].Operation != "failing_operation" {
				t.Errorf("Expected failed trace to be 'failing_operation', got '%s'", failedTraces[0].Operation)
			}
			if failedTraces[0].Success {
				t.Error("Expected failed trace to be marked as unsuccessful")
			}
		}
	})
	
	t.Run("GetTraceStatistics", func(t *testing.T) {
		since := time.Now().Add(-1 * time.Minute)
		
		stats := enhancedTracer.GetTraceStatistics(since)
		
		if stats.TotalTraces != 4 {
			t.Errorf("Expected 4 total traces, got %d", stats.TotalTraces)
		}
		if stats.SuccessfulTraces != 3 {
			t.Errorf("Expected 3 successful traces, got %d", stats.SuccessfulTraces)
		}
		if stats.FailedTraces != 1 {
			t.Errorf("Expected 1 failed trace, got %d", stats.FailedTraces)
		}
		if stats.TracesByOperation["fast_operation"] != 2 {
			t.Errorf("Expected 2 fast_operation traces, got %d", stats.TracesByOperation["fast_operation"])
		}
		if stats.AverageDuration == 0 {
			t.Error("Expected average duration to be calculated")
		}
	})
}

func TestEnhancedRequestTracer_Cleanup(t *testing.T) {
	logger := NewStructuredLogger(WithLevel(slog.LevelDebug))
	tracer := NewRequestTracer(logger)
	enhancedTracer := tracer.(EnhancedRequestTracer)
	
	ctx := context.Background()
	
	t.Run("CleanupOldTraces", func(t *testing.T) {
		// Create some old traces
		oldTrace1, _ := tracer.StartTrace(ctx, "old_operation1")
		tracer.EndTrace(oldTrace1, nil)
		
		oldTrace2, _ := tracer.StartTrace(ctx, "old_operation2")
		tracer.EndTrace(oldTrace2, nil)
		
		// Wait a bit
		time.Sleep(10 * time.Millisecond)
		
		// Create a new trace
		newTrace, _ := tracer.StartTrace(ctx, "new_operation")
		tracer.EndTrace(newTrace, nil)
		
		// Cleanup traces older than 5ms ago
		cutoff := time.Now().Add(-5 * time.Millisecond)
		removedCount := enhancedTracer.CleanupOldTraces(cutoff)
		
		if removedCount != 2 {
			t.Errorf("Expected 2 traces to be removed, got %d", removedCount)
		}
		
		// Verify old traces are gone
		_, err := tracer.GetTrace(oldTrace1.TraceID)
		if err == nil {
			t.Error("Expected old trace1 to be removed")
		}
		
		_, err = tracer.GetTrace(oldTrace2.TraceID)
		if err == nil {
			t.Error("Expected old trace2 to be removed")
		}
		
		// Verify new trace is still there
		_, err = tracer.GetTrace(newTrace.TraceID)
		if err != nil {
			t.Errorf("Expected new trace to still exist, got error: %v", err)
		}
	})
}

func TestRequestTracer_MaxLimits(t *testing.T) {
	logger := NewStructuredLogger(WithLevel(slog.LevelDebug))
	tracer := NewRequestTracer(logger, WithMaxActive(2))
	
	ctx := context.Background()
	
	t.Run("MaxActiveTraces", func(t *testing.T) {
		// Start traces up to the limit
		trace1, err := tracer.StartTrace(ctx, "operation1")
		if err != nil {
			t.Fatalf("Failed to start trace1: %v", err)
		}
		
		trace2, err := tracer.StartTrace(ctx, "operation2")
		if err != nil {
			t.Fatalf("Failed to start trace2: %v", err)
		}
		
		// Try to start one more trace (should fail)
		_, err = tracer.StartTrace(ctx, "operation3")
		if err == nil {
			t.Error("Expected error when exceeding max active traces")
		}
		
		// End one trace and try again
		tracer.EndTrace(trace1, nil)
		
		trace3, err := tracer.StartTrace(ctx, "operation3")
		if err != nil {
			t.Errorf("Expected to be able to start trace after ending one, got error: %v", err)
		}
		
		// Clean up
		tracer.EndTrace(trace2, nil)
		tracer.EndTrace(trace3, nil)
	})
}

// Benchmark tests

func BenchmarkRequestTracer_StartEndTrace(b *testing.B) {
	logger := NewStructuredLogger(WithLevel(slog.LevelError)) // Reduce logging for benchmark
	tracer := NewRequestTracer(logger)
	
	ctx := context.Background()
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		traceCtx, err := tracer.StartTrace(ctx, "benchmark_operation")
		if err != nil {
			b.Fatal(err)
		}
		
		err = tracer.EndTrace(traceCtx, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRequestTracer_GetActiveTraces(b *testing.B) {
	logger := NewStructuredLogger(WithLevel(slog.LevelError))
	tracer := NewRequestTracer(logger)
	
	ctx := context.Background()
	
	// Start some traces
	for i := 0; i < 100; i++ {
		tracer.StartTrace(ctx, "benchmark_operation")
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		tracer.GetActiveTraces()
	}
}

func BenchmarkEnhancedRequestTracer_AddSpan(b *testing.B) {
	logger := NewStructuredLogger(WithLevel(slog.LevelError))
	tracer := NewRequestTracer(logger)
	enhancedTracer := tracer.(EnhancedRequestTracer)
	
	ctx := context.Background()
	traceCtx, _ := tracer.StartTrace(ctx, "parent_operation")
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		spanID, err := enhancedTracer.AddSpan(traceCtx.TraceID, traceCtx.SpanID, "child_operation", nil)
		if err != nil {
			b.Fatal(err)
		}
		
		err = enhancedTracer.EndSpan(traceCtx.TraceID, spanID, nil, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}