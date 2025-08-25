package main

import (
	"log/slog"
	"math"
	"strings"
	"sync"
	"time"
)

// RateLimiter manages rate limiting and backoff for API requests
type RateLimiter struct {
	mu              sync.RWMutex
	lastRequestTime time.Time
	requestCount    int
	resetTime       time.Time
	backoffUntil    time.Time
	failureCount    int
	maxRetries      int
	baseDelay       time.Duration
	maxDelay        time.Duration
}

// NewRateLimiter creates a new rate limiter with default settings
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		maxRetries: 3,
		baseDelay:  100 * time.Millisecond,
		maxDelay:   10 * time.Second,
	}
}

// Wait implements rate limiting and backoff logic
func (r *RateLimiter) Wait() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()

	// Check if we're in backoff period
	if now.Before(r.backoffUntil) {
		waitTime := r.backoffUntil.Sub(now)
		slog.Debug("rate limiter waiting", "duration", waitTime)
		time.Sleep(waitTime)
		return
	}

	// Simple rate limiting: ensure minimum time between requests
	minInterval := 50 * time.Millisecond // 20 requests per second max
	if elapsed := now.Sub(r.lastRequestTime); elapsed < minInterval {
		waitTime := minInterval - elapsed
		slog.Debug("rate limiting", "wait", waitTime)
		time.Sleep(waitTime)
	}

	r.lastRequestTime = time.Now()
}

// RecordSuccess resets failure counters on successful request
func (r *RateLimiter) RecordSuccess() {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.failureCount = 0
	r.backoffUntil = time.Time{}
}

// RecordFailure handles rate limit errors with exponential backoff
func (r *RateLimiter) RecordFailure(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.failureCount++

	// Check if this is a rate limit error
	if isRateLimitError(err) {
		// Calculate exponential backoff
		backoffDuration := r.calculateBackoff()
		r.backoffUntil = time.Now().Add(backoffDuration)
		
		slog.Warn("rate limit detected, backing off", 
			"duration", backoffDuration,
			"failures", r.failureCount,
			"until", r.backoffUntil.Format(time.RFC3339))
	}
}

// calculateBackoff returns the backoff duration using exponential backoff with jitter
func (r *RateLimiter) calculateBackoff() time.Duration {
	// Exponential backoff: baseDelay * 2^(failureCount-1)
	multiplier := math.Pow(2, float64(r.failureCount-1))
	backoff := time.Duration(float64(r.baseDelay) * multiplier)
	
	// Cap at maxDelay
	if backoff > r.maxDelay {
		backoff = r.maxDelay
	}
	
	// Add jitter (±25%)
	jitter := time.Duration(float64(backoff) * 0.25 * (0.5 - float64(time.Now().UnixNano()%100)/100))
	backoff += jitter
	
	return backoff
}

// ShouldRetry returns true if we should retry after a failure
func (r *RateLimiter) ShouldRetry() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	return r.failureCount < r.maxRetries
}

// isRateLimitError checks if an error is a rate limit error
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "rate limit") ||
		strings.Contains(errStr, "429") ||
		strings.Contains(errStr, "too many requests") ||
		strings.Contains(errStr, "quota exceeded")
}

// GlobalRateLimiter manages rate limiting across all models
type GlobalRateLimiter struct {
	mu         sync.RWMutex
	limiters   map[string]*RateLimiter
	globalWait time.Duration
	lastGlobal time.Time
}

// NewGlobalRateLimiter creates a new global rate limiter
func NewGlobalRateLimiter() *GlobalRateLimiter {
	return &GlobalRateLimiter{
		limiters:   make(map[string]*RateLimiter),
		globalWait: 50 * time.Millisecond, // Global minimum between any requests
	}
}

// GetLimiter returns a rate limiter for a specific model
func (g *GlobalRateLimiter) GetLimiter(model string) *RateLimiter {
	g.mu.Lock()
	defer g.mu.Unlock()
	
	if limiter, exists := g.limiters[model]; exists {
		return limiter
	}
	
	limiter := NewRateLimiter()
	g.limiters[model] = limiter
	return limiter
}

// WaitGlobal ensures global rate limiting across all models
func (g *GlobalRateLimiter) WaitGlobal() {
	g.mu.Lock()
	defer g.mu.Unlock()
	
	now := time.Now()
	if elapsed := now.Sub(g.lastGlobal); elapsed < g.globalWait {
		waitTime := g.globalWait - elapsed
		time.Sleep(waitTime)
	}
	g.lastGlobal = time.Now()
}

// RecordRateLimitHeaders updates rate limit info from response headers
func (g *GlobalRateLimiter) RecordRateLimitHeaders(headers map[string]string) {
	// Parse headers like:
	// X-RateLimit-Limit: 100
	// X-RateLimit-Remaining: 45
	// X-RateLimit-Reset: 1234567890
	
	if remaining, exists := headers["X-RateLimit-Remaining"]; exists {
		slog.Debug("rate limit status", "remaining", remaining)
		// Could implement more sophisticated logic based on remaining quota
	}
	
	if reset, exists := headers["X-RateLimit-Reset"]; exists {
		slog.Debug("rate limit reset", "time", reset)
		// Could pause until reset if quota exhausted
	}
}

// ParseErrorForRetryAfter extracts retry-after duration from error
func ParseErrorForRetryAfter(err error) time.Duration {
	if err == nil {
		return 0
	}
	
	errStr := err.Error()
	
	// Look for patterns like "retry after 5s" or "retry-after: 5"
	// This is a simplified implementation
	if strings.Contains(errStr, "retry after") || strings.Contains(errStr, "retry-after") {
		// Extract number and parse
		// For now, return a default
		return 5 * time.Second
	}
	
	return 0
}