package libai

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// WorkerPoolOptions contains configuration options for a worker pool
type WorkerPoolOptions struct {
	Name         string
	Size         int
	QueueSize    int
	AutoScaling  bool
	MinSize      int
	MaxSize      int
	IdleTimeout  time.Duration
	Output       OutputPlugin
	OutputType   OutputType
	Metrics      *DispatcherMetrics
}

// WorkerPool manages a pool of workers for processing dispatch jobs
type WorkerPool struct {
	options   WorkerPoolOptions
	workers   []*Worker
	jobQueue  chan *DispatchJob
	workerCh  chan chan *DispatchJob
	stopCh    chan struct{}
	wg        sync.WaitGroup
	mu        sync.RWMutex
	
	// State
	running     int32 // atomic
	closed      int32 // atomic
	activeJobs  int32 // atomic
	totalJobs   int64 // atomic
	failedJobs  int64 // atomic
}

// Worker represents a single worker that processes jobs
type Worker struct {
	ID         int
	pool       *WorkerPool
	jobCh      chan *DispatchJob
	stopCh     chan struct{}
	lastActive time.Time
	mu         sync.RWMutex
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(options WorkerPoolOptions) (*WorkerPool, error) {
	if options.Size <= 0 {
		options.Size = 2
	}
	if options.QueueSize <= 0 {
		options.QueueSize = 1000
	}
	if options.MinSize <= 0 {
		options.MinSize = 1
	}
	if options.MaxSize <= 0 {
		options.MaxSize = options.Size * 2
	}
	if options.IdleTimeout <= 0 {
		options.IdleTimeout = 5 * time.Minute
	}

	pool := &WorkerPool{
		options:  options,
		workers:  make([]*Worker, 0, options.MaxSize),
		jobQueue: make(chan *DispatchJob, options.QueueSize),
		workerCh: make(chan chan *DispatchJob, options.MaxSize),
		stopCh:   make(chan struct{}),
	}

	return pool, nil
}

// Start starts the worker pool
func (p *WorkerPool) Start() error {
	if !atomic.CompareAndSwapInt32(&p.running, 0, 1) {
		return fmt.Errorf("worker pool %s is already running", p.options.Name)
	}

	// Start initial workers
	for i := 0; i < p.options.Size; i++ {
		if err := p.addWorker(); err != nil {
			return fmt.Errorf("failed to start initial worker %d: %w", i, err)
		}
	}

	// Start dispatcher goroutine
	p.wg.Add(1)
	go p.dispatch()

	// Start auto-scaling monitor if enabled
	if p.options.AutoScaling {
		p.wg.Add(1)
		go p.autoScale()
	}

	return nil
}

// Stop stops the worker pool gracefully
func (p *WorkerPool) Stop() {
	if !atomic.CompareAndSwapInt32(&p.closed, 0, 1) {
		return // Already stopped
	}

	// Signal stop
	close(p.stopCh)

	// Wait for dispatcher to finish
	p.wg.Wait()

	// Stop all workers
	p.mu.Lock()
	for _, worker := range p.workers {
		close(worker.stopCh)
	}
	p.mu.Unlock()

	// Mark as not running
	atomic.StoreInt32(&p.running, 0)
}

// Submit submits a job to the worker pool
func (p *WorkerPool) Submit(job *DispatchJob) error {
	if atomic.LoadInt32(&p.closed) == 1 {
		return fmt.Errorf("worker pool %s is closed", p.options.Name)
	}

	select {
	case p.jobQueue <- job:
		atomic.AddInt64(&p.totalJobs, 1)
		return nil
	default:
		return fmt.Errorf("worker pool %s queue is full", p.options.Name)
	}
}

// dispatch dispatches jobs to available workers
func (p *WorkerPool) dispatch() {
	defer p.wg.Done()

	for {
		select {
		case job := <-p.jobQueue:
			select {
			case workerJobCh := <-p.workerCh:
				workerJobCh <- job
			case <-p.stopCh:
				// Put job back in queue for draining
				select {
				case p.jobQueue <- job:
				default:
					// Queue is full, process synchronously
					p.processJobSync(job)
				}
				return
			}
		case <-p.stopCh:
			// Process remaining jobs
			p.drainJobs()
			return
		}
	}
}

// drainJobs processes remaining jobs during shutdown
func (p *WorkerPool) drainJobs() {
	for {
		select {
		case job := <-p.jobQueue:
			// Process job synchronously
			p.processJobSync(job)
		default:
			return
		}
	}
}

// processJobSync processes a job synchronously
func (p *WorkerPool) processJobSync(job *DispatchJob) {
	ctx := context.Background()
	if err := p.options.Output.Write(ctx, job.Entry); err != nil {
		atomic.AddInt64(&p.failedJobs, 1)
	}
}

// addWorker adds a new worker to the pool
func (p *WorkerPool) addWorker() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.workers) >= p.options.MaxSize {
		return fmt.Errorf("worker pool %s has reached maximum size", p.options.Name)
	}

	worker := &Worker{
		ID:         len(p.workers),
		pool:       p,
		jobCh:      make(chan *DispatchJob),
		stopCh:     make(chan struct{}),
		lastActive: time.Now(),
	}

	p.workers = append(p.workers, worker)
	p.wg.Add(1)
	go worker.run()

	return nil
}

// removeIdleWorker removes an idle worker from the pool
func (p *WorkerPool) removeIdleWorker() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.workers) <= p.options.MinSize {
		return false
	}

	// Find the worker that has been idle the longest
	var oldestWorker *Worker
	oldestIndex := -1
	now := time.Now()

	for i, worker := range p.workers {
		worker.mu.RLock()
		idleDuration := now.Sub(worker.lastActive)
		worker.mu.RUnlock()

		if idleDuration > p.options.IdleTimeout {
			if oldestWorker == nil || worker.lastActive.Before(oldestWorker.lastActive) {
				oldestWorker = worker
				oldestIndex = i
			}
		}
	}

	if oldestWorker != nil {
		// Remove worker
		close(oldestWorker.stopCh)
		p.workers = append(p.workers[:oldestIndex], p.workers[oldestIndex+1:]...)
		
		// Update worker IDs
		for i := oldestIndex; i < len(p.workers); i++ {
			p.workers[i].ID = i
		}
		
		return true
	}

	return false
}

// autoScale monitors load and scales workers up/down
func (p *WorkerPool) autoScale() {
	defer p.wg.Done()

	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.scaleBasedOnLoad()
		case <-p.stopCh:
			return
		}
	}
}

// scaleBasedOnLoad scales the pool based on current load
func (p *WorkerPool) scaleBasedOnLoad() {
	p.mu.RLock()
	currentSize := len(p.workers)
	p.mu.RUnlock()

	queueSize := len(p.jobQueue)
	queueCapacity := cap(p.jobQueue)
	
	// Scale up if queue is more than 70% full and we haven't reached max size
	if queueSize > int(float64(queueCapacity)*0.7) && currentSize < p.options.MaxSize {
		if err := p.addWorker(); err == nil {
			// Successfully added worker
		}
		return
	}

	// Scale down if queue is less than 30% full and we have more than min size
	if queueSize < int(float64(queueCapacity)*0.3) && currentSize > p.options.MinSize {
		p.removeIdleWorker()
	}
}

// ActiveWorkers returns the number of active workers
func (p *WorkerPool) ActiveWorkers() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.workers)
}

// QueueSize returns the current queue size
func (p *WorkerPool) QueueSize() int {
	return len(p.jobQueue)
}

// Stats returns worker pool statistics
func (p *WorkerPool) Stats() map[string]interface{} {
	p.mu.RLock()
	workerCount := len(p.workers)
	p.mu.RUnlock()

	return map[string]interface{}{
		"name":          p.options.Name,
		"worker_count":  workerCount,
		"min_size":      p.options.MinSize,
		"max_size":      p.options.MaxSize,
		"queue_size":    len(p.jobQueue),
		"queue_capacity": cap(p.jobQueue),
		"active_jobs":   atomic.LoadInt32(&p.activeJobs),
		"total_jobs":    atomic.LoadInt64(&p.totalJobs),
		"failed_jobs":   atomic.LoadInt64(&p.failedJobs),
		"auto_scaling":  p.options.AutoScaling,
	}
}

// Worker methods

// run is the main worker loop
func (w *Worker) run() {
	defer w.pool.wg.Done()

	for {
		select {
		case w.pool.workerCh <- w.jobCh:
			// Worker is now available, wait for job or stop signal
			select {
			case job := <-w.jobCh:
				w.processJob(job)
			case <-w.stopCh:
				return
			}
		case <-w.stopCh:
			return
		}
	}
}

// processJob processes a single job
func (w *Worker) processJob(job *DispatchJob) {
	// Update activity timestamp
	w.mu.Lock()
	w.lastActive = time.Now()
	w.mu.Unlock()

	// Increment active jobs counter
	atomic.AddInt32(&w.pool.activeJobs, 1)
	defer atomic.AddInt32(&w.pool.activeJobs, -1)

	// Process the job
	ctx := context.Background()
	if err := w.pool.options.Output.Write(ctx, job.Entry); err != nil {
		atomic.AddInt64(&w.pool.failedJobs, 1)
		
		// Update metrics if available
		if w.pool.options.Metrics != nil {
			atomic.AddInt64(&w.pool.options.Metrics.JobsFailed, 1)
		}
	} else {
		// Update metrics if available
		if w.pool.options.Metrics != nil {
			atomic.AddInt64(&w.pool.options.Metrics.JobsProcessed, 1)
		}
	}
}