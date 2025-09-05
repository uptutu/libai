package libai

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// DispatchMode defines how the dispatcher operates
type DispatchMode string

const (
	SyncMode   DispatchMode = "sync"   // Synchronous processing (existing behavior)
	AsyncMode  DispatchMode = "async"  // Asynchronous processing
	HybridMode DispatchMode = "hybrid" // Critical logs sync, others async
)

// DispatchPriority defines log entry priorities
type DispatchPriority int

const (
	LowPriority    DispatchPriority = 1
	NormalPriority DispatchPriority = 2
	HighPriority   DispatchPriority = 3
	CriticalPriority DispatchPriority = 4
)

// DispatchJob represents a job to be processed by the dispatcher
type DispatchJob struct {
	Entry     *LogEntry        `json:"entry"`
	Outputs   []OutputType     `json:"outputs"`
	Priority  DispatchPriority `json:"priority"`
	CreatedAt time.Time        `json:"created_at"`
	Retries   int              `json:"retries"`
	MaxRetries int             `json:"max_retries"`
}

// DispatcherConfig contains configuration for the output dispatcher
type DispatcherConfig struct {
	Enabled           bool          `json:"enabled" yaml:"enabled"`
	Mode              DispatchMode  `json:"mode" yaml:"mode"`
	MaxWorkers        int           `json:"max_workers" yaml:"max_workers"`
	QueueSize         int           `json:"queue_size" yaml:"queue_size"`
	ProcessingTimeout time.Duration `json:"processing_timeout" yaml:"processing_timeout"`
	HealthCheckInterval time.Duration `json:"health_check_interval" yaml:"health_check_interval"`
	
	// Worker pool configuration
	WorkerPools map[string]WorkerPoolConfig `json:"worker_pools" yaml:"worker_pools"`
	
	// Circuit breaker configuration
	CircuitBreaker CircuitBreakerConfig `json:"circuit_breaker" yaml:"circuit_breaker"`
	
	// Priority queue configuration
	PriorityQueue PriorityQueueConfig `json:"priority_queue" yaml:"priority_queue"`
	
	// Back-pressure handling
	BackPressure BackPressureConfig `json:"back_pressure" yaml:"back_pressure"`
}

// WorkerPoolConfig contains configuration for individual worker pools
type WorkerPoolConfig struct {
	Size         int           `json:"size" yaml:"size"`
	QueueSize    int           `json:"queue_size" yaml:"queue_size"`
	AutoScaling  bool          `json:"auto_scaling" yaml:"auto_scaling"`
	MinSize      int           `json:"min_size" yaml:"min_size"`
	MaxSize      int           `json:"max_size" yaml:"max_size"`
	IdleTimeout  time.Duration `json:"idle_timeout" yaml:"idle_timeout"`
}

// CircuitBreakerConfig contains circuit breaker configuration
type CircuitBreakerConfig struct {
	Enabled          bool          `json:"enabled" yaml:"enabled"`
	FailureThreshold int           `json:"failure_threshold" yaml:"failure_threshold"`
	Timeout          time.Duration `json:"timeout" yaml:"timeout"`
	MaxRequests      int           `json:"max_requests" yaml:"max_requests"`
}

// PriorityQueueConfig contains priority queue configuration
type PriorityQueueConfig struct {
	MaxSize             int           `json:"max_size" yaml:"max_size"`
	HighPriorityRatio   float64       `json:"high_priority_ratio" yaml:"high_priority_ratio"`
	ProcessingTimeout   time.Duration `json:"processing_timeout" yaml:"processing_timeout"`
	DropOldestOnFull    bool          `json:"drop_oldest_on_full" yaml:"drop_oldest_on_full"`
}

// BackPressureConfig contains back-pressure handling configuration  
type BackPressureConfig struct {
	Enabled            bool    `json:"enabled" yaml:"enabled"`
	QueueHighWaterMark int     `json:"queue_high_water_mark" yaml:"queue_high_water_mark"`
	QueueLowWaterMark  int     `json:"queue_low_water_mark" yaml:"queue_low_water_mark"`
	DropRate           float64 `json:"drop_rate" yaml:"drop_rate"` // 0.0 - 1.0
}

// OutputDispatcher manages async processing of log outputs
type OutputDispatcher struct {
	config       DispatcherConfig
	outputs      map[OutputType]OutputPlugin
	workerPools  map[OutputType]*SimpleWorkerPool
	priorityQueue *PriorityQueue
	circuitBreakers map[OutputType]*CircuitBreaker
	healthMonitor *HealthMonitor
	loadBalancer  *LoadBalancer
	metrics       *DispatcherMetrics
	
	// Control channels
	stopCh    chan struct{}
	doneCh    chan struct{}
	wg        sync.WaitGroup
	
	// State
	running   int32 // atomic
	closed    int32 // atomic
	mu        sync.RWMutex
}

// DispatcherMetrics tracks dispatcher performance metrics
type DispatcherMetrics struct {
	mu sync.RWMutex
	
	// Throughput metrics
	JobsReceived    int64 `json:"jobs_received"`
	JobsProcessed   int64 `json:"jobs_processed"`
	JobsFailed      int64 `json:"jobs_failed"`
	JobsDropped     int64 `json:"jobs_dropped"`
	JobsRetried     int64 `json:"jobs_retried"`
	
	// Performance metrics
	AvgProcessingTime time.Duration `json:"avg_processing_time"`
	QueueSize         int           `json:"queue_size"`
	ActiveWorkers     int           `json:"active_workers"`
	
	// Health metrics
	HealthyOutputs   int           `json:"healthy_outputs"`
	UnhealthyOutputs int           `json:"unhealthy_outputs"`
	CircuitOpen      int           `json:"circuit_open"`
	LastUpdateTime   time.Time     `json:"last_update_time"`
}

// NewOutputDispatcher creates a new output dispatcher
func NewOutputDispatcher(config DispatcherConfig, outputs map[OutputType]OutputPlugin) (*OutputDispatcher, error) {
	if !config.Enabled {
		return nil, nil // Dispatcher disabled
	}
	
	dispatcher := &OutputDispatcher{
		config:          config,
		outputs:         outputs,
		workerPools:     make(map[OutputType]*SimpleWorkerPool),
		circuitBreakers: make(map[OutputType]*CircuitBreaker),
		stopCh:          make(chan struct{}),
		doneCh:          make(chan struct{}),
		metrics:         &DispatcherMetrics{},
	}
	
	// Initialize priority queue
	priorityQueue, err := NewPriorityQueue(config.PriorityQueue)
	if err != nil {
		return nil, fmt.Errorf("failed to create priority queue: %w", err)
	}
	dispatcher.priorityQueue = priorityQueue
	
	// Initialize worker pools for each output type
	if err := dispatcher.initializeWorkerPools(); err != nil {
		return nil, fmt.Errorf("failed to initialize worker pools: %w", err)
	}
	
	// Initialize circuit breakers
	if err := dispatcher.initializeCircuitBreakers(); err != nil {
		return nil, fmt.Errorf("failed to initialize circuit breakers: %w", err)
	}
	
	// Initialize health monitor
	healthMonitor, err := NewHealthMonitor(config.HealthCheckInterval, outputs)
	if err != nil {
		return nil, fmt.Errorf("failed to create health monitor: %w", err)
	}
	dispatcher.healthMonitor = healthMonitor
	
	// Initialize load balancer
	loadBalancer, err := NewLoadBalancer(outputs, dispatcher.healthMonitor)
	if err != nil {
		return nil, fmt.Errorf("failed to create load balancer: %w", err)
	}
	dispatcher.loadBalancer = loadBalancer
	
	return dispatcher, nil
}

// initializeWorkerPools initializes worker pools for each output type
func (d *OutputDispatcher) initializeWorkerPools() error {
	for outputType, output := range d.outputs {
		// Get pool config for this output type, or use default
		poolConfig := d.getWorkerPoolConfig(outputType)
		
		pool := NewSimpleWorkerPool(
			fmt.Sprintf("%s_pool", outputType),
			poolConfig.Size,
			poolConfig.QueueSize,
			output,
			d.metrics,
		)
		
		d.workerPools[outputType] = pool
	}
	
	return nil
}

// initializeCircuitBreakers initializes circuit breakers for each output
func (d *OutputDispatcher) initializeCircuitBreakers() error {
	if !d.config.CircuitBreaker.Enabled {
		return nil
	}
	
	for outputType, output := range d.outputs {
		breaker, err := NewCircuitBreaker(CircuitBreakerOptions{
			Name:             fmt.Sprintf("%s_breaker", outputType),
			FailureThreshold: d.config.CircuitBreaker.FailureThreshold,
			Timeout:          d.config.CircuitBreaker.Timeout,
			MaxRequests:      d.config.CircuitBreaker.MaxRequests,
			Output:           output,
		})
		if err != nil {
			return fmt.Errorf("failed to create circuit breaker for %s: %w", outputType, err)
		}
		
		d.circuitBreakers[outputType] = breaker
	}
	
	return nil
}

// getWorkerPoolConfig returns worker pool config for a specific output type
func (d *OutputDispatcher) getWorkerPoolConfig(outputType OutputType) WorkerPoolConfig {
	if config, exists := d.config.WorkerPools[outputType.String()]; exists {
		return config
	}
	
	// Return default config
	return WorkerPoolConfig{
		Size:        2,
		QueueSize:   1000,
		AutoScaling: false,
		MinSize:     1,
		MaxSize:     10,
		IdleTimeout: 5 * time.Minute,
	}
}

// Start starts the output dispatcher
func (d *OutputDispatcher) Start() error {
	if !atomic.CompareAndSwapInt32(&d.running, 0, 1) {
		return fmt.Errorf("dispatcher is already running")
	}
	
	// Start worker pools
	for _, pool := range d.workerPools {
		if err := pool.Start(); err != nil {
			return fmt.Errorf("failed to start worker pool: %w", err)
		}
	}
	
	// Start health monitor
	if err := d.healthMonitor.Start(); err != nil {
		return fmt.Errorf("failed to start health monitor: %w", err)
	}
	
	// Start main dispatcher goroutine
	d.wg.Add(1)
	go d.run()
	
	return nil
}

// Stop stops the output dispatcher
func (d *OutputDispatcher) Stop() error {
	if !atomic.CompareAndSwapInt32(&d.closed, 0, 1) {
		return nil // Already stopped
	}
	
	// Signal stop
	close(d.stopCh)
	
	// Wait for main goroutine to finish
	d.wg.Wait()
	
	// Stop worker pools
	for _, pool := range d.workerPools {
		pool.Stop()
	}
	
	// Stop health monitor
	d.healthMonitor.Stop()
	
	// Close priority queue
	d.priorityQueue.Close()
	
	// Mark as not running
	atomic.StoreInt32(&d.running, 0)
	
	return nil
}

// Dispatch dispatches a log entry for async processing
func (d *OutputDispatcher) Dispatch(entry *LogEntry, outputs []OutputType) error {
	if atomic.LoadInt32(&d.closed) == 1 {
		return fmt.Errorf("dispatcher is closed")
	}
	
	// Determine priority
	priority := d.determinePriority(entry)
	
	// Check if we should process synchronously based on mode and priority
	if d.shouldProcessSync(entry, priority) {
		return d.processSynchronously(entry, outputs)
	}
	
	// Create dispatch job
	job := &DispatchJob{
		Entry:      entry.Copy(),
		Outputs:    outputs,
		Priority:   priority,
		CreatedAt:  time.Now(),
		MaxRetries: 3,
	}
	
	// Add to priority queue
	if err := d.priorityQueue.Push(job); err != nil {
		atomic.AddInt64(&d.metrics.JobsDropped, 1)
		
		// If queue is full and it's a critical log, process synchronously as fallback
		if priority >= CriticalPriority {
			return d.processSynchronously(entry, outputs)
		}
		
		return fmt.Errorf("failed to enqueue job: %w", err)
	}
	
	atomic.AddInt64(&d.metrics.JobsReceived, 1)
	return nil
}

// run is the main dispatcher loop
func (d *OutputDispatcher) run() {
	defer d.wg.Done()
	defer close(d.doneCh)
	
	ticker := time.NewTicker(100 * time.Millisecond) // Process queue every 100ms
	defer ticker.Stop()
	
	for {
		select {
		case <-d.stopCh:
			// Process remaining jobs before stopping
			d.drainQueue()
			return
			
		case <-ticker.C:
			d.processQueue()
			d.updateMetrics()
		}
	}
}

// processQueue processes jobs from the priority queue
func (d *OutputDispatcher) processQueue() {
	// Process jobs in batches for efficiency
	batchSize := 10
	for i := 0; i < batchSize; i++ {
		job, err := d.priorityQueue.Pop()
		if err != nil || job == nil {
			break // Queue is empty
		}
		
		d.processJob(job)
	}
}

// processJob processes a single dispatch job
func (d *OutputDispatcher) processJob(job *DispatchJob) {
	startTime := time.Now()
	defer func() {
		processingTime := time.Since(startTime)
		d.updateProcessingTime(processingTime)
	}()
	
	// Use load balancer to determine optimal outputs
	optimizedOutputs := d.loadBalancer.SelectOutputs(job.Outputs, job.Entry)
	
	// Process each output
	for _, outputType := range optimizedOutputs {
		d.processOutputJob(job, outputType)
	}
}

// processOutputJob processes a job for a specific output
func (d *OutputDispatcher) processOutputJob(job *DispatchJob, outputType OutputType) {
	pool, exists := d.workerPools[outputType]
	if !exists {
		atomic.AddInt64(&d.metrics.JobsFailed, 1)
		return
	}
	
	// Check circuit breaker
	if breaker, exists := d.circuitBreakers[outputType]; exists && breaker != nil {
		if breaker.State() == CircuitBreakerOpen {
			atomic.AddInt64(&d.metrics.JobsFailed, 1)
			return
		}
	}
	
	// Submit to worker pool
	if err := pool.Submit(job); err != nil {
		// If worker pool is full, handle based on priority
		if job.Priority >= CriticalPriority {
			// For critical logs, try to process directly
			d.processDirectly(job.Entry, outputType)
		} else {
			// For non-critical logs, increment retry count
			if job.Retries < job.MaxRetries {
				job.Retries++
				atomic.AddInt64(&d.metrics.JobsRetried, 1)
				
				// Re-queue with delay (exponential backoff)
				delay := time.Duration(job.Retries) * time.Second
				time.AfterFunc(delay, func() {
					d.priorityQueue.Push(job)
				})
			} else {
				atomic.AddInt64(&d.metrics.JobsFailed, 1)
			}
		}
	}
}

// processSynchronously processes a job synchronously (fallback mode)
func (d *OutputDispatcher) processSynchronously(entry *LogEntry, outputs []OutputType) error {
	ctx := context.Background()
	
	for _, outputType := range outputs {
		if output, exists := d.outputs[outputType]; exists {
			if err := output.Write(ctx, entry); err != nil {
				// Continue to next output on error
				continue
			}
		}
	}
	
	return nil
}

// processDirectly processes directly without queuing (emergency fallback)
func (d *OutputDispatcher) processDirectly(entry *LogEntry, outputType OutputType) {
	if output, exists := d.outputs[outputType]; exists {
		ctx := context.Background()
		output.Write(ctx, entry)
	}
}

// determinePriority determines the priority of a log entry
func (d *OutputDispatcher) determinePriority(entry *LogEntry) DispatchPriority {
	switch entry.Level {
	case FatalLevel:
		return CriticalPriority
	case ErrorLevel:
		return HighPriority
	case WarnLevel:
		return NormalPriority
	case InfoLevel:
		return NormalPriority
	case DebugLevel:
		return LowPriority
	default:
		return NormalPriority
	}
}

// shouldProcessSync determines if a log should be processed synchronously
func (d *OutputDispatcher) shouldProcessSync(entry *LogEntry, priority DispatchPriority) bool {
	switch d.config.Mode {
	case SyncMode:
		return true
	case AsyncMode:
		return false
	case HybridMode:
		// Process critical and error logs synchronously
		return priority >= HighPriority
	default:
		return true
	}
}

// drainQueue processes remaining jobs before shutdown
func (d *OutputDispatcher) drainQueue() {
	for {
		job, err := d.priorityQueue.Pop()
		if err != nil || job == nil {
			break
		}
		
		// Process remaining jobs synchronously for guaranteed delivery
		d.processSynchronously(job.Entry, job.Outputs)
	}
}

// updateMetrics updates dispatcher metrics
func (d *OutputDispatcher) updateMetrics() {
	d.metrics.mu.Lock()
	defer d.metrics.mu.Unlock()
	
	d.metrics.QueueSize = d.priorityQueue.Size()
	d.metrics.ActiveWorkers = d.getActiveWorkers()
	d.metrics.HealthyOutputs, d.metrics.UnhealthyOutputs = d.healthMonitor.GetHealthCounts()
	d.metrics.CircuitOpen = d.getOpenCircuitCount()
	d.metrics.LastUpdateTime = time.Now()
}

// updateProcessingTime updates average processing time
func (d *OutputDispatcher) updateProcessingTime(duration time.Duration) {
	d.metrics.mu.Lock()
	defer d.metrics.mu.Unlock()
	
	// Simple moving average
	if d.metrics.AvgProcessingTime == 0 {
		d.metrics.AvgProcessingTime = duration
	} else {
		d.metrics.AvgProcessingTime = (d.metrics.AvgProcessingTime + duration) / 2
	}
}

// getActiveWorkers returns the total number of active workers
func (d *OutputDispatcher) getActiveWorkers() int {
	total := 0
	for _, pool := range d.workerPools {
		total += pool.ActiveWorkers()
	}
	return total
}

// getOpenCircuitCount returns the number of open circuit breakers
func (d *OutputDispatcher) getOpenCircuitCount() int {
	count := 0
	for _, breaker := range d.circuitBreakers {
		if breaker.State() == CircuitBreakerOpen {
			count++
		}
	}
	return count
}

// GetMetrics returns current dispatcher metrics
func (d *OutputDispatcher) GetMetrics() *DispatcherMetrics {
	d.metrics.mu.RLock()
	defer d.metrics.mu.RUnlock()
	
	// Return a copy
	metrics := *d.metrics
	return &metrics
}

// IsRunning returns true if the dispatcher is running
func (d *OutputDispatcher) IsRunning() bool {
	return atomic.LoadInt32(&d.running) == 1
}

// IsClosed returns true if the dispatcher is closed
func (d *OutputDispatcher) IsClosed() bool {
	return atomic.LoadInt32(&d.closed) == 1
}