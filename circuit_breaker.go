package libai

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// CircuitBreakerState represents the current state of a circuit breaker
type CircuitBreakerState int

const (
	CircuitBreakerClosed CircuitBreakerState = iota
	CircuitBreakerOpen
	CircuitBreakerHalfOpen
)

// String returns the string representation of the circuit breaker state
func (s CircuitBreakerState) String() string {
	switch s {
	case CircuitBreakerClosed:
		return "closed"
	case CircuitBreakerOpen:
		return "open"
	case CircuitBreakerHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreakerOptions contains configuration options for a circuit breaker
type CircuitBreakerOptions struct {
	Name             string
	FailureThreshold int
	Timeout          time.Duration
	MaxRequests      int
	Output           OutputPlugin
}

// CircuitBreaker implements the circuit breaker pattern for output plugins
type CircuitBreaker struct {
	options CircuitBreakerOptions
	mu      sync.RWMutex
	
	// State
	state         CircuitBreakerState
	failures      int32 // atomic
	requests      int32 // atomic
	successes     int32 // atomic
	lastFailTime  time.Time
	lastResetTime time.Time
	
	// Statistics
	totalRequests    int64 // atomic
	totalFailures    int64 // atomic
	totalSuccesses   int64 // atomic
	stateChanges     int64 // atomic
	
	// Half-open state management
	halfOpenRequests int32 // atomic
}

// CircuitBreakerError represents an error from the circuit breaker
type CircuitBreakerError struct {
	State   CircuitBreakerState
	Message string
}

func (e *CircuitBreakerError) Error() string {
	return fmt.Sprintf("circuit breaker %s: %s", e.State.String(), e.Message)
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(options CircuitBreakerOptions) (*CircuitBreaker, error) {
	if options.FailureThreshold <= 0 {
		options.FailureThreshold = 5
	}
	if options.Timeout <= 0 {
		options.Timeout = 30 * time.Second
	}
	if options.MaxRequests <= 0 {
		options.MaxRequests = 3
	}
	if options.Name == "" {
		options.Name = "unnamed"
	}

	cb := &CircuitBreaker{
		options:       options,
		state:         CircuitBreakerClosed,
		lastResetTime: time.Now(),
	}

	return cb, nil
}

// Execute executes a function through the circuit breaker
func (cb *CircuitBreaker) Execute(ctx context.Context, entry *LogEntry) error {
	// Check if request should be allowed
	if err := cb.allowRequest(); err != nil {
		return err
	}

	// Increment request counter
	atomic.AddInt32(&cb.requests, 1)
	atomic.AddInt64(&cb.totalRequests, 1)

	// Execute the operation
	err := cb.options.Output.Write(ctx, entry)
	
	// Record the result
	cb.recordResult(err)
	
	return err
}

// allowRequest checks if a request should be allowed based on current state
func (cb *CircuitBreaker) allowRequest() error {
	cb.mu.RLock()
	state := cb.state
	cb.mu.RUnlock()

	switch state {
	case CircuitBreakerClosed:
		return nil // Allow all requests
		
	case CircuitBreakerOpen:
		// Check if timeout period has elapsed
		cb.mu.RLock()
		sinceFailure := time.Since(cb.lastFailTime)
		cb.mu.RUnlock()
		
		if sinceFailure >= cb.options.Timeout {
			// Transition to half-open
			cb.setState(CircuitBreakerHalfOpen)
			atomic.StoreInt32(&cb.halfOpenRequests, 0)
			return nil
		}
		
		return &CircuitBreakerError{
			State:   CircuitBreakerOpen,
			Message: fmt.Sprintf("circuit breaker open, timeout in %v", cb.options.Timeout-sinceFailure),
		}
		
	case CircuitBreakerHalfOpen:
		// Allow limited requests
		halfOpenCount := atomic.LoadInt32(&cb.halfOpenRequests)
		if halfOpenCount >= int32(cb.options.MaxRequests) {
			return &CircuitBreakerError{
				State:   CircuitBreakerHalfOpen,
				Message: "circuit breaker half-open, max requests reached",
			}
		}
		
		atomic.AddInt32(&cb.halfOpenRequests, 1)
		return nil
		
	default:
		return &CircuitBreakerError{
			State:   state,
			Message: "unknown circuit breaker state",
		}
	}
}

// recordResult records the result of an operation
func (cb *CircuitBreaker) recordResult(err error) {
	if err != nil {
		cb.recordFailure()
	} else {
		cb.recordSuccess()
	}
}

// recordFailure records a failure
func (cb *CircuitBreaker) recordFailure() {
	atomic.AddInt32(&cb.failures, 1)
	atomic.AddInt64(&cb.totalFailures, 1)
	
	cb.mu.Lock()
	cb.lastFailTime = time.Now()
	currentState := cb.state
	currentFailures := atomic.LoadInt32(&cb.failures)
	cb.mu.Unlock()

	// Check if we should open the circuit
	if currentState == CircuitBreakerClosed && currentFailures >= int32(cb.options.FailureThreshold) {
		cb.setState(CircuitBreakerOpen)
	} else if currentState == CircuitBreakerHalfOpen {
		// Any failure in half-open state should open the circuit
		cb.setState(CircuitBreakerOpen)
	}
}

// recordSuccess records a success
func (cb *CircuitBreaker) recordSuccess() {
	atomic.AddInt32(&cb.successes, 1)
	atomic.AddInt64(&cb.totalSuccesses, 1)
	
	cb.mu.RLock()
	currentState := cb.state
	cb.mu.RUnlock()

	if currentState == CircuitBreakerHalfOpen {
		// Check if we have enough successful requests to close the circuit
		halfOpenCount := atomic.LoadInt32(&cb.halfOpenRequests)
		if halfOpenCount >= int32(cb.options.MaxRequests) {
			cb.setState(CircuitBreakerClosed)
			cb.resetCounters()
		}
	}
}

// setState sets the circuit breaker state
func (cb *CircuitBreaker) setState(newState CircuitBreakerState) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	if cb.state != newState {
		oldState := cb.state
		cb.state = newState
		atomic.AddInt64(&cb.stateChanges, 1)
		
		// Reset appropriate counters on state change
		if newState == CircuitBreakerClosed {
			cb.lastResetTime = time.Now()
		}
		
		// Log state change (in a real implementation, you might want to use actual logging)
		_ = oldState // Placeholder to avoid unused variable
	}
}

// resetCounters resets failure/success counters
func (cb *CircuitBreaker) resetCounters() {
	atomic.StoreInt32(&cb.failures, 0)
	atomic.StoreInt32(&cb.requests, 0)
	atomic.StoreInt32(&cb.successes, 0)
	atomic.StoreInt32(&cb.halfOpenRequests, 0)
}

// State returns the current state of the circuit breaker
func (cb *CircuitBreaker) State() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// IsOpen returns true if the circuit breaker is open
func (cb *CircuitBreaker) IsOpen() bool {
	return cb.State() == CircuitBreakerOpen
}

// IsClosed returns true if the circuit breaker is closed
func (cb *CircuitBreaker) IsClosed() bool {
	return cb.State() == CircuitBreakerClosed
}

// IsHalfOpen returns true if the circuit breaker is half-open
func (cb *CircuitBreaker) IsHalfOpen() bool {
	return cb.State() == CircuitBreakerHalfOpen
}

// Reset manually resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.setState(CircuitBreakerClosed)
	cb.resetCounters()
}

// ForceOpen manually opens the circuit breaker
func (cb *CircuitBreaker) ForceOpen() {
	cb.setState(CircuitBreakerOpen)
	cb.mu.Lock()
	cb.lastFailTime = time.Now()
	cb.mu.Unlock()
}

// Stats returns circuit breaker statistics
func (cb *CircuitBreaker) Stats() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	
	stats := map[string]interface{}{
		"name":              cb.options.Name,
		"state":             cb.state.String(),
		"failure_threshold": cb.options.FailureThreshold,
		"timeout":           cb.options.Timeout.String(),
		"max_requests":      cb.options.MaxRequests,
		
		// Current window stats
		"current_failures":  atomic.LoadInt32(&cb.failures),
		"current_requests":  atomic.LoadInt32(&cb.requests),
		"current_successes": atomic.LoadInt32(&cb.successes),
		
		// Total stats
		"total_requests":  atomic.LoadInt64(&cb.totalRequests),
		"total_failures":  atomic.LoadInt64(&cb.totalFailures),
		"total_successes": atomic.LoadInt64(&cb.totalSuccesses),
		"state_changes":   atomic.LoadInt64(&cb.stateChanges),
	}
	
	// Add timing information
	if !cb.lastFailTime.IsZero() {
		stats["last_fail_time"] = cb.lastFailTime
		stats["time_since_last_fail"] = time.Since(cb.lastFailTime).String()
	}
	
	if !cb.lastResetTime.IsZero() {
		stats["last_reset_time"] = cb.lastResetTime
		stats["time_since_last_reset"] = time.Since(cb.lastResetTime).String()
	}
	
	// Add failure rate
	totalRequests := atomic.LoadInt64(&cb.totalRequests)
	if totalRequests > 0 {
		failureRate := float64(atomic.LoadInt64(&cb.totalFailures)) / float64(totalRequests)
		stats["failure_rate"] = failureRate
	}
	
	// Half-open specific stats
	if cb.state == CircuitBreakerHalfOpen {
		stats["half_open_requests"] = atomic.LoadInt32(&cb.halfOpenRequests)
	}
	
	return stats
}

// HealthCheck performs a health check on the underlying output
func (cb *CircuitBreaker) HealthCheck(ctx context.Context) error {
	// Only perform health check if circuit is not open
	if cb.IsOpen() {
		return &CircuitBreakerError{
			State:   CircuitBreakerOpen,
			Message: "circuit breaker is open, skipping health check",
		}
	}
	
	// Create a dummy log entry for health check
	entry := &LogEntry{
		Level:     InfoLevel,
		Message:   "health-check",
		Timestamp: time.Now(),
		Origin:    "circuit-breaker",
		Fields:    map[string]interface{}{"health_check": true},
	}
	
	// Execute through the circuit breaker
	return cb.Execute(ctx, entry)
}

// CircuitBreakerGroup manages multiple circuit breakers
type CircuitBreakerGroup struct {
	breakers map[string]*CircuitBreaker
	mu       sync.RWMutex
}

// NewCircuitBreakerGroup creates a new circuit breaker group
func NewCircuitBreakerGroup() *CircuitBreakerGroup {
	return &CircuitBreakerGroup{
		breakers: make(map[string]*CircuitBreaker),
	}
}

// Add adds a circuit breaker to the group
func (g *CircuitBreakerGroup) Add(name string, breaker *CircuitBreaker) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.breakers[name] = breaker
}

// Get retrieves a circuit breaker from the group
func (g *CircuitBreakerGroup) Get(name string) (*CircuitBreaker, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	breaker, exists := g.breakers[name]
	return breaker, exists
}

// Remove removes a circuit breaker from the group
func (g *CircuitBreakerGroup) Remove(name string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.breakers, name)
}

// GetAll returns all circuit breakers
func (g *CircuitBreakerGroup) GetAll() map[string]*CircuitBreaker {
	g.mu.RLock()
	defer g.mu.RUnlock()
	
	result := make(map[string]*CircuitBreaker)
	for name, breaker := range g.breakers {
		result[name] = breaker
	}
	return result
}

// ResetAll resets all circuit breakers in the group
func (g *CircuitBreakerGroup) ResetAll() {
	g.mu.RLock()
	defer g.mu.RUnlock()
	
	for _, breaker := range g.breakers {
		breaker.Reset()
	}
}

// Stats returns statistics for all circuit breakers in the group
func (g *CircuitBreakerGroup) Stats() map[string]interface{} {
	g.mu.RLock()
	defer g.mu.RUnlock()
	
	stats := make(map[string]interface{})
	for name, breaker := range g.breakers {
		stats[name] = breaker.Stats()
	}
	return stats
}