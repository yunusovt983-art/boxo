package monitoring

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLogNotifier(t *testing.T) {
	mockOutput := newMockLogOutput("test")
	logger := NewStructuredLogger(
		WithLevel(slog.LevelDebug),
		WithOutput("test", mockOutput),
	)
	
	notifier := NewLogNotifier(logger)
	
	t.Run("SendAlert", func(t *testing.T) {
		mockOutput.clear()
		
		alert := Alert{
			ID:          "test-alert-1",
			RuleID:      "rule-1",
			Title:       "Test Alert",
			Description: "This is a test alert",
			Severity:    SeverityHigh,
			Component:   "bitswap",
			Metric:      "response_time",
			Value:       150 * time.Millisecond,
			Threshold:   100 * time.Millisecond,
			State:       AlertStateFiring,
			FireCount:   1,
			CreatedAt:   time.Now(),
		}
		
		ctx := context.Background()
		err := notifier.SendAlert(ctx, alert)
		if err != nil {
			t.Errorf("Expected no error sending alert, got %v", err)
		}
		
		entries := mockOutput.getEntries()
		if len(entries) != 1 {
			t.Fatalf("Expected 1 log entry, got %d", len(entries))
		}
		
		entry := entries[0]
		if !strings.Contains(entry.Message, "ALERT: Test Alert") {
			t.Errorf("Expected alert message in log, got '%s'", entry.Message)
		}
		if entry.Fields["alert_id"] != "test-alert-1" {
			t.Errorf("Expected alert_id 'test-alert-1', got %v", entry.Fields["alert_id"])
		}
		if entry.Fields["severity"] != string(SeverityHigh) {
			t.Errorf("Expected severity 'high', got %v", entry.Fields["severity"])
		}
	})
	
	t.Run("SendResolution", func(t *testing.T) {
		mockOutput.clear()
		
		alert := Alert{
			ID:         "test-alert-1",
			Title:      "Test Alert",
			Component:  "bitswap",
			ResolvedBy: "test-user",
			State:      AlertStateResolved,
		}
		
		ctx := context.Background()
		err := notifier.SendResolution(ctx, alert)
		if err != nil {
			t.Errorf("Expected no error sending resolution, got %v", err)
		}
		
		entries := mockOutput.getEntries()
		if len(entries) != 1 {
			t.Fatalf("Expected 1 log entry, got %d", len(entries))
		}
		
		entry := entries[0]
		if !strings.Contains(entry.Message, "RESOLVED: Test Alert") {
			t.Errorf("Expected resolution message in log, got '%s'", entry.Message)
		}
		if entry.Fields["resolved_by"] != "test-user" {
			t.Errorf("Expected resolved_by 'test-user', got %v", entry.Fields["resolved_by"])
		}
	})
	
	t.Run("GetName", func(t *testing.T) {
		name := notifier.GetName()
		if name != "log" {
			t.Errorf("Expected name 'log', got '%s'", name)
		}
	})
	
	t.Run("IsHealthy", func(t *testing.T) {
		healthy := notifier.IsHealthy()
		if !healthy {
			t.Error("Expected log notifier to always be healthy")
		}
	})
}

func TestWebhookNotifier(t *testing.T) {
	// Create a test server
	var receivedPayloads []WebhookPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload WebhookPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		receivedPayloads = append(receivedPayloads, payload)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	
	logger := NewStructuredLogger(WithLevel(slog.LevelDebug))
	config := WebhookConfig{
		Name: "test-webhook",
		URL:  server.URL,
		Headers: map[string]string{
			"Authorization": "Bearer test-token",
		},
		Timeout: 5 * time.Second,
	}
	
	notifier := NewWebhookNotifier(config, logger)
	
	t.Run("SendAlert", func(t *testing.T) {
		receivedPayloads = nil
		
		alert := Alert{
			ID:          "webhook-alert-1",
			RuleID:      "rule-1",
			Title:       "Webhook Test Alert",
			Description: "This is a webhook test alert",
			Severity:    SeverityCritical,
			Component:   "network",
			Metric:      "latency",
			Value:       500 * time.Millisecond,
			Threshold:   200 * time.Millisecond,
			State:       AlertStateFiring,
			FireCount:   2,
			CreatedAt:   time.Now(),
		}
		
		ctx := context.Background()
		err := notifier.SendAlert(ctx, alert)
		if err != nil {
			t.Errorf("Expected no error sending webhook alert, got %v", err)
		}
		
		if len(receivedPayloads) != 1 {
			t.Fatalf("Expected 1 webhook payload, got %d", len(receivedPayloads))
		}
		
		payload := receivedPayloads[0]
		if payload.Type != "alert" {
			t.Errorf("Expected payload type 'alert', got '%s'", payload.Type)
		}
		if payload.Alert.ID != "webhook-alert-1" {
			t.Errorf("Expected alert ID 'webhook-alert-1', got '%s'", payload.Alert.ID)
		}
		if payload.Source != "boxo-monitoring" {
			t.Errorf("Expected source 'boxo-monitoring', got '%s'", payload.Source)
		}
	})
	
	t.Run("SendResolution", func(t *testing.T) {
		receivedPayloads = nil
		
		alert := Alert{
			ID:         "webhook-alert-1",
			Title:      "Webhook Test Alert",
			Component:  "network",
			ResolvedBy: "webhook-user",
			State:      AlertStateResolved,
		}
		
		ctx := context.Background()
		err := notifier.SendResolution(ctx, alert)
		if err != nil {
			t.Errorf("Expected no error sending webhook resolution, got %v", err)
		}
		
		if len(receivedPayloads) != 1 {
			t.Fatalf("Expected 1 webhook payload, got %d", len(receivedPayloads))
		}
		
		payload := receivedPayloads[0]
		if payload.Type != "resolution" {
			t.Errorf("Expected payload type 'resolution', got '%s'", payload.Type)
		}
		if payload.Alert.ResolvedBy != "webhook-user" {
			t.Errorf("Expected resolved_by 'webhook-user', got '%s'", payload.Alert.ResolvedBy)
		}
	})
	
	t.Run("GetName", func(t *testing.T) {
		name := notifier.GetName()
		if name != "test-webhook" {
			t.Errorf("Expected name 'test-webhook', got '%s'", name)
		}
	})
	
	t.Run("IsHealthy", func(t *testing.T) {
		healthy := notifier.IsHealthy()
		if !healthy {
			t.Error("Expected webhook notifier to be healthy after successful requests")
		}
	})
}

func TestWebhookNotifier_ErrorHandling(t *testing.T) {
	logger := NewStructuredLogger(WithLevel(slog.LevelDebug))
	
	t.Run("ServerError", func(t *testing.T) {
		// Create a server that returns errors
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}))
		defer server.Close()
		
		config := WebhookConfig{
			Name:    "error-webhook",
			URL:     server.URL,
			Timeout: 1 * time.Second,
		}
		
		notifier := NewWebhookNotifier(config, logger)
		
		alert := Alert{
			ID:    "error-alert",
			Title: "Error Test Alert",
		}
		
		ctx := context.Background()
		err := notifier.SendAlert(ctx, alert)
		if err == nil {
			t.Error("Expected error when server returns 500")
		}
		
		// Notifier should be marked as unhealthy
		if notifier.IsHealthy() {
			t.Error("Expected notifier to be unhealthy after server error")
		}
	})
	
	t.Run("NetworkError", func(t *testing.T) {
		config := WebhookConfig{
			Name:    "network-error-webhook",
			URL:     "http://non-existent-server:12345/webhook",
			Timeout: 1 * time.Second,
		}
		
		notifier := NewWebhookNotifier(config, logger)
		
		alert := Alert{
			ID:    "network-error-alert",
			Title: "Network Error Test Alert",
		}
		
		ctx := context.Background()
		err := notifier.SendAlert(ctx, alert)
		if err == nil {
			t.Error("Expected error when server is unreachable")
		}
		
		// Notifier should be marked as unhealthy
		if notifier.IsHealthy() {
			t.Error("Expected notifier to be unhealthy after network error")
		}
	})
	
	t.Run("Timeout", func(t *testing.T) {
		// Create a server that delays response
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(2 * time.Second)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()
		
		config := WebhookConfig{
			Name:    "timeout-webhook",
			URL:     server.URL,
			Timeout: 100 * time.Millisecond, // Short timeout
		}
		
		notifier := NewWebhookNotifier(config, logger)
		
		alert := Alert{
			ID:    "timeout-alert",
			Title: "Timeout Test Alert",
		}
		
		ctx := context.Background()
		err := notifier.SendAlert(ctx, alert)
		if err == nil {
			t.Error("Expected timeout error")
		}
		
		// Notifier should be marked as unhealthy
		if notifier.IsHealthy() {
			t.Error("Expected notifier to be unhealthy after timeout")
		}
	})
}

func TestEmailNotifier(t *testing.T) {
	// Note: This test doesn't actually send emails, it just tests the structure
	logger := NewStructuredLogger(WithLevel(slog.LevelDebug))
	
	config := EmailConfig{
		Name:     "test-email",
		SMTPHost: "smtp.example.com",
		SMTPPort: 587,
		Username: "test@example.com",
		Password: "password",
		From:     "alerts@example.com",
		To:       []string{"admin@example.com", "ops@example.com"},
	}
	
	notifier := NewEmailNotifier(config, logger)
	
	t.Run("GetName", func(t *testing.T) {
		name := notifier.GetName()
		if name != "test-email" {
			t.Errorf("Expected name 'test-email', got '%s'", name)
		}
	})
	
	t.Run("IsHealthy", func(t *testing.T) {
		healthy := notifier.IsHealthy()
		if !healthy {
			t.Error("Expected email notifier to be healthy initially")
		}
	})
	
	t.Run("FormatAlertEmail", func(t *testing.T) {
		emailNotifier := notifier.(*EmailNotifier)
		
		alert := Alert{
			ID:          "email-alert-1",
			Title:       "Email Test Alert",
			Description: "This is an email test alert",
			Severity:    SeverityMedium,
			Component:   "blockstore",
			Metric:      "cache_hit_rate",
			Value:       0.65,
			Threshold:   0.80,
			FireCount:   1,
			CreatedAt:   time.Now(),
			TraceID:     "trace-123",
			Labels: map[string]string{
				"environment": "production",
			},
			Annotations: map[string]string{
				"runbook": "https://docs.example.com/runbooks/cache",
			},
		}
		
		body := emailNotifier.formatAlertEmail(alert)
		
		if !strings.Contains(body, "Email Test Alert") {
			t.Error("Expected alert title in email body")
		}
		if !strings.Contains(body, "medium") {
			t.Error("Expected severity in email body")
		}
		if !strings.Contains(body, "blockstore") {
			t.Error("Expected component in email body")
		}
		if !strings.Contains(body, "trace-123") {
			t.Error("Expected trace ID in email body")
		}
		if !strings.Contains(body, "environment: production") {
			t.Error("Expected labels in email body")
		}
		if !strings.Contains(body, "runbook: https://docs.example.com/runbooks/cache") {
			t.Error("Expected annotations in email body")
		}
	})
	
	t.Run("FormatResolutionEmail", func(t *testing.T) {
		emailNotifier := notifier.(*EmailNotifier)
		
		resolvedTime := time.Now()
		alert := Alert{
			ID:         "email-alert-1",
			Title:      "Email Test Alert",
			Component:  "blockstore",
			ResolvedBy: "email-user",
			ResolvedAt: &resolvedTime,
			CreatedAt:  resolvedTime.Add(-30 * time.Minute),
		}
		
		body := emailNotifier.formatResolutionEmail(alert)
		
		if !strings.Contains(body, "Alert Resolved: Email Test Alert") {
			t.Error("Expected resolution title in email body")
		}
		if !strings.Contains(body, "email-user") {
			t.Error("Expected resolved by in email body")
		}
		if !strings.Contains(body, "Duration:") {
			t.Error("Expected duration in email body")
		}
	})
}

func TestSlackNotifier(t *testing.T) {
	// Create a test server to receive Slack webhooks
	var receivedMessages []SlackMessage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var message SlackMessage
		if err := json.NewDecoder(r.Body).Decode(&message); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		receivedMessages = append(receivedMessages, message)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	
	logger := NewStructuredLogger(WithLevel(slog.LevelDebug))
	config := SlackConfig{
		Name:       "test-slack",
		WebhookURL: server.URL,
		Channel:    "#alerts",
		Username:   "Boxo Monitor",
		IconEmoji:  ":warning:",
		Timeout:    5 * time.Second,
	}
	
	notifier := NewSlackNotifier(config, logger)
	
	t.Run("SendAlert", func(t *testing.T) {
		receivedMessages = nil
		
		alert := Alert{
			ID:          "slack-alert-1",
			RuleID:      "rule-1",
			Title:       "Slack Test Alert",
			Description: "This is a Slack test alert",
			Severity:    SeverityHigh,
			Component:   "bitswap",
			Metric:      "queue_depth",
			Value:       int64(1500),
			Threshold:   int64(1000),
			State:       AlertStateFiring,
			FireCount:   3,
			CreatedAt:   time.Now(),
			TraceID:     "trace-456",
		}
		
		ctx := context.Background()
		err := notifier.SendAlert(ctx, alert)
		if err != nil {
			t.Errorf("Expected no error sending Slack alert, got %v", err)
		}
		
		if len(receivedMessages) != 1 {
			t.Fatalf("Expected 1 Slack message, got %d", len(receivedMessages))
		}
		
		message := receivedMessages[0]
		if message.Channel != "#alerts" {
			t.Errorf("Expected channel '#alerts', got '%s'", message.Channel)
		}
		if message.Username != "Boxo Monitor" {
			t.Errorf("Expected username 'Boxo Monitor', got '%s'", message.Username)
		}
		if !strings.Contains(message.Text, "Alert Fired") {
			t.Errorf("Expected 'Alert Fired' in message text, got '%s'", message.Text)
		}
		
		if len(message.Attachments) != 1 {
			t.Fatalf("Expected 1 attachment, got %d", len(message.Attachments))
		}
		
		attachment := message.Attachments[0]
		if attachment.Color != "warning" { // High severity maps to warning color
			t.Errorf("Expected color 'warning', got '%s'", attachment.Color)
		}
		if attachment.Title != "Slack Test Alert" {
			t.Errorf("Expected title 'Slack Test Alert', got '%s'", attachment.Title)
		}
		
		// Check fields
		fieldMap := make(map[string]string)
		for _, field := range attachment.Fields {
			fieldMap[field.Title] = field.Value
		}
		
		if fieldMap["Severity"] != "high" {
			t.Errorf("Expected severity field 'high', got '%s'", fieldMap["Severity"])
		}
		if fieldMap["Component"] != "bitswap" {
			t.Errorf("Expected component field 'bitswap', got '%s'", fieldMap["Component"])
		}
		if fieldMap["Trace ID"] != "trace-456" {
			t.Errorf("Expected trace ID field 'trace-456', got '%s'", fieldMap["Trace ID"])
		}
	})
	
	t.Run("SendResolution", func(t *testing.T) {
		receivedMessages = nil
		
		resolvedTime := time.Now()
		alert := Alert{
			ID:         "slack-alert-1",
			Title:      "Slack Test Alert",
			Component:  "bitswap",
			ResolvedBy: "slack-user",
			ResolvedAt: &resolvedTime,
			CreatedAt:  resolvedTime.Add(-15 * time.Minute),
		}
		
		ctx := context.Background()
		err := notifier.SendResolution(ctx, alert)
		if err != nil {
			t.Errorf("Expected no error sending Slack resolution, got %v", err)
		}
		
		if len(receivedMessages) != 1 {
			t.Fatalf("Expected 1 Slack message, got %d", len(receivedMessages))
		}
		
		message := receivedMessages[0]
		if !strings.Contains(message.Text, "Alert Resolved") {
			t.Errorf("Expected 'Alert Resolved' in message text, got '%s'", message.Text)
		}
		if message.IconEmoji != ":white_check_mark:" {
			t.Errorf("Expected resolution icon ':white_check_mark:', got '%s'", message.IconEmoji)
		}
		
		if len(message.Attachments) != 1 {
			t.Fatalf("Expected 1 attachment, got %d", len(message.Attachments))
		}
		
		attachment := message.Attachments[0]
		if attachment.Color != "good" {
			t.Errorf("Expected color 'good' for resolution, got '%s'", attachment.Color)
		}
		
		// Check fields
		fieldMap := make(map[string]string)
		for _, field := range attachment.Fields {
			fieldMap[field.Title] = field.Value
		}
		
		if fieldMap["Resolved By"] != "slack-user" {
			t.Errorf("Expected resolved by field 'slack-user', got '%s'", fieldMap["Resolved By"])
		}
		if fieldMap["Duration"] == "" {
			t.Error("Expected duration field to be present")
		}
	})
	
	t.Run("GetSeverityColor", func(t *testing.T) {
		slackNotifier := notifier.(*SlackNotifier)
		
		if color := slackNotifier.getSeverityColor(SeverityCritical); color != "danger" {
			t.Errorf("Expected critical severity color 'danger', got '%s'", color)
		}
		if color := slackNotifier.getSeverityColor(SeverityHigh); color != "warning" {
			t.Errorf("Expected high severity color 'warning', got '%s'", color)
		}
		if color := slackNotifier.getSeverityColor(SeverityMedium); color != "#ff9900" {
			t.Errorf("Expected medium severity color '#ff9900', got '%s'", color)
		}
		if color := slackNotifier.getSeverityColor(SeverityLow); color != "#36a64f" {
			t.Errorf("Expected low severity color '#36a64f', got '%s'", color)
		}
	})
	
	t.Run("GetName", func(t *testing.T) {
		name := notifier.GetName()
		if name != "test-slack" {
			t.Errorf("Expected name 'test-slack', got '%s'", name)
		}
	})
	
	t.Run("IsHealthy", func(t *testing.T) {
		healthy := notifier.IsHealthy()
		if !healthy {
			t.Error("Expected Slack notifier to be healthy after successful requests")
		}
	})
}

func TestSlackNotifier_ErrorHandling(t *testing.T) {
	logger := NewStructuredLogger(WithLevel(slog.LevelDebug))
	
	t.Run("SlackAPIError", func(t *testing.T) {
		// Create a server that returns Slack API errors
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "channel_not_found", http.StatusNotFound)
		}))
		defer server.Close()
		
		config := SlackConfig{
			Name:       "error-slack",
			WebhookURL: server.URL,
			Timeout:    1 * time.Second,
		}
		
		notifier := NewSlackNotifier(config, logger)
		
		alert := Alert{
			ID:    "slack-error-alert",
			Title: "Slack Error Test Alert",
		}
		
		ctx := context.Background()
		err := notifier.SendAlert(ctx, alert)
		if err == nil {
			t.Error("Expected error when Slack API returns 404")
		}
		
		// Notifier should be marked as unhealthy
		if notifier.IsHealthy() {
			t.Error("Expected notifier to be unhealthy after Slack API error")
		}
	})
}

// Benchmark tests

func BenchmarkLogNotifier_SendAlert(b *testing.B) {
	mockOutput := newMockLogOutput("test")
	logger := NewStructuredLogger(
		WithLevel(slog.LevelInfo),
		WithOutput("test", mockOutput),
	)
	
	notifier := NewLogNotifier(logger)
	
	alert := Alert{
		ID:          "benchmark-alert",
		Title:       "Benchmark Alert",
		Description: "This is a benchmark alert",
		Severity:    SeverityMedium,
		Component:   "test",
		Metric:      "test_metric",
		Value:       100,
		Threshold:   80,
		State:       AlertStateFiring,
		FireCount:   1,
		CreatedAt:   time.Now(),
	}
	
	ctx := context.Background()
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		notifier.SendAlert(ctx, alert)
	}
}

func BenchmarkWebhookNotifier_SendAlert(b *testing.B) {
	// Create a test server that responds quickly
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	
	logger := NewStructuredLogger(WithLevel(slog.LevelError)) // Reduce logging for benchmark
	config := WebhookConfig{
		Name:    "benchmark-webhook",
		URL:     server.URL,
		Timeout: 5 * time.Second,
	}
	
	notifier := NewWebhookNotifier(config, logger)
	
	alert := Alert{
		ID:          "benchmark-alert",
		Title:       "Benchmark Alert",
		Description: "This is a benchmark alert",
		Severity:    SeverityMedium,
		Component:   "test",
		Metric:      "test_metric",
		Value:       100,
		Threshold:   80,
		State:       AlertStateFiring,
		FireCount:   1,
		CreatedAt:   time.Now(),
	}
	
	ctx := context.Background()
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		err := notifier.SendAlert(ctx, alert)
		if err != nil {
			b.Fatal(err)
		}
	}
}