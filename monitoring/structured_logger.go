package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"sync"
	"time"
)

// StructuredLogger defines interface for structured logging with context
type StructuredLogger interface {
	// Context-aware logging methods
	LogWithContext(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) error
	InfoWithContext(ctx context.Context, msg string, attrs ...slog.Attr)
	WarnWithContext(ctx context.Context, msg string, attrs ...slog.Attr)
	ErrorWithContext(ctx context.Context, msg string, attrs ...slog.Attr)
	DebugWithContext(ctx context.Context, msg string, attrs ...slog.Attr)
	
	// Performance logging
	LogPerformanceMetrics(ctx context.Context, component string, metrics map[string]interface{})
	LogSlowOperation(ctx context.Context, operation string, duration time.Duration, threshold time.Duration)
	LogBottleneck(ctx context.Context, bottleneck Bottleneck)
	LogAlert(ctx context.Context, alert Alert)
	
	// Request tracing
	LogRequestStart(ctx context.Context, operation string, params map[string]interface{}) context.Context
	LogRequestEnd(ctx context.Context, operation string, duration time.Duration, err error)
	
	// Configuration
	SetLevel(level slog.Level)
	SetOutput(output LogOutput)
	AddHook(hook LogHook) error
	RemoveHook(hookName string) error
	
	// Utility methods
	WithFields(fields map[string]interface{}) StructuredLogger
	WithComponent(component string) StructuredLogger
	Flush() error
}

// LogOutput defines different output destinations for logs
type LogOutput interface {
	Write(entry LogEntry) error
	Flush() error
	Close() error
	GetName() string
}

// LogHook defines interface for log processing hooks
type LogHook interface {
	Process(entry LogEntry) (LogEntry, error)
	GetName() string
	ShouldProcess(level slog.Level) bool
}

// LogEntry represents a structured log entry
type LogEntry struct {
	Timestamp   time.Time              `json:"timestamp"`
	Level       slog.Level             `json:"level"`
	Message     string                 `json:"message"`
	Component   string                 `json:"component,omitempty"`
	Operation   string                 `json:"operation,omitempty"`
	TraceID     string                 `json:"trace_id,omitempty"`
	SpanID      string                 `json:"span_id,omitempty"`
	RequestID   string                 `json:"request_id,omitempty"`
	UserID      string                 `json:"user_id,omitempty"`
	Fields      map[string]interface{} `json:"fields,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Stack       string                 `json:"stack,omitempty"`
	Duration    *time.Duration         `json:"duration,omitempty"`
	Caller      string                 `json:"caller,omitempty"`
	Hostname    string                 `json:"hostname,omitempty"`
	PID         int                    `json:"pid,omitempty"`
	Goroutine   int64                  `json:"goroutine,omitempty"`
}

// LogContext contains contextual information for logging
type LogContext struct {
	TraceID     string                 `json:"trace_id,omitempty"`
	SpanID      string                 `json:"span_id,omitempty"`
	RequestID   string                 `json:"request_id,omitempty"`
	UserID      string                 `json:"user_id,omitempty"`
	Component   string                 `json:"component,omitempty"`
	Operation   string                 `json:"operation,omitempty"`
	StartTime   time.Time              `json:"start_time,omitempty"`
	Fields      map[string]interface{} `json:"fields,omitempty"`
}

// Context keys for logging context
type contextKey string

const (
	logContextKey contextKey = "log_context"
	traceIDKey    contextKey = "trace_id"
	spanIDKey     contextKey = "span_id"
	requestIDKey  contextKey = "request_id"
	userIDKey     contextKey = "user_id"
	componentKey  contextKey = "component"
	operationKey  contextKey = "operation"
)

// structuredLogger implements the StructuredLogger interface
type structuredLogger struct {
	mu        sync.RWMutex
	logger    *slog.Logger
	level     slog.Level
	outputs   map[string]LogOutput
	hooks     map[string]LogHook
	fields    map[string]interface{}
	component string
	hostname  string
	pid       int
}

// NewStructuredLogger creates a new structured logger instance
func NewStructuredLogger(options ...LoggerOption) StructuredLogger {
	hostname, _ := os.Hostname()
	
	sl := &structuredLogger{
		level:     slog.LevelInfo,
		outputs:   make(map[string]LogOutput),
		hooks:     make(map[string]LogHook),
		fields:    make(map[string]interface{}),
		hostname:  hostname,
		pid:       os.Getpid(),
	}
	
	// Apply options
	for _, option := range options {
		option(sl)
	}
	
	// Create default slog logger if not set
	if sl.logger == nil {
		handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: sl.level,
		})
		sl.logger = slog.New(handler)
	}
	
	// Add default console output if no outputs configured
	if len(sl.outputs) == 0 {
		sl.outputs["console"] = NewConsoleOutput()
	}
	
	return sl
}

// LoggerOption defines configuration options for the logger
type LoggerOption func(*structuredLogger)

// WithLevel sets the logging level
func WithLevel(level slog.Level) LoggerOption {
	return func(sl *structuredLogger) {
		sl.level = level
	}
}

// WithComponent sets the default component name
func WithComponent(component string) LoggerOption {
	return func(sl *structuredLogger) {
		sl.component = component
	}
}

// WithFields sets default fields
func WithFields(fields map[string]interface{}) LoggerOption {
	return func(sl *structuredLogger) {
		sl.fields = fields
	}
}

// WithOutput adds a log output
func WithOutput(name string, output LogOutput) LoggerOption {
	return func(sl *structuredLogger) {
		sl.outputs[name] = output
	}
}

// WithHook adds a log hook
func WithHook(hook LogHook) LoggerOption {
	return func(sl *structuredLogger) {
		sl.hooks[hook.GetName()] = hook
	}
}

// LogWithContext logs a message with context
func (sl *structuredLogger) LogWithContext(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) error {
	if level < sl.level {
		return nil
	}
	
	entry := sl.createLogEntry(ctx, level, msg, attrs...)
	
	// Process hooks
	for _, hook := range sl.hooks {
		if hook.ShouldProcess(level) {
			processedEntry, err := hook.Process(entry)
			if err != nil {
				// Log hook error but continue
				sl.logger.Error("Log hook processing failed",
					slog.String("hook", hook.GetName()),
					slog.String("error", err.Error()),
				)
			} else {
				entry = processedEntry
			}
		}
	}
	
	// Write to outputs
	for name, output := range sl.outputs {
		if err := output.Write(entry); err != nil {
			// Log output error but continue
			sl.logger.Error("Log output write failed",
				slog.String("output", name),
				slog.String("error", err.Error()),
			)
		}
	}
	
	return nil
}

// createLogEntry creates a log entry from context and attributes
func (sl *structuredLogger) createLogEntry(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) LogEntry {
	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   msg,
		Component: sl.component,
		Fields:    make(map[string]interface{}),
		Hostname:  sl.hostname,
		PID:       sl.pid,
		Goroutine: getGoroutineID(),
		Caller:    getCaller(3),
	}
	
	// Add default fields
	for k, v := range sl.fields {
		entry.Fields[k] = v
	}
	
	// Extract context information
	if logCtx := GetLogContext(ctx); logCtx != nil {
		entry.TraceID = logCtx.TraceID
		entry.SpanID = logCtx.SpanID
		entry.RequestID = logCtx.RequestID
		entry.UserID = logCtx.UserID
		
		if entry.Component == "" && logCtx.Component != "" {
			entry.Component = logCtx.Component
		}
		
		if logCtx.Operation != "" {
			entry.Operation = logCtx.Operation
		}
		
		// Add context fields
		for k, v := range logCtx.Fields {
			entry.Fields[k] = v
		}
		
		// Calculate duration if start time is available
		if !logCtx.StartTime.IsZero() {
			duration := time.Since(logCtx.StartTime)
			entry.Duration = &duration
		}
	}
	
	// Process attributes
	for _, attr := range attrs {
		switch attr.Key {
		case "error":
			if err, ok := attr.Value.Any().(error); ok {
				entry.Error = err.Error()
				entry.Stack = getStackTrace()
			} else {
				entry.Error = attr.Value.String()
			}
		case "duration":
			if duration, ok := attr.Value.Any().(time.Duration); ok {
				entry.Duration = &duration
			}
		case "component":
			entry.Component = attr.Value.String()
		case "operation":
			entry.Operation = attr.Value.String()
		default:
			entry.Fields[attr.Key] = attr.Value.Any()
		}
	}
	
	return entry
}

// InfoWithContext logs an info message with context
func (sl *structuredLogger) InfoWithContext(ctx context.Context, msg string, attrs ...slog.Attr) {
	sl.LogWithContext(ctx, slog.LevelInfo, msg, attrs...)
}

// WarnWithContext logs a warning message with context
func (sl *structuredLogger) WarnWithContext(ctx context.Context, msg string, attrs ...slog.Attr) {
	sl.LogWithContext(ctx, slog.LevelWarn, msg, attrs...)
}

// ErrorWithContext logs an error message with context
func (sl *structuredLogger) ErrorWithContext(ctx context.Context, msg string, attrs ...slog.Attr) {
	sl.LogWithContext(ctx, slog.LevelError, msg, attrs...)
}

// DebugWithContext logs a debug message with context
func (sl *structuredLogger) DebugWithContext(ctx context.Context, msg string, attrs ...slog.Attr) {
	sl.LogWithContext(ctx, slog.LevelDebug, msg, attrs...)
}

// LogPerformanceMetrics logs performance metrics
func (sl *structuredLogger) LogPerformanceMetrics(ctx context.Context, component string, metrics map[string]interface{}) {
	attrs := []slog.Attr{
		slog.String("component", component),
		slog.String("type", "performance_metrics"),
	}
	
	for k, v := range metrics {
		attrs = append(attrs, slog.Any(k, v))
	}
	
	sl.InfoWithContext(ctx, "Performance metrics collected", attrs...)
}

// LogSlowOperation logs slow operations
func (sl *structuredLogger) LogSlowOperation(ctx context.Context, operation string, duration time.Duration, threshold time.Duration) {
	sl.WarnWithContext(ctx, "Slow operation detected",
		slog.String("operation", operation),
		slog.Duration("duration", duration),
		slog.Duration("threshold", threshold),
		slog.Float64("slowness_ratio", float64(duration)/float64(threshold)),
		slog.String("type", "slow_operation"),
	)
}

// LogBottleneck logs detected bottlenecks
func (sl *structuredLogger) LogBottleneck(ctx context.Context, bottleneck Bottleneck) {
	attrs := []slog.Attr{
		slog.String("bottleneck_id", bottleneck.ID),
		slog.String("component", bottleneck.Component),
		slog.String("bottleneck_type", string(bottleneck.Type)),
		slog.String("severity", string(bottleneck.Severity)),
		slog.String("description", bottleneck.Description),
		slog.Time("detected_at", bottleneck.DetectedAt),
		slog.String("type", "bottleneck"),
	}
	
	for k, v := range bottleneck.Metrics {
		attrs = append(attrs, slog.Any(fmt.Sprintf("metric_%s", k), v))
	}
	
	sl.WarnWithContext(ctx, "Performance bottleneck detected", attrs...)
}

// LogAlert logs alerts
func (sl *structuredLogger) LogAlert(ctx context.Context, alert Alert) {
	attrs := []slog.Attr{
		slog.String("alert_id", alert.ID),
		slog.String("rule_id", alert.RuleID),
		slog.String("component", alert.Component),
		slog.String("metric", alert.Metric),
		slog.String("severity", string(alert.Severity)),
		slog.String("state", string(alert.State)),
		slog.Any("value", alert.Value),
		slog.Any("threshold", alert.Threshold),
		slog.Int("fire_count", alert.FireCount),
		slog.Time("created_at", alert.CreatedAt),
		slog.String("type", "alert"),
	}
	
	if alert.TraceID != "" {
		attrs = append(attrs, slog.String("trace_id", alert.TraceID))
	}
	
	for k, v := range alert.Labels {
		attrs = append(attrs, slog.String(fmt.Sprintf("label_%s", k), v))
	}
	
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
	
	sl.LogWithContext(ctx, level, fmt.Sprintf("Alert: %s", alert.Title), attrs...)
}

// LogRequestStart logs the start of a request and returns context with logging info
func (sl *structuredLogger) LogRequestStart(ctx context.Context, operation string, params map[string]interface{}) context.Context {
	requestID := generateRequestID()
	traceID := getTraceIDFromContext(ctx)
	if traceID == "" {
		traceID = generateTraceID()
	}
	
	logCtx := &LogContext{
		TraceID:   traceID,
		RequestID: requestID,
		Operation: operation,
		StartTime: time.Now(),
		Fields:    make(map[string]interface{}),
	}
	
	// Add parameters to context fields
	for k, v := range params {
		logCtx.Fields[k] = v
	}
	
	// Add existing context fields
	if existingCtx := GetLogContext(ctx); existingCtx != nil {
		logCtx.SpanID = existingCtx.SpanID
		logCtx.UserID = existingCtx.UserID
		logCtx.Component = existingCtx.Component
		
		for k, v := range existingCtx.Fields {
			if _, exists := logCtx.Fields[k]; !exists {
				logCtx.Fields[k] = v
			}
		}
	}
	
	newCtx := WithLogContext(ctx, logCtx)
	
	sl.DebugWithContext(newCtx, "Request started",
		slog.String("operation", operation),
		slog.String("request_id", requestID),
		slog.String("trace_id", traceID),
		slog.String("type", "request_start"),
	)
	
	return newCtx
}

// LogRequestEnd logs the end of a request
func (sl *structuredLogger) LogRequestEnd(ctx context.Context, operation string, duration time.Duration, err error) {
	attrs := []slog.Attr{
		slog.String("operation", operation),
		slog.Duration("duration", duration),
		slog.String("type", "request_end"),
	}
	
	if err != nil {
		attrs = append(attrs, slog.Any("error", err))
		sl.ErrorWithContext(ctx, "Request completed with error", attrs...)
	} else {
		sl.DebugWithContext(ctx, "Request completed successfully", attrs...)
	}
}

// SetLevel sets the logging level
func (sl *structuredLogger) SetLevel(level slog.Level) {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	
	sl.level = level
}

// SetOutput sets a log output
func (sl *structuredLogger) SetOutput(output LogOutput) {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	
	sl.outputs[output.GetName()] = output
}

// AddHook adds a log hook
func (sl *structuredLogger) AddHook(hook LogHook) error {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	
	sl.hooks[hook.GetName()] = hook
	return nil
}

// RemoveHook removes a log hook
func (sl *structuredLogger) RemoveHook(hookName string) error {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	
	delete(sl.hooks, hookName)
	return nil
}

// WithFields returns a new logger with additional fields
func (sl *structuredLogger) WithFields(fields map[string]interface{}) StructuredLogger {
	newFields := make(map[string]interface{})
	
	// Copy existing fields
	for k, v := range sl.fields {
		newFields[k] = v
	}
	
	// Add new fields
	for k, v := range fields {
		newFields[k] = v
	}
	
	return &structuredLogger{
		logger:    sl.logger,
		level:     sl.level,
		outputs:   sl.outputs,
		hooks:     sl.hooks,
		fields:    newFields,
		component: sl.component,
		hostname:  sl.hostname,
		pid:       sl.pid,
	}
}

// WithComponent returns a new logger with a specific component
func (sl *structuredLogger) WithComponent(component string) StructuredLogger {
	return &structuredLogger{
		logger:    sl.logger,
		level:     sl.level,
		outputs:   sl.outputs,
		hooks:     sl.hooks,
		fields:    sl.fields,
		component: component,
		hostname:  sl.hostname,
		pid:       sl.pid,
	}
}

// Flush flushes all outputs
func (sl *structuredLogger) Flush() error {
	sl.mu.RLock()
	defer sl.mu.RUnlock()
	
	for _, output := range sl.outputs {
		if err := output.Flush(); err != nil {
			return err
		}
	}
	
	return nil
}

// Context helper functions

// WithLogContext adds logging context to a context
func WithLogContext(ctx context.Context, logCtx *LogContext) context.Context {
	return context.WithValue(ctx, logContextKey, logCtx)
}

// GetLogContext retrieves logging context from a context
func GetLogContext(ctx context.Context) *LogContext {
	if logCtx, ok := ctx.Value(logContextKey).(*LogContext); ok {
		return logCtx
	}
	return nil
}

// WithTraceID adds a trace ID to context
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey, traceID)
}

// WithRequestID adds a request ID to context
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// WithUserID adds a user ID to context
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// Utility functions

// generateRequestID generates a unique request ID
func generateRequestID() string {
	return fmt.Sprintf("req_%d_%d", time.Now().UnixNano(), getGoroutineID())
}

// generateTraceID generates a unique trace ID
func generateTraceID() string {
	return fmt.Sprintf("trace_%d_%d", time.Now().UnixNano(), getGoroutineID())
}

// getTraceIDFromContext extracts trace ID from context
func getTraceIDFromContext(ctx context.Context) string {
	if traceID, ok := ctx.Value(traceIDKey).(string); ok {
		return traceID
	}
	
	if logCtx := GetLogContext(ctx); logCtx != nil {
		return logCtx.TraceID
	}
	
	return ""
}

// getGoroutineID gets the current goroutine ID
func getGoroutineID() int64 {
	// This is a simplified implementation
	// In production, you might want to use a more robust method
	return int64(runtime.NumGoroutine())
}

// getCaller gets the caller information
func getCaller(skip int) string {
	_, file, line, ok := runtime.Caller(skip)
	if !ok {
		return "unknown"
	}
	
	// Extract just the filename
	for i := len(file) - 1; i > 0; i-- {
		if file[i] == '/' {
			file = file[i+1:]
			break
		}
	}
	
	return fmt.Sprintf("%s:%d", file, line)
}

// getStackTrace gets the current stack trace
func getStackTrace() string {
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}

// Console output implementation
type consoleOutput struct {
	encoder *json.Encoder
}

// NewConsoleOutput creates a new console output
func NewConsoleOutput() LogOutput {
	return &consoleOutput{
		encoder: json.NewEncoder(os.Stdout),
	}
}

func (co *consoleOutput) Write(entry LogEntry) error {
	return co.encoder.Encode(entry)
}

func (co *consoleOutput) Flush() error {
	return nil // stdout doesn't need flushing
}

func (co *consoleOutput) Close() error {
	return nil // stdout doesn't need closing
}

func (co *consoleOutput) GetName() string {
	return "console"
}