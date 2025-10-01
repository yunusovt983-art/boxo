package monitoring

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/smtp"
	"strings"
	"sync"
	"time"
)

// LogNotifier sends alerts to structured logs
type LogNotifier struct {
	logger StructuredLogger
	name   string
}

// NewLogNotifier creates a new log-based alert notifier
func NewLogNotifier(logger StructuredLogger) AlertNotifier {
	return &LogNotifier{
		logger: logger,
		name:   "log",
	}
}

func (ln *LogNotifier) SendAlert(ctx context.Context, alert Alert) error {
	level := slog.LevelInfo
	switch alert.Severity {
	case SeverityCritical:
		level = slog.LevelError
	case SeverityHigh:
		level = slog.LevelWarn
	case SeverityMedium:
		level = slog.LevelWarn
	case SeverityLow:
		level = slog.LevelInfo
	}
	
	ln.logger.LogWithContext(ctx, level, fmt.Sprintf("ALERT: %s", alert.Title),
		slog.String("alert_id", alert.ID),
		slog.String("rule_id", alert.RuleID),
		slog.String("component", alert.Component),
		slog.String("metric", alert.Metric),
		slog.String("severity", string(alert.Severity)),
		slog.String("description", alert.Description),
		slog.Any("value", alert.Value),
		slog.Any("threshold", alert.Threshold),
		slog.String("state", string(alert.State)),
		slog.Int("fire_count", alert.FireCount),
		slog.String("notification_type", "alert"),
	)
	
	return nil
}

func (ln *LogNotifier) SendResolution(ctx context.Context, alert Alert) error {
	ln.logger.InfoWithContext(ctx, fmt.Sprintf("RESOLVED: %s", alert.Title),
		slog.String("alert_id", alert.ID),
		slog.String("component", alert.Component),
		slog.String("resolved_by", alert.ResolvedBy),
		slog.String("notification_type", "resolution"),
	)
	
	return nil
}

func (ln *LogNotifier) GetName() string {
	return ln.name
}

func (ln *LogNotifier) IsHealthy() bool {
	return true // Log notifier is always healthy
}

// WebhookNotifier sends alerts to HTTP webhooks
type WebhookNotifier struct {
	name       string
	url        string
	headers    map[string]string
	timeout    time.Duration
	client     *http.Client
	logger     StructuredLogger
	mu         sync.RWMutex
	healthy    bool
	lastError  error
}

// WebhookConfig contains configuration for webhook notifier
type WebhookConfig struct {
	Name    string            `json:"name"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Timeout time.Duration     `json:"timeout"`
}

// NewWebhookNotifier creates a new webhook-based alert notifier
func NewWebhookNotifier(config WebhookConfig, logger StructuredLogger) AlertNotifier {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	
	return &WebhookNotifier{
		name:    config.Name,
		url:     config.URL,
		headers: config.Headers,
		timeout: config.Timeout,
		client: &http.Client{
			Timeout: config.Timeout,
		},
		logger:  logger,
		healthy: true,
	}
}

// WebhookPayload represents the payload sent to webhooks
type WebhookPayload struct {
	Type        string                 `json:"type"` // "alert" or "resolution"
	Alert       Alert                  `json:"alert"`
	Timestamp   time.Time              `json:"timestamp"`
	Source      string                 `json:"source"`
	Environment string                 `json:"environment,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

func (wn *WebhookNotifier) SendAlert(ctx context.Context, alert Alert) error {
	payload := WebhookPayload{
		Type:      "alert",
		Alert:     alert,
		Timestamp: time.Now(),
		Source:    "boxo-monitoring",
		Metadata: map[string]interface{}{
			"version": "1.0",
		},
	}
	
	return wn.sendWebhook(ctx, payload)
}

func (wn *WebhookNotifier) SendResolution(ctx context.Context, alert Alert) error {
	payload := WebhookPayload{
		Type:      "resolution",
		Alert:     alert,
		Timestamp: time.Now(),
		Source:    "boxo-monitoring",
		Metadata: map[string]interface{}{
			"version": "1.0",
		},
	}
	
	return wn.sendWebhook(ctx, payload)
}

func (wn *WebhookNotifier) sendWebhook(ctx context.Context, payload WebhookPayload) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		wn.setUnhealthy(err)
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}
	
	req, err := http.NewRequestWithContext(ctx, "POST", wn.url, bytes.NewBuffer(jsonData))
	if err != nil {
		wn.setUnhealthy(err)
		return fmt.Errorf("failed to create webhook request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "boxo-monitoring/1.0")
	
	// Add custom headers
	for key, value := range wn.headers {
		req.Header.Set(key, value)
	}
	
	resp, err := wn.client.Do(req)
	if err != nil {
		wn.setUnhealthy(err)
		if wn.logger != nil {
			wn.logger.ErrorWithContext(ctx, "Webhook request failed",
				slog.String("notifier", wn.name),
				slog.String("url", wn.url),
				slog.String("error", err.Error()),
			)
		}
		return fmt.Errorf("webhook request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		err := fmt.Errorf("webhook returned status %d", resp.StatusCode)
		wn.setUnhealthy(err)
		if wn.logger != nil {
			wn.logger.ErrorWithContext(ctx, "Webhook returned error status",
				slog.String("notifier", wn.name),
				slog.String("url", wn.url),
				slog.Int("status_code", resp.StatusCode),
			)
		}
		return err
	}
	
	wn.setHealthy()
	
	if wn.logger != nil {
		wn.logger.DebugWithContext(ctx, "Webhook notification sent successfully",
			slog.String("notifier", wn.name),
			slog.String("url", wn.url),
			slog.String("alert_id", payload.Alert.ID),
			slog.String("type", payload.Type),
		)
	}
	
	return nil
}

func (wn *WebhookNotifier) setHealthy() {
	wn.mu.Lock()
	defer wn.mu.Unlock()
	
	wn.healthy = true
	wn.lastError = nil
}

func (wn *WebhookNotifier) setUnhealthy(err error) {
	wn.mu.Lock()
	defer wn.mu.Unlock()
	
	wn.healthy = false
	wn.lastError = err
}

func (wn *WebhookNotifier) GetName() string {
	return wn.name
}

func (wn *WebhookNotifier) IsHealthy() bool {
	wn.mu.RLock()
	defer wn.mu.RUnlock()
	
	return wn.healthy
}

func (wn *WebhookNotifier) GetLastError() error {
	wn.mu.RLock()
	defer wn.mu.RUnlock()
	
	return wn.lastError
}

// EmailNotifier sends alerts via email
type EmailNotifier struct {
	name       string
	smtpHost   string
	smtpPort   int
	username   string
	password   string
	from       string
	to         []string
	logger     StructuredLogger
	mu         sync.RWMutex
	healthy    bool
	lastError  error
}

// EmailConfig contains configuration for email notifier
type EmailConfig struct {
	Name     string   `json:"name"`
	SMTPHost string   `json:"smtp_host"`
	SMTPPort int      `json:"smtp_port"`
	Username string   `json:"username"`
	Password string   `json:"password"`
	From     string   `json:"from"`
	To       []string `json:"to"`
}

// NewEmailNotifier creates a new email-based alert notifier
func NewEmailNotifier(config EmailConfig, logger StructuredLogger) AlertNotifier {
	return &EmailNotifier{
		name:     config.Name,
		smtpHost: config.SMTPHost,
		smtpPort: config.SMTPPort,
		username: config.Username,
		password: config.Password,
		from:     config.From,
		to:       config.To,
		logger:   logger,
		healthy:  true,
	}
}

func (en *EmailNotifier) SendAlert(ctx context.Context, alert Alert) error {
	subject := fmt.Sprintf("[%s] Alert: %s", strings.ToUpper(string(alert.Severity)), alert.Title)
	body := en.formatAlertEmail(alert)
	
	return en.sendEmail(ctx, subject, body)
}

func (en *EmailNotifier) SendResolution(ctx context.Context, alert Alert) error {
	subject := fmt.Sprintf("[RESOLVED] %s", alert.Title)
	body := en.formatResolutionEmail(alert)
	
	return en.sendEmail(ctx, subject, body)
}

func (en *EmailNotifier) formatAlertEmail(alert Alert) string {
	var buf strings.Builder
	
	buf.WriteString(fmt.Sprintf("Alert: %s\n", alert.Title))
	buf.WriteString(fmt.Sprintf("Severity: %s\n", alert.Severity))
	buf.WriteString(fmt.Sprintf("Component: %s\n", alert.Component))
	buf.WriteString(fmt.Sprintf("Metric: %s\n", alert.Metric))
	buf.WriteString(fmt.Sprintf("Description: %s\n", alert.Description))
	buf.WriteString(fmt.Sprintf("Value: %v\n", alert.Value))
	buf.WriteString(fmt.Sprintf("Threshold: %v\n", alert.Threshold))
	buf.WriteString(fmt.Sprintf("Fire Count: %d\n", alert.FireCount))
	buf.WriteString(fmt.Sprintf("Created At: %s\n", alert.CreatedAt.Format(time.RFC3339)))
	
	if alert.TraceID != "" {
		buf.WriteString(fmt.Sprintf("Trace ID: %s\n", alert.TraceID))
	}
	
	if len(alert.Labels) > 0 {
		buf.WriteString("\nLabels:\n")
		for k, v := range alert.Labels {
			buf.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
		}
	}
	
	if len(alert.Annotations) > 0 {
		buf.WriteString("\nAnnotations:\n")
		for k, v := range alert.Annotations {
			buf.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
		}
	}
	
	return buf.String()
}

func (en *EmailNotifier) formatResolutionEmail(alert Alert) string {
	var buf strings.Builder
	
	buf.WriteString(fmt.Sprintf("Alert Resolved: %s\n", alert.Title))
	buf.WriteString(fmt.Sprintf("Component: %s\n", alert.Component))
	buf.WriteString(fmt.Sprintf("Resolved By: %s\n", alert.ResolvedBy))
	
	if alert.ResolvedAt != nil {
		buf.WriteString(fmt.Sprintf("Resolved At: %s\n", alert.ResolvedAt.Format(time.RFC3339)))
		
		duration := alert.ResolvedAt.Sub(alert.CreatedAt)
		buf.WriteString(fmt.Sprintf("Duration: %s\n", duration))
	}
	
	return buf.String()
}

func (en *EmailNotifier) sendEmail(ctx context.Context, subject, body string) error {
	auth := smtp.PlainAuth("", en.username, en.password, en.smtpHost)
	
	msg := fmt.Sprintf("To: %s\r\nSubject: %s\r\n\r\n%s",
		strings.Join(en.to, ","), subject, body)
	
	addr := fmt.Sprintf("%s:%d", en.smtpHost, en.smtpPort)
	
	err := smtp.SendMail(addr, auth, en.from, en.to, []byte(msg))
	if err != nil {
		en.setUnhealthy(err)
		if en.logger != nil {
			en.logger.ErrorWithContext(ctx, "Failed to send email notification",
				slog.String("notifier", en.name),
				slog.String("smtp_host", en.smtpHost),
				slog.Int("smtp_port", en.smtpPort),
				slog.String("error", err.Error()),
			)
		}
		return fmt.Errorf("failed to send email: %w", err)
	}
	
	en.setHealthy()
	
	if en.logger != nil {
		en.logger.DebugWithContext(ctx, "Email notification sent successfully",
			slog.String("notifier", en.name),
			slog.String("subject", subject),
			slog.Any("recipients", en.to),
		)
	}
	
	return nil
}

func (en *EmailNotifier) setHealthy() {
	en.mu.Lock()
	defer en.mu.Unlock()
	
	en.healthy = true
	en.lastError = nil
}

func (en *EmailNotifier) setUnhealthy(err error) {
	en.mu.Lock()
	defer en.mu.Unlock()
	
	en.healthy = false
	en.lastError = err
}

func (en *EmailNotifier) GetName() string {
	return en.name
}

func (en *EmailNotifier) IsHealthy() bool {
	en.mu.RLock()
	defer en.mu.RUnlock()
	
	return en.healthy
}

func (en *EmailNotifier) GetLastError() error {
	en.mu.RLock()
	defer en.mu.RUnlock()
	
	return en.lastError
}

// SlackNotifier sends alerts to Slack webhooks
type SlackNotifier struct {
	name      string
	webhookURL string
	channel   string
	username  string
	iconEmoji string
	client    *http.Client
	logger    StructuredLogger
	mu        sync.RWMutex
	healthy   bool
	lastError error
}

// SlackConfig contains configuration for Slack notifier
type SlackConfig struct {
	Name       string        `json:"name"`
	WebhookURL string        `json:"webhook_url"`
	Channel    string        `json:"channel"`
	Username   string        `json:"username"`
	IconEmoji  string        `json:"icon_emoji"`
	Timeout    time.Duration `json:"timeout"`
}

// SlackMessage represents a Slack message payload
type SlackMessage struct {
	Channel     string            `json:"channel,omitempty"`
	Username    string            `json:"username,omitempty"`
	IconEmoji   string            `json:"icon_emoji,omitempty"`
	Text        string            `json:"text"`
	Attachments []SlackAttachment `json:"attachments,omitempty"`
}

// SlackAttachment represents a Slack message attachment
type SlackAttachment struct {
	Color     string       `json:"color,omitempty"`
	Title     string       `json:"title,omitempty"`
	Text      string       `json:"text,omitempty"`
	Fields    []SlackField `json:"fields,omitempty"`
	Timestamp int64        `json:"ts,omitempty"`
}

// SlackField represents a field in a Slack attachment
type SlackField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

// NewSlackNotifier creates a new Slack-based alert notifier
func NewSlackNotifier(config SlackConfig, logger StructuredLogger) AlertNotifier {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	
	if config.Username == "" {
		config.Username = "Boxo Monitor"
	}
	
	if config.IconEmoji == "" {
		config.IconEmoji = ":warning:"
	}
	
	return &SlackNotifier{
		name:       config.Name,
		webhookURL: config.WebhookURL,
		channel:    config.Channel,
		username:   config.Username,
		iconEmoji:  config.IconEmoji,
		client: &http.Client{
			Timeout: config.Timeout,
		},
		logger:  logger,
		healthy: true,
	}
}

func (sn *SlackNotifier) SendAlert(ctx context.Context, alert Alert) error {
	color := sn.getSeverityColor(alert.Severity)
	
	message := SlackMessage{
		Channel:   sn.channel,
		Username:  sn.username,
		IconEmoji: sn.iconEmoji,
		Text:      fmt.Sprintf("🚨 *Alert Fired*: %s", alert.Title),
		Attachments: []SlackAttachment{
			{
				Color: color,
				Title: alert.Title,
				Text:  alert.Description,
				Fields: []SlackField{
					{Title: "Severity", Value: string(alert.Severity), Short: true},
					{Title: "Component", Value: alert.Component, Short: true},
					{Title: "Metric", Value: alert.Metric, Short: true},
					{Title: "Fire Count", Value: fmt.Sprintf("%d", alert.FireCount), Short: true},
					{Title: "Value", Value: fmt.Sprintf("%v", alert.Value), Short: true},
					{Title: "Threshold", Value: fmt.Sprintf("%v", alert.Threshold), Short: true},
				},
				Timestamp: alert.CreatedAt.Unix(),
			},
		},
	}
	
	if alert.TraceID != "" {
		message.Attachments[0].Fields = append(message.Attachments[0].Fields,
			SlackField{Title: "Trace ID", Value: alert.TraceID, Short: false})
	}
	
	return sn.sendSlackMessage(ctx, message)
}

func (sn *SlackNotifier) SendResolution(ctx context.Context, alert Alert) error {
	message := SlackMessage{
		Channel:   sn.channel,
		Username:  sn.username,
		IconEmoji: ":white_check_mark:",
		Text:      fmt.Sprintf("✅ *Alert Resolved*: %s", alert.Title),
		Attachments: []SlackAttachment{
			{
				Color: "good",
				Title: alert.Title,
				Fields: []SlackField{
					{Title: "Component", Value: alert.Component, Short: true},
					{Title: "Resolved By", Value: alert.ResolvedBy, Short: true},
				},
			},
		},
	}
	
	if alert.ResolvedAt != nil {
		duration := alert.ResolvedAt.Sub(alert.CreatedAt)
		message.Attachments[0].Fields = append(message.Attachments[0].Fields,
			SlackField{Title: "Duration", Value: duration.String(), Short: true})
		message.Attachments[0].Timestamp = alert.ResolvedAt.Unix()
	}
	
	return sn.sendSlackMessage(ctx, message)
}

func (sn *SlackNotifier) getSeverityColor(severity Severity) string {
	switch severity {
	case SeverityCritical:
		return "danger"
	case SeverityHigh:
		return "warning"
	case SeverityMedium:
		return "#ff9900"
	case SeverityLow:
		return "#36a64f"
	default:
		return "#36a64f"
	}
}

func (sn *SlackNotifier) sendSlackMessage(ctx context.Context, message SlackMessage) error {
	jsonData, err := json.Marshal(message)
	if err != nil {
		sn.setUnhealthy(err)
		return fmt.Errorf("failed to marshal Slack message: %w", err)
	}
	
	req, err := http.NewRequestWithContext(ctx, "POST", sn.webhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		sn.setUnhealthy(err)
		return fmt.Errorf("failed to create Slack request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := sn.client.Do(req)
	if err != nil {
		sn.setUnhealthy(err)
		if sn.logger != nil {
			sn.logger.ErrorWithContext(ctx, "Slack notification failed",
				slog.String("notifier", sn.name),
				slog.String("error", err.Error()),
			)
		}
		return fmt.Errorf("Slack request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("Slack returned status %d", resp.StatusCode)
		sn.setUnhealthy(err)
		if sn.logger != nil {
			sn.logger.ErrorWithContext(ctx, "Slack returned error status",
				slog.String("notifier", sn.name),
				slog.Int("status_code", resp.StatusCode),
			)
		}
		return err
	}
	
	sn.setHealthy()
	
	if sn.logger != nil {
		sn.logger.DebugWithContext(ctx, "Slack notification sent successfully",
			slog.String("notifier", sn.name),
			slog.String("channel", sn.channel),
		)
	}
	
	return nil
}

func (sn *SlackNotifier) setHealthy() {
	sn.mu.Lock()
	defer sn.mu.Unlock()
	
	sn.healthy = true
	sn.lastError = nil
}

func (sn *SlackNotifier) setUnhealthy(err error) {
	sn.mu.Lock()
	defer sn.mu.Unlock()
	
	sn.healthy = false
	sn.lastError = err
}

func (sn *SlackNotifier) GetName() string {
	return sn.name
}

func (sn *SlackNotifier) IsHealthy() bool {
	sn.mu.RLock()
	defer sn.mu.RUnlock()
	
	return sn.healthy
}

func (sn *SlackNotifier) GetLastError() error {
	sn.mu.RLock()
	defer sn.mu.RUnlock()
	
	return sn.lastError
}