package libai

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// HealthStatus represents the health status of an output
type HealthStatus int

const (
	HealthStatusUnknown HealthStatus = iota
	HealthStatusHealthy
	HealthStatusDegraded
	HealthStatusUnhealthy
)

// String returns the string representation of the health status
func (s HealthStatus) String() string {
	switch s {
	case HealthStatusHealthy:
		return "healthy"
	case HealthStatusDegraded:
		return "degraded"
	case HealthStatusUnhealthy:
		return "unhealthy"
	default:
		return "unknown"
	}
}

// OutputHealth represents the health information of an output
type OutputHealth struct {
	OutputType         OutputType
	Plugin             OutputPlugin
	Status             HealthStatus
	LastCheck          time.Time
	LastHealthyTime    time.Time
	LastUnhealthyTime  time.Time
	ConsecutiveFailures int32
	ConsecutiveSuccesses int32
	ResponseTime       time.Duration
	ErrorMessage       string
	
	// Statistics
	TotalChecks   int64
	SuccessfulChecks int64
	FailedChecks     int64
}

// HealthMonitor monitors the health of output plugins
type HealthMonitor struct {
	outputs     map[OutputType]OutputPlugin
	healthMap   map[OutputType]*OutputHealth
	interval    time.Duration
	mu          sync.RWMutex
	stopCh      chan struct{}
	wg          sync.WaitGroup
	running     int32 // atomic
	
	// Configuration
	unhealthyThreshold   int32         // Consecutive failures to mark as unhealthy
	degradedThreshold    int32         // Consecutive failures to mark as degraded
	healthyThreshold     int32         // Consecutive successes to mark as healthy
	checkTimeout         time.Duration // Timeout for individual health checks
	
	// Callbacks
	onHealthChange func(OutputType, HealthStatus, HealthStatus)
}

// NewHealthMonitor creates a new health monitor
func NewHealthMonitor(interval time.Duration, outputs map[OutputType]OutputPlugin) (*HealthMonitor, error) {
	if interval <= 0 {
		interval = 30 * time.Second
	}

	hm := &HealthMonitor{
		outputs:            outputs,
		healthMap:          make(map[OutputType]*OutputHealth),
		interval:           interval,
		stopCh:             make(chan struct{}),
		unhealthyThreshold: 3,
		degradedThreshold:  2,
		healthyThreshold:   2,
		checkTimeout:       10 * time.Second,
	}

	// Initialize health status for all outputs
	for outputType, plugin := range outputs {
		hm.healthMap[outputType] = &OutputHealth{
			OutputType: outputType,
			Plugin:     plugin,
			Status:     HealthStatusUnknown,
			LastCheck:  time.Now(),
		}
	}

	return hm, nil
}

// Start starts the health monitoring
func (hm *HealthMonitor) Start() error {
	if !atomic.CompareAndSwapInt32(&hm.running, 0, 1) {
		return fmt.Errorf("health monitor is already running")
	}

	// Start monitoring goroutine
	hm.wg.Add(1)
	go hm.monitorLoop()

	// Perform initial health check
	go hm.checkAllOutputs()

	return nil
}

// Stop stops the health monitoring
func (hm *HealthMonitor) Stop() {
	if !atomic.CompareAndSwapInt32(&hm.running, 0, 0) {
		close(hm.stopCh)
		hm.wg.Wait()
	}
}

// monitorLoop is the main monitoring loop
func (hm *HealthMonitor) monitorLoop() {
	defer hm.wg.Done()

	ticker := time.NewTicker(hm.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			hm.checkAllOutputs()
		case <-hm.stopCh:
			return
		}
	}
}

// checkAllOutputs performs health checks on all outputs
func (hm *HealthMonitor) checkAllOutputs() {
	hm.mu.RLock()
	outputs := make([]OutputType, 0, len(hm.outputs))
	for outputType := range hm.outputs {
		outputs = append(outputs, outputType)
	}
	hm.mu.RUnlock()

	// Check outputs concurrently
	var wg sync.WaitGroup
	for _, outputType := range outputs {
		wg.Add(1)
		go func(ot OutputType) {
			defer wg.Done()
			hm.checkOutput(ot)
		}(outputType)
	}
	wg.Wait()
}

// checkOutput performs a health check on a specific output
func (hm *HealthMonitor) checkOutput(outputType OutputType) {
	hm.mu.RLock()
	health, exists := hm.healthMap[outputType]
	plugin, pluginExists := hm.outputs[outputType]
	hm.mu.RUnlock()

	if !exists || !pluginExists {
		return
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), hm.checkTimeout)
	defer cancel()

	// Perform health check
	startTime := time.Now()
	err := hm.performHealthCheck(ctx, plugin)
	responseTime := time.Since(startTime)

	// Update health status
	hm.updateHealthStatus(outputType, health, err, responseTime)
}

// performHealthCheck performs the actual health check
func (hm *HealthMonitor) performHealthCheck(ctx context.Context, plugin OutputPlugin) error {
	// Create a health check log entry
	entry := &LogEntry{
		Level:     InfoLevel,
		Message:   "health-check",
		Timestamp: time.Now(),
		Origin:    "health-monitor",
		Fields: map[string]interface{}{
			"health_check": true,
			"check_time":   time.Now().Unix(),
		},
	}

	// Try to write the health check entry
	return plugin.Write(ctx, entry)
}

// updateHealthStatus updates the health status based on the check result
func (hm *HealthMonitor) updateHealthStatus(outputType OutputType, health *OutputHealth, err error, responseTime time.Duration) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	now := time.Now()
	oldStatus := health.Status

	// Update statistics
	health.LastCheck = now
	health.ResponseTime = responseTime
	atomic.AddInt64(&health.TotalChecks, 1)

	if err != nil {
		// Health check failed
		health.ErrorMessage = err.Error()
		health.LastUnhealthyTime = now
		health.ConsecutiveFailures++
		health.ConsecutiveSuccesses = 0
		atomic.AddInt64(&health.FailedChecks, 1)

		// Determine new status based on consecutive failures
		if health.ConsecutiveFailures >= hm.unhealthyThreshold {
			health.Status = HealthStatusUnhealthy
		} else if health.ConsecutiveFailures >= hm.degradedThreshold {
			health.Status = HealthStatusDegraded
		}
	} else {
		// Health check succeeded
		health.ErrorMessage = ""
		health.LastHealthyTime = now
		health.ConsecutiveSuccesses++
		health.ConsecutiveFailures = 0
		atomic.AddInt64(&health.SuccessfulChecks, 1)

		// Determine new status based on consecutive successes
		if health.ConsecutiveSuccesses >= hm.healthyThreshold {
			health.Status = HealthStatusHealthy
		}
	}

	// Notify about status change
	if oldStatus != health.Status && hm.onHealthChange != nil {
		go hm.onHealthChange(outputType, oldStatus, health.Status)
	}
}

// GetHealth returns the health status of a specific output
func (hm *HealthMonitor) GetHealth(outputType OutputType) (*OutputHealth, bool) {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	health, exists := hm.healthMap[outputType]
	if !exists {
		return nil, false
	}

	// Return a copy to prevent concurrent access issues
	healthCopy := *health
	return &healthCopy, true
}

// GetAllHealth returns the health status of all outputs
func (hm *HealthMonitor) GetAllHealth() map[OutputType]*OutputHealth {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	result := make(map[OutputType]*OutputHealth)
	for outputType, health := range hm.healthMap {
		healthCopy := *health
		result[outputType] = &healthCopy
	}
	return result
}

// IsHealthy returns true if the specified output is healthy
func (hm *HealthMonitor) IsHealthy(outputType OutputType) bool {
	health, exists := hm.GetHealth(outputType)
	return exists && health.Status == HealthStatusHealthy
}

// GetHealthyOutputs returns a list of healthy outputs
func (hm *HealthMonitor) GetHealthyOutputs() []OutputType {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	var healthy []OutputType
	for outputType, health := range hm.healthMap {
		if health.Status == HealthStatusHealthy {
			healthy = append(healthy, outputType)
		}
	}
	return healthy
}

// GetUnhealthyOutputs returns a list of unhealthy outputs
func (hm *HealthMonitor) GetUnhealthyOutputs() []OutputType {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	var unhealthy []OutputType
	for outputType, health := range hm.healthMap {
		if health.Status == HealthStatusUnhealthy {
			unhealthy = append(unhealthy, outputType)
		}
	}
	return unhealthy
}

// GetHealthCounts returns the count of healthy and unhealthy outputs
func (hm *HealthMonitor) GetHealthCounts() (healthy, unhealthy int) {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	for _, health := range hm.healthMap {
		switch health.Status {
		case HealthStatusHealthy:
			healthy++
		case HealthStatusUnhealthy, HealthStatusDegraded:
			unhealthy++
		}
	}
	return healthy, unhealthy
}

// SetHealthChangeCallback sets a callback function to be called when health status changes
func (hm *HealthMonitor) SetHealthChangeCallback(callback func(OutputType, HealthStatus, HealthStatus)) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.onHealthChange = callback
}

// ForceCheck forces an immediate health check on all outputs
func (hm *HealthMonitor) ForceCheck() {
	go hm.checkAllOutputs()
}

// ForceCheckOutput forces an immediate health check on a specific output
func (hm *HealthMonitor) ForceCheckOutput(outputType OutputType) {
	go hm.checkOutput(outputType)
}

// Stats returns health monitoring statistics
func (hm *HealthMonitor) Stats() map[string]interface{} {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	stats := map[string]interface{}{
		"interval":             hm.interval.String(),
		"running":              atomic.LoadInt32(&hm.running) == 1,
		"unhealthy_threshold":  hm.unhealthyThreshold,
		"degraded_threshold":   hm.degradedThreshold,
		"healthy_threshold":    hm.healthyThreshold,
		"check_timeout":        hm.checkTimeout.String(),
		"total_outputs":        len(hm.healthMap),
	}

	// Calculate overall statistics
	var totalChecks, totalSuccesses, totalFailures int64
	healthyCounts := make(map[string]int)

	for outputType, health := range hm.healthMap {
		totalChecks += atomic.LoadInt64(&health.TotalChecks)
		totalSuccesses += atomic.LoadInt64(&health.SuccessfulChecks)
		totalFailures += atomic.LoadInt64(&health.FailedChecks)
		
		statusStr := health.Status.String()
		healthyCounts[statusStr]++

		// Add per-output stats
		outputStats := map[string]interface{}{
			"status":                 health.Status.String(),
			"last_check":             health.LastCheck,
			"consecutive_failures":   health.ConsecutiveFailures,
			"consecutive_successes":  health.ConsecutiveSuccesses,
			"total_checks":           atomic.LoadInt64(&health.TotalChecks),
			"successful_checks":      atomic.LoadInt64(&health.SuccessfulChecks),
			"failed_checks":          atomic.LoadInt64(&health.FailedChecks),
			"response_time":          health.ResponseTime.String(),
		}

		if !health.LastHealthyTime.IsZero() {
			outputStats["last_healthy_time"] = health.LastHealthyTime
		}
		if !health.LastUnhealthyTime.IsZero() {
			outputStats["last_unhealthy_time"] = health.LastUnhealthyTime
		}
		if health.ErrorMessage != "" {
			outputStats["error_message"] = health.ErrorMessage
		}

		stats[fmt.Sprintf("output_%s", outputType.String())] = outputStats
	}

	stats["total_checks"] = totalChecks
	stats["total_successes"] = totalSuccesses
	stats["total_failures"] = totalFailures
	stats["health_distribution"] = healthyCounts

	if totalChecks > 0 {
		stats["overall_success_rate"] = float64(totalSuccesses) / float64(totalChecks)
	}

	return stats
}

// Configuration methods

// SetUnhealthyThreshold sets the threshold for marking outputs as unhealthy
func (hm *HealthMonitor) SetUnhealthyThreshold(threshold int32) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.unhealthyThreshold = threshold
}

// SetDegradedThreshold sets the threshold for marking outputs as degraded
func (hm *HealthMonitor) SetDegradedThreshold(threshold int32) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.degradedThreshold = threshold
}

// SetHealthyThreshold sets the threshold for marking outputs as healthy
func (hm *HealthMonitor) SetHealthyThreshold(threshold int32) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.healthyThreshold = threshold
}

// SetCheckTimeout sets the timeout for individual health checks
func (hm *HealthMonitor) SetCheckTimeout(timeout time.Duration) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.checkTimeout = timeout
}