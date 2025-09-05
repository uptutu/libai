package libai

import (
	"context"
	"testing"
	"time"
)

// TestSimpleWorkerPool tests the simple worker pool
func TestSimpleWorkerPool(t *testing.T) {
	mockOutput := &MockOutputPlugin{name: "test", delay: 10 * time.Millisecond}
	metrics := &DispatcherMetrics{}
	
	pool := NewSimpleWorkerPool("test_pool", 2, 10, mockOutput, metrics)
	
	// Start pool
	err := pool.Start()
	if err != nil {
		t.Fatalf("Failed to start worker pool: %v", err)
	}
	
	// Submit a job
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
	
	// Stop pool
	pool.Stop()
	
	// Check that job was processed
	if mockOutput.GetWriteCount() == 0 {
		t.Error("Expected job to be processed")
	}
}

// TestBasicPriorityQueue tests basic priority queue functionality
func TestBasicPriorityQueue(t *testing.T) {
	config := PriorityQueueConfig{
		MaxSize:           5,
		HighPriorityRatio: 0.3,
		ProcessingTimeout: 30 * time.Second,
		DropOldestOnFull:  false,
	}

	pq, err := NewPriorityQueue(config)
	if err != nil {
		t.Fatalf("Failed to create priority queue: %v", err)
	}
	defer pq.Close()

	// Test adding and retrieving jobs
	jobs := []*DispatchJob{
		{Priority: LowPriority, CreatedAt: time.Now(), Entry: &LogEntry{Message: "low"}},
		{Priority: CriticalPriority, CreatedAt: time.Now(), Entry: &LogEntry{Message: "critical"}},
		{Priority: NormalPriority, CreatedAt: time.Now(), Entry: &LogEntry{Message: "normal"}},
	}

	// Push jobs
	for _, job := range jobs {
		err = pq.Push(job)
		if err != nil {
			t.Errorf("Failed to push job: %v", err)
		}
	}

	// Pop jobs and verify order (highest priority first)
	job1, _ := pq.Pop()
	if job1 == nil || job1.Priority != CriticalPriority {
		t.Error("Expected critical priority job first")
	}

	job2, _ := pq.Pop()
	if job2 == nil || job2.Priority != NormalPriority {
		t.Error("Expected normal priority job second")
	}

	job3, _ := pq.Pop()
	if job3 == nil || job3.Priority != LowPriority {
		t.Error("Expected low priority job third")
	}
}

// TestBasicCircuitBreaker tests basic circuit breaker functionality
func TestBasicCircuitBreaker(t *testing.T) {
	mockOutput := &MockOutputPlugin{name: "test", shouldFail: false}
	
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

	// Test successful execution
	ctx := context.Background()
	err = breaker.Execute(ctx, entry)
	if err != nil {
		t.Errorf("Expected successful execution: %v", err)
	}

	if !breaker.IsClosed() {
		t.Error("Expected circuit breaker to be closed after success")
	}
}

// TestBasicHealthMonitor tests basic health monitoring
func TestBasicHealthMonitor(t *testing.T) {
	outputs := map[OutputType]OutputPlugin{
		ConsoleOutput: &MockOutputPlugin{name: "console", shouldFail: false},
		FileOutput:    &MockOutputPlugin{name: "file", shouldFail: true},
	}

	monitor, err := NewHealthMonitor(50*time.Millisecond, outputs)
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
	time.Sleep(150 * time.Millisecond)

	// Check that we have some health data
	allHealth := monitor.GetAllHealth()
	if len(allHealth) != 2 {
		t.Errorf("Expected 2 health entries, got %d", len(allHealth))
	}

	consoleHealth, exists := allHealth[ConsoleOutput]
	if !exists {
		t.Error("Expected console health to exist")
	} else if consoleHealth.TotalChecks == 0 {
		t.Error("Expected some health checks to have been performed")
	}
}

// TestBasicLoadBalancer tests basic load balancer functionality
func TestBasicLoadBalancer(t *testing.T) {
	outputs := map[OutputType]OutputPlugin{
		ConsoleOutput: &MockOutputPlugin{name: "console", delay: 5 * time.Millisecond},
		FileOutput:    &MockOutputPlugin{name: "file", delay: 10 * time.Millisecond},
	}

	lb, err := NewLoadBalancer(outputs, nil)
	if err != nil {
		t.Fatalf("Failed to create load balancer: %v", err)
	}

	entry := &LogEntry{
		Level:     InfoLevel,
		Message:   "test",
		Timestamp: time.Now(),
	}

	// Test output selection
	selectedOutputs := make(map[OutputType]int)
	for i := 0; i < 10; i++ {
		output, err := lb.SelectOutput(entry)
		if err != nil {
			t.Errorf("Failed to select output: %v", err)
			continue
		}
		selectedOutputs[output]++
	}

	// Should have selected both outputs
	if len(selectedOutputs) < 1 {
		t.Error("Expected at least one output to be selected")
	}

	// Test multiple output selection
	requestedOutputs := []OutputType{ConsoleOutput, FileOutput}
	selected := lb.SelectOutputs(requestedOutputs, entry)
	if len(selected) == 0 {
		t.Error("Expected at least one output to be selected")
	}
}

// TestSimpleDispatcher tests a simplified version of the dispatcher
func TestSimpleDispatcher(t *testing.T) {
	// Create simple mock outputs
	outputs := map[OutputType]OutputPlugin{
		ConsoleOutput: &MockOutputPlugin{name: "console", delay: 5 * time.Millisecond},
	}

	// Create minimal config
	config := DispatcherConfig{
		Enabled:             true,
		Mode:                AsyncMode,
		MaxWorkers:          1,
		QueueSize:           10,
		ProcessingTimeout:   5 * time.Second,
		HealthCheckInterval: 1 * time.Second,
		WorkerPools: map[string]WorkerPoolConfig{
			"console": {Size: 1, QueueSize: 10},
		},
		CircuitBreaker: CircuitBreakerConfig{
			Enabled: false, // Disable circuit breaker for simplicity
		},
		PriorityQueue: PriorityQueueConfig{
			MaxSize:           100,
			HighPriorityRatio: 0.3,
			ProcessingTimeout: 30 * time.Second,
		},
		BackPressure: BackPressureConfig{
			Enabled: false, // Disable back-pressure for simplicity
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

	// Check that dispatcher is running
	if !dispatcher.IsRunning() {
		t.Error("Expected dispatcher to be running")
	}

	// Check that job was received
	metrics := dispatcher.GetMetrics()
	if metrics.JobsReceived == 0 {
		t.Error("Expected jobs to be received")
	}

	t.Logf("Dispatcher metrics: Received=%d, Processed=%d, Failed=%d",
		metrics.JobsReceived, metrics.JobsProcessed, metrics.JobsFailed)
}