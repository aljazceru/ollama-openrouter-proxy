package main

import (
	"log/slog"
	"strings"
	"sync"
	"time"
)

// PermanentFailureTracker tracks models that should be skipped for the entire runtime
type PermanentFailureTracker struct {
	mu               sync.RWMutex
	permanentFailed  map[string]time.Time // Models that are permanently unavailable (404, etc)
	temporaryFailed  map[string]time.Time // Models that are temporarily unavailable
}

// NewPermanentFailureTracker creates a new tracker
func NewPermanentFailureTracker() *PermanentFailureTracker {
	return &PermanentFailureTracker{
		permanentFailed: make(map[string]time.Time),
		temporaryFailed: make(map[string]time.Time),
	}
}

// MarkPermanentFailure marks a model as permanently failed (404, model not found)
func (p *PermanentFailureTracker) MarkPermanentFailure(model string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.permanentFailed[model] = time.Now()
	slog.Warn("Model marked as permanently unavailable for this session", "model", model)
}

// MarkTemporaryFailure marks a model as temporarily failed
func (p *PermanentFailureTracker) MarkTemporaryFailure(model string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.temporaryFailed[model] = time.Now()
}

// IsPermanentlyFailed checks if a model is permanently failed
func (p *PermanentFailureTracker) IsPermanentlyFailed(model string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	_, exists := p.permanentFailed[model]
	return exists
}

// ShouldSkip checks if a model should be skipped (either permanent or recent temporary failure)
func (p *PermanentFailureTracker) ShouldSkip(model string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	// Check permanent failures
	if _, exists := p.permanentFailed[model]; exists {
		return true
	}
	
	// Check temporary failures (5 minute cooldown)
	if failTime, exists := p.temporaryFailed[model]; exists {
		if time.Since(failTime) < 5*time.Minute {
			return true
		}
	}
	
	return false
}

// ClearTemporaryFailure removes a model from temporary failures on success
func (p *PermanentFailureTracker) ClearTemporaryFailure(model string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.temporaryFailed, model)
}

// GetStats returns statistics about failures
func (p *PermanentFailureTracker) GetStats() (permanent int, temporary int) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	permanent = len(p.permanentFailed)
	
	// Count non-expired temporary failures
	now := time.Now()
	for _, failTime := range p.temporaryFailed {
		if now.Sub(failTime) < 5*time.Minute {
			temporary++
		}
	}
	
	return permanent, temporary
}

// isPermanentError determines if an error indicates a permanent failure
func isPermanentError(err error) bool {
	if err == nil {
		return false
	}
	
	errStr := strings.ToLower(err.Error())
	
	// 404 errors mean the model doesn't exist
	if strings.Contains(errStr, "404") || strings.Contains(errStr, "not found") {
		return true
	}
	
	// Model endpoint errors
	if strings.Contains(errStr, "no endpoints found") {
		return true
	}
	
	// Model not available errors
	if strings.Contains(errStr, "model not available") || strings.Contains(errStr, "model does not exist") {
		return true
	}
	
	return false
}

// isTemporaryError determines if an error is temporary (rate limits, timeouts, etc)
func isTemporaryError(err error) bool {
	if err == nil {
		return false
	}
	
	errStr := strings.ToLower(err.Error())
	
	// Rate limit errors
	if strings.Contains(errStr, "429") || strings.Contains(errStr, "rate limit") || strings.Contains(errStr, "too many requests") {
		return true
	}
	
	// Timeout errors
	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded") {
		return true
	}
	
	// Temporary server errors
	if strings.Contains(errStr, "503") || strings.Contains(errStr, "service unavailable") {
		return true
	}
	
	// Connection errors
	if strings.Contains(errStr, "connection refused") || strings.Contains(errStr, "connection reset") {
		return true
	}
	
	return true // Default to temporary for unknown errors
}