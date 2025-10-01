package monitoring

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"
)

func TestSimpleAlertManager(t *testing.T) {
	// Create a simple logger with a proper writer
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	
	// Create mock components
	perfMonitor := newMockPerformanceMonitor()
	bottleneckAnalyzer := newMockBottleneckAnalyzer()
	requestTracer := newMockRequestTracer()
	
	// Create alert manager
	alertManager := NewAlertManager(perfMonitor, bottleneckAnalyzer, requestTracer, logger)
	
	t.Run("BasicCreation", func(t *testing.T) {
		if alertManager == nil {
			t.Fatal("AlertManager should not be nil")
		}
		
		// Test basic operations
		activeAlerts := alertManager.GetActiveAlerts()
		if len(activeAlerts) != 0 {
			t.Errorf("Expected 0 active alerts, got %d", len(activeAlerts))
		}
		
		stats := alertManager.GetAlertStatistics()
		if stats.TotalAlerts != 0 {
			t.Errorf("Expected 0 total alerts, got %d", stats.TotalAlerts)
		}
	})
	
	t.Run("RegisterNotifier", func(t *testing.T) {
		notifier := newMockAlertNotifier("test-notifier")
		err := alertManager.RegisterNotifier("test", notifier)
		if err != nil {
			t.Errorf("Expected no error registering notifier, got %v", err)
		}
	})
}

func TestSimpleStructuredLogger(t *testing.T) {
	mockOutput := newMockLogOutput("test")
	logger := NewStructuredLogger(
		WithLevel(slog.LevelDebug),
		WithOutput("test", mockOutput),
	)
	
	t.Run("BasicLogging", func(t *testing.T) {
		ctx := context.Background()
		
		logger.InfoWithContext(ctx, "Test message",
			slog.String("key", "value"),
		)
		
		entries := mockOutput.getEntries()
		if len(entries) != 1 {
			t.Fatalf("Expected 1 log entry, got %d", len(entries))
		}
		
		entry := entries[0]
		if entry.Message != "Test message" {
			t.Errorf("Expected message 'Test message', got '%s'", entry.Message)
		}
		if entry.Fields["key"] != "value" {
			t.Errorf("Expected field key='value', got %v", entry.Fields["key"])
		}
	})
}

func TestSimpleRequestTracer(t *testing.T) {
	logger := NewStructuredLogger(WithLevel(slog.LevelDebug))
	tracer := NewRequestTracer(logger)
	
	t.Run("BasicTracing", func(t *testing.T) {
		ctx := context.Background()
		
		traceCtx, err := tracer.StartTrace(ctx, "test_operation")
		if err != nil {
			t.Fatalf("Expected no error starting trace, got %v", err)
		}
		
		if traceCtx.TraceID == "" {
			t.Error("Expected trace ID to be generated")
		}
		
		// Simulate some work
		time.Sleep(10 * time.Millisecond)
		
		err = tracer.EndTrace(traceCtx, nil)
		if err != nil {
			t.Errorf("Expected no error ending trace, got %v", err)
		}
	})
}

func TestSimpleNotifiers(t *testing.T) {
	logger := NewStructuredLogger(WithLevel(slog.LevelDebug))
	
	t.Run("LogNotifier", func(t *testing.T) {
		notifier := NewLogNotifier(logger)
		
		if notifier.GetName() != "log" {
			t.Errorf("Expected name 'log', got '%s'", notifier.GetName())
		}
		
		if !notifier.IsHealthy() {
			t.Error("Expected log notifier to be healthy")
		}
		
		alert := Alert{
			ID:    "test-alert",
			Title: "Test Alert",
		}
		
		ctx := context.Background()
		err := notifier.SendAlert(ctx, alert)
		if err != nil {
			t.Errorf("Expected no error sending alert, got %v", err)
		}
	})
}