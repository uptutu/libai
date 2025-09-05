package libai

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestOutputDispatcher tests the core functionality of the OutputDispatcher
func TestOutputDispatcher(t *testing.T) {
	// Create mock outputs
	outputs := map[OutputType]OutputPlugin{
		ConsoleOutput: &MockOutputPlugin{name: "console", delay: 10 * time.Millisecond},
		FileOutput:    &MockOutputPlugin{name: "file", delay: 20 * time.Millisecond},
	}

	// Create dispatcher config
	config := DispatcherConfig{
		Enabled:             true,
		Mode:                AsyncMode,
		MaxWorkers:          2,
		QueueSize:           100,
		ProcessingTimeout:   30 * time.Second,
		HealthCheckInterval: 5 * time.Second,
		WorkerPools: map[string]WorkerPoolConfig{
			"console": {Size: 1, QueueSize: 10},
			"file":    {Size: 1, QueueSize: 10},
		},
		CircuitBreaker: CircuitBreakerConfig{
			Enabled:          true,
			FailureThreshold: 3,
			Timeout:          10 * time.Second,
			MaxRequests:      2,
		},
		PriorityQueue: PriorityQueueConfig{
			MaxSize:           1000,
			HighPriorityRatio: 0.3,
			ProcessingTimeout: 30 * time.Second,
		},
		BackPressure: BackPressureConfig{
			Enabled:            true,
			QueueHighWaterMark: 80,
			QueueLowWaterMark:  20,
			DropRate:           0.1,
		},
	}

	// Create dispatcher
	dispatcher, err := NewOutputDispatcher(config, outputs)
	if err != nil {
		t.Fatalf("Failed to create dispatcher: %v", err)
	}
	defer dispatcher.Stop()

	// Start dispatcher
	err = dispatcher.Start()
	if err != nil {
		t.Fatalf("Failed to start dispatcher: %v", err)
	}

	// Test basic dispatch
	entry := &LogEntry{
		Level:     InfoLevel,
		Message:   "test message",
		Timestamp: time.Now(),
		Origin:    "test",
	}

	err = dispatcher.Dispatch(entry, []OutputType{ConsoleOutput})
	if err != nil {
		t.Errorf("Failed to dispatch entry: %v", err)
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Check metrics
	metrics := dispatcher.GetMetrics()
	if metrics.JobsReceived == 0 {
		t.Error("Expected jobs to be received")
	}

	// Test running state
	if !dispatcher.IsRunning() {
		t.Error("Expected dispatcher to be running")
	}

	if dispatcher.IsClosed() {
		t.Error("Expected dispatcher not to be closed")
	}
}

// TestPriorityQueue tests the priority queue functionality
func TestPriorityQueue(t *testing.T) {
	config := PriorityQueueConfig{
		MaxSize:           10,
		HighPriorityRatio: 0.3,
		ProcessingTimeout: 30 * time.Second,
		DropOldestOnFull:  false,
	}

	pq, err := NewPriorityQueue(config)
	if err != nil {
		t.Fatalf("Failed to create priority queue: %v", err)
	}
	defer pq.Close()

	// Test empty queue
	if !pq.IsEmpty() {
		t.Error("Expected queue to be empty initially")
	}

	// Test adding jobs with different priorities
	jobs := []*DispatchJob{
		{Priority: LowPriority, CreatedAt: time.Now(), Entry: &LogEntry{Message: "low"}},
		{Priority: CriticalPriority, CreatedAt: time.Now(), Entry: &LogEntry{Message: "critical"}},
		{Priority: NormalPriority, CreatedAt: time.Now(), Entry: &LogEntry{Message: "normal"}},
		{Priority: HighPriority, CreatedAt: time.Now(), Entry: &LogEntry{Message: "high"}},
	}

	// Push jobs
	for _, job := range jobs {
		err = pq.Push(job)
		if err != nil {
			t.Errorf("Failed to push job: %v", err)
		}
	}

	// Verify size
	if pq.Size() != len(jobs) {
		t.Errorf("Expected queue size %d, got %d", len(jobs), pq.Size())
	}

	// Pop jobs and verify priority order
	expectedOrder := []DispatchPriority{CriticalPriority, HighPriority, NormalPriority, LowPriority}
	for i, expectedPriority := range expectedOrder {
		job, err := pq.Pop()
		if err != nil {
			t.Errorf("Failed to pop job %d: %v", i, err)
			continue
		}
		if job == nil {
			t.Errorf("Got nil job at position %d", i)
			continue
		}
		if job.Priority != expectedPriority {
			t.Errorf("Expected priority %d at position %d, got %d", expectedPriority, i, job.Priority)
		}
	}

	// Verify empty queue
	if !pq.IsEmpty() {
		t.Error("Expected queue to be empty after popping all jobs")
	}
}

// TestWorkerPool tests the worker pool functionality
func TestWorkerPool(t *testing.T) {
	mockOutput := &MockOutputPlugin{name: "test", delay: 10 * time.Millisecond}
	
	options := WorkerPoolOptions{
		Name:        "test_pool",
		Size:        2,
		QueueSize:   10,
		AutoScaling: false,
		MinSize:     1,
		MaxSize:     5,
		IdleTimeout: 1 * time.Minute,
		Output:      mockOutput,
		OutputType:  ConsoleOutput,
		Metrics:     &DispatcherMetrics{},
	}

	pool, err := NewWorkerPool(options)
	if err != nil {
		t.Fatalf("Failed to create worker pool: %v", err)
	}
	defer pool.Stop()

	// Start pool
	err = pool.Start()
	if err != nil {
		t.Fatalf("Failed to start worker pool: %v", err)
	}

	// Submit jobs
	job := &DispatchJob{
		Entry: &LogEntry{
			Level:     InfoLevel,
			Message:   "test job",
			Timestamp: time.Now(),
		},
		Priority:  NormalPriority,
		CreatedAt: time.Now(),
	}

	err = pool.Submit(job)
	if err != nil {
		t.Errorf("Failed to submit job: %v", err)
	}

	// Wait for processing
	time.Sleep(50 * time.Millisecond)

	// Check stats
	stats := pool.Stats()
	if stats["name"] != "test_pool" {
		t.Error("Expected pool name to be 'test_pool'")
	}

	activeWorkers := stats["worker_count"].(int)
	if activeWorkers != 2 {
		t.Errorf("Expected 2 active workers, got %d", activeWorkers)
	}
}

// TestCircuitBreaker tests the circuit breaker functionality
func TestCircuitBreaker(t *testing.T) {
	mockOutput := &MockOutputPlugin{name: "test", shouldFail: true}
	
	options := CircuitBreakerOptions{
		Name:             "test_breaker",
		FailureThreshold: 2,
		Timeout:          100 * time.Millisecond,
		MaxRequests:      1,
		Output:           mockOutput,
	}

	breaker, err := NewCircuitBreaker(options)
	if err != nil {
		t.Fatalf("Failed to create circuit breaker: %v", err)
	}

	entry := &LogEntry{
		Level:     InfoLevel,
		Message:   "test",
		Timestamp: time.Now(),
	}

	// Test initial closed state
	if !breaker.IsClosed() {
		t.Error("Expected circuit breaker to be initially closed")
	}

	// Trigger failures to open circuit
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		breaker.Execute(ctx, entry)
	}

	// Circuit should now be open
	if !breaker.IsOpen() {
		t.Error("Expected circuit breaker to be open after failures")
	}

	// Test that requests are rejected when open
	err = breaker.Execute(ctx, entry)
	if err == nil {
		t.Error("Expected error when circuit breaker is open")
	}

	// Wait for timeout
	time.Sleep(150 * time.Millisecond)

	// Fix the mock output
	mockOutput.shouldFail = false

	// Should transition to half-open
	err = breaker.Execute(ctx, entry)
	if err != nil {
		t.Errorf("Expected success in half-open state: %v", err)
	}

	// Should close after successful request
	if !breaker.IsClosed() {
		time.Sleep(10 * time.Millisecond) // Give it a moment to update state
		if !breaker.IsClosed() {
			t.Error("Expected circuit breaker to close after successful request")
		}
	}
}

// TestHealthMonitor tests the health monitoring functionality
func TestHealthMonitor(t *testing.T) {
	outputs := map[OutputType]OutputPlugin{
		ConsoleOutput: &MockOutputPlugin{name: "console", shouldFail: false},
		FileOutput:    &MockOutputPlugin{name: "file", shouldFail: true},
	}

	monitor, err := NewHealthMonitor(100*time.Millisecond, outputs)
	if err != nil {
		t.Fatalf("Failed to create health monitor: %v", err)
	}
	defer monitor.Stop()

	// Start monitoring
	err = monitor.Start()
	if err != nil {
		t.Fatalf("Failed to start health monitor: %v", err)
	}

	// Wait for health checks
	time.Sleep(200 * time.Millisecond)

	// Check health status
	consoleHealth, exists := monitor.GetHealth(ConsoleOutput)
	if !exists {
		t.Error("Expected console output health to exist")
	} else if consoleHealth.Status != HealthStatusHealthy {
		t.Errorf("Expected console to be healthy, got %s", consoleHealth.Status.String())
	}

	fileHealth, exists := monitor.GetHealth(FileOutput)
	if !exists {
		t.Error("Expected file output health to exist")
	} else if fileHealth.Status == HealthStatusHealthy {
		t.Error("Expected file output to be unhealthy")
	}

	// Test health counts
	healthy, unhealthy := monitor.GetHealthCounts()
	if healthy == 0 {
		t.Error("Expected at least one healthy output")
	}
	if unhealthy == 0 {
		t.Error("Expected at least one unhealthy output")
	}

	// Test force check
	monitor.ForceCheck()
	time.Sleep(50 * time.Millisecond)
}

// TestLoadBalancer tests the load balancer functionality
func TestLoadBalancer(t *testing.T) {
	outputs := map[OutputType]OutputPlugin{
		ConsoleOutput: &MockOutputPlugin{name: "console", delay: 10 * time.Millisecond},
		FileOutput:    &MockOutputPlugin{name: "file", delay: 20 * time.Millisecond},
	}

	healthMonitor, _ := NewHealthMonitor(1*time.Second, outputs)
	defer healthMonitor.Stop()

	lb, err := NewLoadBalancer(outputs, healthMonitor)
	if err != nil {
		t.Fatalf("Failed to create load balancer: %v", err)
	}

	entry := &LogEntry{
		Level:     InfoLevel,
		Message:   "test",
		Timestamp: time.Now(),
	}

	// Test round robin selection
	lb.SetStrategy(RoundRobinStrategy)
	
	selectedOutputs := make(map[OutputType]int)
	for i := 0; i < 10; i++ {
		output, err := lb.SelectOutput(entry)
		if err != nil {
			t.Errorf("Failed to select output: %v", err)
			continue
		}
		selectedOutputs[output]++
	}

	// Should distribute fairly
	if len(selectedOutputs) != 2 {
		t.Errorf("Expected 2 different outputs to be selected, got %d", len(selectedOutputs))
	}

	// Test weighted round robin
	lb.SetStrategy(WeightedRoundRobinStrategy)
	lb.SetWeight(ConsoleOutput, 3)
	lb.SetWeight(FileOutput, 1)

	selectedOutputs = make(map[OutputType]int)
	for i := 0; i < 20; i++ {
		output, err := lb.SelectOutput(entry)
		if err != nil {
			t.Errorf("Failed to select output: %v", err)
			continue
		}
		selectedOutputs[output]++
	}

	// Console should be selected more often due to higher weight
	if selectedOutputs[ConsoleOutput] <= selectedOutputs[FileOutput] {
		t.Error("Expected console output to be selected more often due to higher weight")
	}

	// Test random selection
	lb.SetStrategy(RandomStrategy)
	selectedOutputs = make(map[OutputType]int)
	for i := 0; i < 100; i++ {
		output, err := lb.SelectOutput(entry)
		if err != nil {
			t.Errorf("Failed to select output: %v", err)
			continue
		}
		selectedOutputs[output]++
	}

	// Should have some distribution (not perfect due to randomness)
	if len(selectedOutputs) == 0 {
		t.Error("Expected some outputs to be selected")
	}
}

// TestAsyncProcessingIntegration tests the full integration of async processing components
func TestAsyncProcessingIntegration(t *testing.T) {
	// Create mock outputs with different characteristics
	outputs := map[OutputType]OutputPlugin{
		ConsoleOutput:      &MockOutputPlugin{name: "console", delay: 5 * time.Millisecond},
		FileOutput:         &MockOutputPlugin{name: "file", delay: 10 * time.Millisecond},
		MessageQueueOutput: &MockOutputPlugin{name: "mq", delay: 15 * time.Millisecond},
	}

	// Create dispatcher with full configuration
	config := DispatcherConfig{
		Enabled:             true,
		Mode:                HybridMode,
		MaxWorkers:          3,
		QueueSize:           50,
		ProcessingTimeout:   10 * time.Second,
		HealthCheckInterval: 1 * time.Second,
		WorkerPools: map[string]WorkerPoolConfig{
			"console":      {Size: 1, QueueSize: 10, AutoScaling: true, MinSize: 1, MaxSize: 3},
			"file":         {Size: 1, QueueSize: 10, AutoScaling: false},
			"messagequeue": {Size: 2, QueueSize: 20, AutoScaling: true, MinSize: 1, MaxSize: 4},
		},
		CircuitBreaker: CircuitBreakerConfig{
			Enabled:          true,
			FailureThreshold: 3,
			Timeout:          5 * time.Second,
			MaxRequests:      2,
		},
		PriorityQueue: PriorityQueueConfig{
			MaxSize:           100,
			HighPriorityRatio: 0.4,
			ProcessingTimeout: 30 * time.Second,
			DropOldestOnFull:  true,
		},
		BackPressure: BackPressureConfig{
			Enabled:            true,
			QueueHighWaterMark: 40,
			QueueLowWaterMark:  10,
			DropRate:           0.2,
		},
	}

	dispatcher, err := NewOutputDispatcher(config, outputs)
	if err != nil {
		t.Fatalf("Failed to create dispatcher: %v", err)
	}
	defer dispatcher.Stop()

	err = dispatcher.Start()
	if err != nil {
		t.Fatalf("Failed to start dispatcher: %v", err)
	}

	// Test dispatching different types of log entries
	testCases := []struct {
		level    LogLevel
		outputs  []OutputType
		expected string
	}{
		{FatalLevel, []OutputType{ConsoleOutput, FileOutput}, "should process sync due to critical priority"},
		{ErrorLevel, []OutputType{MessageQueueOutput}, "should process sync in hybrid mode"},
		{InfoLevel, []OutputType{ConsoleOutput}, "should process async"},
		{DebugLevel, []OutputType{FileOutput, MessageQueueOutput}, "should process async with low priority"},
	}

	var wg sync.WaitGroup
	for i, tc := range testCases {
		wg.Add(1)
		go func(i int, tc struct {
			level    LogLevel
			outputs  []OutputType
			expected string
		}) {
			defer wg.Done()

			entry := &LogEntry{
				Level:     tc.level,
				Message:   fmt.Sprintf("test message %d - %s", i, tc.expected),
				Timestamp: time.Now(),
				Origin:    "integration_test",
				Fields:    map[string]interface{}{"test_id": i},
			}

			err := dispatcher.Dispatch(entry, tc.outputs)
			if err != nil {
				t.Errorf("Failed to dispatch entry %d: %v", i, err)
			}
		}(i, tc)
	}

	wg.Wait()

	// Wait for async processing
	time.Sleep(200 * time.Millisecond)

	// Verify metrics
	metrics := dispatcher.GetMetrics()
	if metrics.JobsReceived == 0 {
		t.Error("Expected jobs to be received")
	}

	if metrics.JobsProcessed == 0 {
		t.Error("Expected jobs to be processed")
	}

	// Test load under concurrent stress
	const numGoroutines = 10
	const messagesPerGoroutine = 20

	var stressWg sync.WaitGroup
	for g := 0; g < numGoroutines; g++ {
		stressWg.Add(1)
		go func(goroutineID int) {
			defer stressWg.Done()
			for i := 0; i < messagesPerGoroutine; i++ {
				entry := &LogEntry{
					Level:     InfoLevel,
					Message:   fmt.Sprintf("stress test message %d-%d", goroutineID, i),
					Timestamp: time.Now(),
					Origin:    "stress_test",
					Fields: map[string]interface{}{
						"goroutine_id": goroutineID,
						"message_id":   i,
					},
				}

				err := dispatcher.Dispatch(entry, []OutputType{ConsoleOutput, FileOutput})
				if err != nil {
					// Some failures are expected under high load
					t.Logf("Dispatch failed during stress test: %v", err)
				}
			}
		}(g)
	}

	stressWg.Wait()

	// Wait for processing
	time.Sleep(500 * time.Millisecond)

	// Check final metrics
	finalMetrics := dispatcher.GetMetrics()
	t.Logf("Final metrics: Received=%d, Processed=%d, Failed=%d, Dropped=%d",
		finalMetrics.JobsReceived, finalMetrics.JobsProcessed,
		finalMetrics.JobsFailed, finalMetrics.JobsDropped)

	if finalMetrics.JobsReceived == 0 {
		t.Error("Expected jobs to be received during stress test")
	}

	// Test graceful shutdown
	err = dispatcher.Stop()
	if err != nil {
		t.Errorf("Failed to stop dispatcher: %v", err)
	}

	if !dispatcher.IsClosed() {
		t.Error("Expected dispatcher to be closed after stop")
	}
}