package libai

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// LoadBalancingStrategy represents different load balancing strategies
type LoadBalancingStrategy int

const (
	RoundRobinStrategy LoadBalancingStrategy = iota
	WeightedRoundRobinStrategy
	LeastConnectionsStrategy
	HealthBasedStrategy
	ResponseTimeStrategy
	RandomStrategy
)

// String returns the string representation of the load balancing strategy
func (s LoadBalancingStrategy) String() string {
	switch s {
	case RoundRobinStrategy:
		return "round_robin"
	case WeightedRoundRobinStrategy:
		return "weighted_round_robin"
	case LeastConnectionsStrategy:
		return "least_connections"
	case HealthBasedStrategy:
		return "health_based"
	case ResponseTimeStrategy:
		return "response_time"
	case RandomStrategy:
		return "random"
	default:
		return "unknown"
	}
}

// OutputWeight represents the weight configuration for an output
type OutputWeight struct {
	OutputType OutputType
	Weight     int
	Enabled    bool
}

// LoadBalancerConfig contains configuration for the load balancer
type LoadBalancerConfig struct {
	Strategy      LoadBalancingStrategy
	Weights       map[OutputType]int
	HealthAware   bool
	MaxRetries    int
	RetryDelay    time.Duration
	StickySession bool // For session-based load balancing
}

// LoadBalancer manages load balancing across multiple outputs
type LoadBalancer struct {
	config        LoadBalancerConfig
	outputs       map[OutputType]OutputPlugin
	healthMonitor *HealthMonitor
	
	// State tracking
	mu                sync.RWMutex
	roundRobinCounter int64 // atomic
	connectionCounts  map[OutputType]int64
	responseTimes     map[OutputType]time.Duration
	lastAccess        map[OutputType]time.Time
	
	// Weighted round robin state
	weightedOutputs []OutputType
	currentWeights  map[OutputType]int
	
	// Statistics
	totalRequests     int64
	successfulRoutes  int64
	failedRoutes      int64
	retryAttempts     int64
}

// NewLoadBalancer creates a new load balancer
func NewLoadBalancer(outputs map[OutputType]OutputPlugin, healthMonitor *HealthMonitor) (*LoadBalancer, error) {
	config := LoadBalancerConfig{
		Strategy:      RoundRobinStrategy,
		Weights:       make(map[OutputType]int),
		HealthAware:   true,
		MaxRetries:    3,
		RetryDelay:    100 * time.Millisecond,
		StickySession: false,
	}

	// Set default weights
	for outputType := range outputs {
		config.Weights[outputType] = 1
	}

	lb := &LoadBalancer{
		config:           config,
		outputs:          outputs,
		healthMonitor:    healthMonitor,
		connectionCounts: make(map[OutputType]int64),
		responseTimes:    make(map[OutputType]time.Duration),
		lastAccess:       make(map[OutputType]time.Time),
		currentWeights:   make(map[OutputType]int),
	}

	// Initialize weighted round robin
	lb.initializeWeightedRoundRobin()

	return lb, nil
}

// initializeWeightedRoundRobin initializes the weighted round robin state
func (lb *LoadBalancer) initializeWeightedRoundRobin() {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	lb.weightedOutputs = make([]OutputType, 0)
	lb.currentWeights = make(map[OutputType]int)

	for outputType, weight := range lb.config.Weights {
		lb.currentWeights[outputType] = 0
		for i := 0; i < weight; i++ {
			lb.weightedOutputs = append(lb.weightedOutputs, outputType)
		}
	}

	// Shuffle to avoid predictable patterns
	rand.Shuffle(len(lb.weightedOutputs), func(i, j int) {
		lb.weightedOutputs[i], lb.weightedOutputs[j] = lb.weightedOutputs[j], lb.weightedOutputs[i]
	})
}

// SelectOutput selects the best output based on the configured strategy
func (lb *LoadBalancer) SelectOutput(entry *LogEntry) (OutputType, error) {
	atomic.AddInt64(&lb.totalRequests, 1)

	availableOutputs := lb.getAvailableOutputs()
	if len(availableOutputs) == 0 {
		atomic.AddInt64(&lb.failedRoutes, 1)
		return 0, fmt.Errorf("no available outputs for load balancing")
	}

	var selectedOutput OutputType
	var err error

	switch lb.config.Strategy {
	case RoundRobinStrategy:
		selectedOutput, err = lb.selectRoundRobin(availableOutputs)
	case WeightedRoundRobinStrategy:
		selectedOutput, err = lb.selectWeightedRoundRobin(availableOutputs)
	case LeastConnectionsStrategy:
		selectedOutput, err = lb.selectLeastConnections(availableOutputs)
	case HealthBasedStrategy:
		selectedOutput, err = lb.selectHealthBased(availableOutputs)
	case ResponseTimeStrategy:
		selectedOutput, err = lb.selectResponseTime(availableOutputs)
	case RandomStrategy:
		selectedOutput, err = lb.selectRandom(availableOutputs)
	default:
		selectedOutput, err = lb.selectRoundRobin(availableOutputs)
	}

	if err != nil {
		atomic.AddInt64(&lb.failedRoutes, 1)
		return 0, err
	}

	// Update connection count
	lb.mu.Lock()
	if _, exists := lb.connectionCounts[selectedOutput]; !exists {
		lb.connectionCounts[selectedOutput] = 0
	}
	lb.connectionCounts[selectedOutput]++
	lb.mu.Unlock()
	
	// Update last access time
	lb.mu.Lock()
	lb.lastAccess[selectedOutput] = time.Now()
	lb.mu.Unlock()

	atomic.AddInt64(&lb.successfulRoutes, 1)
	return selectedOutput, nil
}

// SelectOutputs selects multiple outputs for redundancy/broadcast
func (lb *LoadBalancer) SelectOutputs(requestedOutputs []OutputType, entry *LogEntry) []OutputType {
	if len(requestedOutputs) == 0 {
		// If no specific outputs requested, select the best one
		if output, err := lb.SelectOutput(entry); err == nil {
			return []OutputType{output}
		}
		return []OutputType{}
	}

	// Filter requested outputs by availability
	var selectedOutputs []OutputType
	for _, outputType := range requestedOutputs {
		if lb.isOutputAvailable(outputType) {
			selectedOutputs = append(selectedOutputs, outputType)
		}
	}

	// If no requested outputs are available, try to select an alternative
	if len(selectedOutputs) == 0 {
		if output, err := lb.SelectOutput(entry); err == nil {
			selectedOutputs = append(selectedOutputs, output)
		}
	}

	return selectedOutputs
}

// getAvailableOutputs returns a list of available outputs
func (lb *LoadBalancer) getAvailableOutputs() []OutputType {
	var available []OutputType
	for outputType := range lb.outputs {
		if lb.isOutputAvailable(outputType) {
			available = append(available, outputType)
		}
	}
	return available
}

// isOutputAvailable checks if an output is available for use
func (lb *LoadBalancer) isOutputAvailable(outputType OutputType) bool {
	// Check if output exists
	if _, exists := lb.outputs[outputType]; !exists {
		return false
	}

	// Check health if health monitoring is enabled
	if lb.config.HealthAware && lb.healthMonitor != nil {
		if !lb.healthMonitor.IsHealthy(outputType) {
			return false
		}
	}

	return true
}

// Selection strategies

func (lb *LoadBalancer) selectRoundRobin(availableOutputs []OutputType) (OutputType, error) {
	if len(availableOutputs) == 0 {
		return 0, fmt.Errorf("no available outputs")
	}

	counter := atomic.AddInt64(&lb.roundRobinCounter, 1)
	index := int((counter - 1) % int64(len(availableOutputs)))
	return availableOutputs[index], nil
}

func (lb *LoadBalancer) selectWeightedRoundRobin(availableOutputs []OutputType) (OutputType, error) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if len(lb.weightedOutputs) == 0 {
		return lb.selectRoundRobin(availableOutputs)
	}

	// Find the output with highest current weight
	var selectedOutput OutputType
	maxWeight := -1

	for _, outputType := range availableOutputs {
		weight, exists := lb.config.Weights[outputType]
		if !exists {
			continue
		}

		lb.currentWeights[outputType] += weight
		if lb.currentWeights[outputType] > maxWeight {
			maxWeight = lb.currentWeights[outputType]
			selectedOutput = outputType
		}
	}

	if maxWeight == -1 {
		return lb.selectRoundRobin(availableOutputs)
	}

	// Decrease weights
	totalWeight := 0
	for _, outputType := range availableOutputs {
		if weight, exists := lb.config.Weights[outputType]; exists {
			totalWeight += weight
		}
	}

	lb.currentWeights[selectedOutput] -= totalWeight
	return selectedOutput, nil
}

func (lb *LoadBalancer) selectLeastConnections(availableOutputs []OutputType) (OutputType, error) {
	if len(availableOutputs) == 0 {
		return 0, fmt.Errorf("no available outputs")
	}

	lb.mu.RLock()
	selectedOutput := availableOutputs[0]
	minConnections := lb.connectionCounts[selectedOutput]

	for _, outputType := range availableOutputs[1:] {
		connections := lb.connectionCounts[outputType]
		if connections < minConnections {
			minConnections = connections
			selectedOutput = outputType
		}
	}
	lb.mu.RUnlock()

	return selectedOutput, nil
}

func (lb *LoadBalancer) selectHealthBased(availableOutputs []OutputType) (OutputType, error) {
	if lb.healthMonitor == nil {
		return lb.selectRoundRobin(availableOutputs)
	}

	// Score outputs based on health metrics
	type outputScore struct {
		OutputType OutputType
		Score      float64
	}

	var scores []outputScore
	for _, outputType := range availableOutputs {
		if health, exists := lb.healthMonitor.GetHealth(outputType); exists {
			score := lb.calculateHealthScore(health)
			scores = append(scores, outputScore{
				OutputType: outputType,
				Score:      score,
			})
		}
	}

	if len(scores) == 0 {
		return lb.selectRoundRobin(availableOutputs)
	}

	// Select the output with the highest score
	bestScore := scores[0]
	for _, score := range scores[1:] {
		if score.Score > bestScore.Score {
			bestScore = score
		}
	}

	return bestScore.OutputType, nil
}

func (lb *LoadBalancer) selectResponseTime(availableOutputs []OutputType) (OutputType, error) {
	if len(availableOutputs) == 0 {
		return 0, fmt.Errorf("no available outputs")
	}

	lb.mu.RLock()
	defer lb.mu.RUnlock()

	selectedOutput := availableOutputs[0]
	minResponseTime := lb.responseTimes[selectedOutput]

	for _, outputType := range availableOutputs[1:] {
		responseTime := lb.responseTimes[outputType]
		if responseTime > 0 && (minResponseTime == 0 || responseTime < minResponseTime) {
			minResponseTime = responseTime
			selectedOutput = outputType
		}
	}

	return selectedOutput, nil
}

func (lb *LoadBalancer) selectRandom(availableOutputs []OutputType) (OutputType, error) {
	if len(availableOutputs) == 0 {
		return 0, fmt.Errorf("no available outputs")
	}

	index := rand.Intn(len(availableOutputs))
	return availableOutputs[index], nil
}

// calculateHealthScore calculates a health score for an output
func (lb *LoadBalancer) calculateHealthScore(health *OutputHealth) float64 {
	score := 0.0

	// Base score from health status
	switch health.Status {
	case HealthStatusHealthy:
		score += 100.0
	case HealthStatusDegraded:
		score += 50.0
	case HealthStatusUnhealthy:
		score += 0.0
	default:
		score += 25.0
	}

	// Adjust based on success rate
	totalChecks := atomic.LoadInt64(&health.TotalChecks)
	if totalChecks > 0 {
		successRate := float64(atomic.LoadInt64(&health.SuccessfulChecks)) / float64(totalChecks)
		score *= successRate
	}

	// Adjust based on response time (penalize slow responses)
	if health.ResponseTime > 0 {
		responseTimeMs := float64(health.ResponseTime.Nanoseconds()) / 1000000.0
		if responseTimeMs > 1000 { // Penalize if response time > 1s
			score *= 0.8
		} else if responseTimeMs < 100 { // Bonus for fast response time < 100ms
			score *= 1.1
		}
	}

	// Penalize consecutive failures
	if health.ConsecutiveFailures > 0 {
		penalty := float64(health.ConsecutiveFailures) * 10.0
		score = max(0, score-penalty)
	}

	return score
}

// ExecuteWithLoadBalancing executes an operation with load balancing and retry logic
func (lb *LoadBalancer) ExecuteWithLoadBalancing(ctx context.Context, entry *LogEntry, operation func(context.Context, OutputType, *LogEntry) error) error {
	var lastErr error
	
	for attempt := 0; attempt < lb.config.MaxRetries; attempt++ {
		if attempt > 0 {
			atomic.AddInt64(&lb.retryAttempts, 1)
			time.Sleep(lb.config.RetryDelay)
		}

		outputType, err := lb.SelectOutput(entry)
		if err != nil {
			lastErr = err
			continue
		}

		startTime := time.Now()
		err = operation(ctx, outputType, entry)
		responseTime := time.Since(startTime)

		// Update response time
		lb.mu.Lock()
		lb.responseTimes[outputType] = responseTime
		lb.mu.Unlock()

		// Decrease connection count
		lb.mu.Lock()
		if lb.connectionCounts[outputType] > 0 {
			lb.connectionCounts[outputType]--
		}
		lb.mu.Unlock()

		if err == nil {
			return nil // Success
		}

		lastErr = err
	}

	return fmt.Errorf("load balancing failed after %d attempts, last error: %w", lb.config.MaxRetries, lastErr)
}

// Configuration methods

// SetStrategy sets the load balancing strategy
func (lb *LoadBalancer) SetStrategy(strategy LoadBalancingStrategy) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.config.Strategy = strategy
	
	if strategy == WeightedRoundRobinStrategy {
		lb.initializeWeightedRoundRobin()
	}
}

// SetWeight sets the weight for an output type
func (lb *LoadBalancer) SetWeight(outputType OutputType, weight int) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.config.Weights[outputType] = weight
	
	if lb.config.Strategy == WeightedRoundRobinStrategy {
		lb.initializeWeightedRoundRobin()
	}
}

// SetHealthAware enables or disables health-aware load balancing
func (lb *LoadBalancer) SetHealthAware(enabled bool) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.config.HealthAware = enabled
}

// Stats returns load balancer statistics
func (lb *LoadBalancer) Stats() map[string]interface{} {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	stats := map[string]interface{}{
		"strategy":          lb.config.Strategy.String(),
		"health_aware":      lb.config.HealthAware,
		"max_retries":       lb.config.MaxRetries,
		"retry_delay":       lb.config.RetryDelay.String(),
		"total_requests":    atomic.LoadInt64(&lb.totalRequests),
		"successful_routes": atomic.LoadInt64(&lb.successfulRoutes),
		"failed_routes":     atomic.LoadInt64(&lb.failedRoutes),
		"retry_attempts":    atomic.LoadInt64(&lb.retryAttempts),
	}

	// Add per-output stats
	outputStats := make(map[string]interface{})
	for outputType := range lb.outputs {
		outputStats[outputType.String()] = map[string]interface{}{
			"weight":           lb.config.Weights[outputType],
			"connections":      lb.connectionCounts[outputType],
			"response_time":    lb.responseTimes[outputType].String(),
			"available":        lb.isOutputAvailable(outputType),
		}

		if lastAccess, exists := lb.lastAccess[outputType]; exists {
			outputStats[outputType.String()].(map[string]interface{})["last_access"] = lastAccess
		}
	}
	stats["outputs"] = outputStats

	// Calculate success rate
	totalRequests := atomic.LoadInt64(&lb.totalRequests)
	if totalRequests > 0 {
		successRate := float64(atomic.LoadInt64(&lb.successfulRoutes)) / float64(totalRequests)
		stats["success_rate"] = successRate
	}

	return stats
}

// Helper function for max (not available in older Go versions)
func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}