package monitoring

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// requestTracer implements the RequestTracer interface
type requestTracer struct {
	mu           sync.RWMutex
	activeTraces map[string]*TraceData
	traceHistory []TraceData
	maxHistory   int
	maxActive    int
	logger       StructuredLogger
}

// NewRequestTracer creates a new request tracer instance
func NewRequestTracer(logger StructuredLogger, options ...TracerOption) RequestTracer {
	rt := &requestTracer{
		activeTraces: make(map[string]*TraceData),
		traceHistory: make([]TraceData, 0),
		maxHistory:   10000,
		maxActive:    1000,
		logger:       logger,
	}
	
	// Apply options
	for _, option := range options {
		option(rt)
	}
	
	return rt
}

// TracerOption defines configuration options for the tracer
type TracerOption func(*requestTracer)

// WithMaxHistory sets the maximum number of traces to keep in history
func WithMaxHistory(maxHistory int) TracerOption {
	return func(rt *requestTracer) {
		rt.maxHistory = maxHistory
	}
}

// WithMaxActive sets the maximum number of active traces
func WithMaxActive(maxActive int) TracerOption {
	return func(rt *requestTracer) {
		rt.maxActive = maxActive
	}
}

// StartTrace starts a new trace for an operation
func (rt *requestTracer) StartTrace(ctx context.Context, operation string) (TraceContext, error) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	
	// Check if we've reached the maximum number of active traces
	if len(rt.activeTraces) >= rt.maxActive {
		return TraceContext{}, fmt.Errorf("maximum number of active traces reached (%d)", rt.maxActive)
	}
	
	traceID := rt.generateTraceID()
	spanID := rt.generateSpanID()
	
	traceCtx := TraceContext{
		TraceID:   traceID,
		SpanID:    spanID,
		Operation: operation,
		StartTime: time.Now(),
		Labels:    make(map[string]string),
		Attributes: make(map[string]interface{}),
	}
	
	// Extract context information
	if logCtx := GetLogContext(ctx); logCtx != nil {
		traceCtx.Labels["component"] = logCtx.Component
		traceCtx.Labels["request_id"] = logCtx.RequestID
		traceCtx.Labels["user_id"] = logCtx.UserID
		
		// Copy relevant fields as attributes
		for k, v := range logCtx.Fields {
			traceCtx.Attributes[k] = v
		}
	}
	
	// Create trace data
	traceData := &TraceData{
		TraceID:   traceID,
		Operation: operation,
		StartTime: time.Now(),
		Success:   false, // Will be updated when trace ends
		Spans:     make([]SpanData, 0),
		Labels:    copyStringMap(traceCtx.Labels),
		Attributes: copyInterfaceMap(traceCtx.Attributes),
	}
	
	// Add initial span
	initialSpan := SpanData{
		SpanID:     spanID,
		Operation:  operation,
		StartTime:  time.Now(),
		Success:    false,
		Attributes: make(map[string]interface{}),
	}
	
	traceData.Spans = append(traceData.Spans, initialSpan)
	rt.activeTraces[traceID] = traceData
	
	if rt.logger != nil {
		rt.logger.DebugWithContext(ctx, "Trace started",
			slog.String("trace_id", traceID),
			slog.String("span_id", spanID),
			slog.String("operation", operation),
			slog.String("type", "trace_start"),
		)
	}
	
	return traceCtx, nil
}

// EndTrace ends a trace
func (rt *requestTracer) EndTrace(traceCtx TraceContext, err error) error {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	
	traceData, exists := rt.activeTraces[traceCtx.TraceID]
	if !exists {
		return fmt.Errorf("trace %s not found", traceCtx.TraceID)
	}
	
	now := time.Now()
	traceData.EndTime = &now
	traceData.Duration = now.Sub(traceData.StartTime)
	traceData.Success = err == nil
	
	if err != nil {
		traceData.Error = err.Error()
	}
	
	// Update the corresponding span
	for i := range traceData.Spans {
		if traceData.Spans[i].SpanID == traceCtx.SpanID {
			traceData.Spans[i].EndTime = &now
			traceData.Spans[i].Duration = now.Sub(traceData.Spans[i].StartTime)
			traceData.Spans[i].Success = err == nil
			if err != nil {
				traceData.Spans[i].Error = err.Error()
			}
			break
		}
	}
	
	// Move to history
	rt.addToHistory(*traceData)
	delete(rt.activeTraces, traceCtx.TraceID)
	
	if rt.logger != nil {
		logLevel := slog.LevelDebug
		if err != nil {
			logLevel = slog.LevelError
		}
		
		rt.logger.LogWithContext(context.Background(), logLevel, "Trace ended",
			slog.String("trace_id", traceCtx.TraceID),
			slog.String("span_id", traceCtx.SpanID),
			slog.String("operation", traceCtx.Operation),
			slog.Duration("duration", traceData.Duration),
			slog.Bool("success", traceData.Success),
			slog.String("type", "trace_end"),
		)
	}
	
	return nil
}

// GetTrace retrieves a specific trace by ID
func (rt *requestTracer) GetTrace(traceID string) (*TraceData, error) {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	
	// Check active traces first
	if traceData, exists := rt.activeTraces[traceID]; exists {
		traceCopy := *traceData
		return &traceCopy, nil
	}
	
	// Check history
	for _, traceData := range rt.traceHistory {
		if traceData.TraceID == traceID {
			traceCopy := traceData
			return &traceCopy, nil
		}
	}
	
	return nil, fmt.Errorf("trace %s not found", traceID)
}

// GetActiveTraces returns all currently active traces
func (rt *requestTracer) GetActiveTraces() []TraceData {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	
	traces := make([]TraceData, 0, len(rt.activeTraces))
	for _, traceData := range rt.activeTraces {
		traceCopy := *traceData
		traces = append(traces, traceCopy)
	}
	
	return traces
}

// AddSpan adds a new span to an existing trace
func (rt *requestTracer) AddSpan(traceID, parentSpanID, operation string, attributes map[string]interface{}) (string, error) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	
	traceData, exists := rt.activeTraces[traceID]
	if !exists {
		return "", fmt.Errorf("trace %s not found", traceID)
	}
	
	spanID := rt.generateSpanID()
	
	span := SpanData{
		SpanID:     spanID,
		ParentID:   parentSpanID,
		Operation:  operation,
		StartTime:  time.Now(),
		Success:    false,
		Attributes: copyInterfaceMap(attributes),
	}
	
	traceData.Spans = append(traceData.Spans, span)
	
	if rt.logger != nil {
		rt.logger.DebugWithContext(context.Background(), "Span added",
			slog.String("trace_id", traceID),
			slog.String("span_id", spanID),
			slog.String("parent_span_id", parentSpanID),
			slog.String("operation", operation),
			slog.String("type", "span_start"),
		)
	}
	
	return spanID, nil
}

// EndSpan ends a specific span within a trace
func (rt *requestTracer) EndSpan(traceID, spanID string, err error, attributes map[string]interface{}) error {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	
	traceData, exists := rt.activeTraces[traceID]
	if !exists {
		return fmt.Errorf("trace %s not found", traceID)
	}
	
	// Find and update the span
	for i := range traceData.Spans {
		if traceData.Spans[i].SpanID == spanID {
			now := time.Now()
			traceData.Spans[i].EndTime = &now
			traceData.Spans[i].Duration = now.Sub(traceData.Spans[i].StartTime)
			traceData.Spans[i].Success = err == nil
			
			if err != nil {
				traceData.Spans[i].Error = err.Error()
			}
			
			// Add additional attributes
			for k, v := range attributes {
				traceData.Spans[i].Attributes[k] = v
			}
			
			if rt.logger != nil {
				logLevel := slog.LevelDebug
				if err != nil {
					logLevel = slog.LevelError
				}
				
				rt.logger.LogWithContext(context.Background(), logLevel, "Span ended",
					slog.String("trace_id", traceID),
					slog.String("span_id", spanID),
					slog.Duration("duration", traceData.Spans[i].Duration),
					slog.Bool("success", traceData.Spans[i].Success),
					slog.String("type", "span_end"),
				)
			}
			
			return nil
		}
	}
	
	return fmt.Errorf("span %s not found in trace %s", spanID, traceID)
}

// GetTracesByOperation returns traces for a specific operation
func (rt *requestTracer) GetTracesByOperation(operation string, since time.Time) []TraceData {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	
	var traces []TraceData
	
	// Check active traces
	for _, traceData := range rt.activeTraces {
		if traceData.Operation == operation && traceData.StartTime.After(since) {
			traceCopy := *traceData
			traces = append(traces, traceCopy)
		}
	}
	
	// Check history
	for _, traceData := range rt.traceHistory {
		if traceData.Operation == operation && traceData.StartTime.After(since) {
			traces = append(traces, traceData)
		}
	}
	
	return traces
}

// GetSlowTraces returns traces that exceeded the specified duration threshold
func (rt *requestTracer) GetSlowTraces(threshold time.Duration, since time.Time) []TraceData {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	
	var slowTraces []TraceData
	
	// Check active traces (calculate current duration)
	now := time.Now()
	for _, traceData := range rt.activeTraces {
		if traceData.StartTime.After(since) {
			currentDuration := now.Sub(traceData.StartTime)
			if currentDuration > threshold {
				traceCopy := *traceData
				traceCopy.Duration = currentDuration
				slowTraces = append(slowTraces, traceCopy)
			}
		}
	}
	
	// Check history
	for _, traceData := range rt.traceHistory {
		if traceData.StartTime.After(since) && traceData.Duration > threshold {
			slowTraces = append(slowTraces, traceData)
		}
	}
	
	return slowTraces
}

// GetFailedTraces returns traces that ended with errors
func (rt *requestTracer) GetFailedTraces(since time.Time) []TraceData {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	
	var failedTraces []TraceData
	
	// Check history (active traces haven't ended yet)
	for _, traceData := range rt.traceHistory {
		if traceData.StartTime.After(since) && !traceData.Success {
			failedTraces = append(failedTraces, traceData)
		}
	}
	
	return failedTraces
}

// GetTraceStatistics returns statistics about traces
func (rt *requestTracer) GetTraceStatistics(since time.Time) TraceStatistics {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	
	stats := TraceStatistics{
		ActiveTraces:      len(rt.activeTraces),
		TotalTraces:       0,
		SuccessfulTraces:  0,
		FailedTraces:      0,
		AverageDuration:   0,
		TracesByOperation: make(map[string]int),
		LastUpdated:       time.Now(),
	}
	
	var totalDuration time.Duration
	
	// Count history traces
	for _, traceData := range rt.traceHistory {
		if traceData.StartTime.After(since) {
			stats.TotalTraces++
			totalDuration += traceData.Duration
			
			if traceData.Success {
				stats.SuccessfulTraces++
			} else {
				stats.FailedTraces++
			}
			
			stats.TracesByOperation[traceData.Operation]++
		}
	}
	
	// Count active traces
	now := time.Now()
	for _, traceData := range rt.activeTraces {
		if traceData.StartTime.After(since) {
			stats.TotalTraces++
			currentDuration := now.Sub(traceData.StartTime)
			totalDuration += currentDuration
			
			stats.TracesByOperation[traceData.Operation]++
		}
	}
	
	if stats.TotalTraces > 0 {
		stats.AverageDuration = totalDuration / time.Duration(stats.TotalTraces)
	}
	
	return stats
}

// TraceStatistics contains statistics about traces
type TraceStatistics struct {
	ActiveTraces      int                `json:"active_traces"`
	TotalTraces       int                `json:"total_traces"`
	SuccessfulTraces  int                `json:"successful_traces"`
	FailedTraces      int                `json:"failed_traces"`
	AverageDuration   time.Duration      `json:"average_duration"`
	TracesByOperation map[string]int     `json:"traces_by_operation"`
	LastUpdated       time.Time          `json:"last_updated"`
}

// CleanupOldTraces removes old traces from history
func (rt *requestTracer) CleanupOldTraces(olderThan time.Time) int {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	
	var newHistory []TraceData
	removedCount := 0
	
	for _, traceData := range rt.traceHistory {
		if traceData.StartTime.After(olderThan) {
			newHistory = append(newHistory, traceData)
		} else {
			removedCount++
		}
	}
	
	rt.traceHistory = newHistory
	
	if rt.logger != nil && removedCount > 0 {
		rt.logger.InfoWithContext(context.Background(), "Old traces cleaned up",
			slog.Int("removed_count", removedCount),
			slog.Time("older_than", olderThan),
			slog.String("type", "trace_cleanup"),
		)
	}
	
	return removedCount
}

// addToHistory adds a trace to the history
func (rt *requestTracer) addToHistory(traceData TraceData) {
	rt.traceHistory = append(rt.traceHistory, traceData)
	
	if len(rt.traceHistory) > rt.maxHistory {
		rt.traceHistory = rt.traceHistory[len(rt.traceHistory)-rt.maxHistory:]
	}
}

// generateTraceID generates a unique trace ID
func (rt *requestTracer) generateTraceID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// generateSpanID generates a unique span ID
func (rt *requestTracer) generateSpanID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// copyStringMap creates a copy of a string map
func copyStringMap(original map[string]string) map[string]string {
	if original == nil {
		return nil
	}
	
	copy := make(map[string]string, len(original))
	for k, v := range original {
		copy[k] = v
	}
	return copy
}

// copyInterfaceMap creates a copy of an interface map
func copyInterfaceMap(original map[string]interface{}) map[string]interface{} {
	if original == nil {
		return make(map[string]interface{})
	}
	
	copy := make(map[string]interface{}, len(original))
	for k, v := range original {
		copy[k] = v
	}
	return copy
}

// Enhanced RequestTracer interface with additional methods
type EnhancedRequestTracer interface {
	RequestTracer
	
	// Span management
	AddSpan(traceID, parentSpanID, operation string, attributes map[string]interface{}) (string, error)
	EndSpan(traceID, spanID string, err error, attributes map[string]interface{}) error
	
	// Query methods
	GetTracesByOperation(operation string, since time.Time) []TraceData
	GetSlowTraces(threshold time.Duration, since time.Time) []TraceData
	GetFailedTraces(since time.Time) []TraceData
	GetTraceStatistics(since time.Time) TraceStatistics
	
	// Maintenance
	CleanupOldTraces(olderThan time.Time) int
}

// Ensure requestTracer implements EnhancedRequestTracer
var _ EnhancedRequestTracer = (*requestTracer)(nil)