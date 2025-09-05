package libai

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
)

// SimpleWorkerPool is a simpler, more robust worker pool implementation
type SimpleWorkerPool struct {
	name     string
	size     int
	output   OutputPlugin
	metrics  *DispatcherMetrics
	
	jobs   chan *DispatchJob
	stopCh chan struct{}
	wg     sync.WaitGroup
	closed int32 // atomic
}

// NewSimpleWorkerPool creates a new simple worker pool
func NewSimpleWorkerPool(name string, size int, queueSize int, output OutputPlugin, metrics *DispatcherMetrics) *SimpleWorkerPool {
	if size <= 0 {
		size = 1
	}
	if queueSize <= 0 {
		queueSize = 100
	}
	
	return &SimpleWorkerPool{
		name:    name,
		size:    size,
		output:  output,
		metrics: metrics,
		jobs:    make(chan *DispatchJob, queueSize),
		stopCh:  make(chan struct{}),
	}
}

// Start starts the worker pool
func (p *SimpleWorkerPool) Start() error {
	// Start workers
	for i := 0; i < p.size; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
	return nil
}

// Stop stops the worker pool
func (p *SimpleWorkerPool) Stop() {
	if !atomic.CompareAndSwapInt32(&p.closed, 0, 1) {
		return // Already closed
	}
	
	close(p.stopCh)
	p.wg.Wait()
}

// Submit submits a job to the pool
func (p *SimpleWorkerPool) Submit(job *DispatchJob) error {
	if atomic.LoadInt32(&p.closed) == 1 {
		return fmt.Errorf("worker pool %s is closed", p.name)
	}
	
	select {
	case p.jobs <- job:
		return nil
	default:
		return fmt.Errorf("worker pool %s queue is full", p.name)
	}
}

// worker is the worker goroutine
func (p *SimpleWorkerPool) worker(id int) {
	defer p.wg.Done()
	
	ctx := context.Background()
	for {
		select {
		case job := <-p.jobs:
			err := p.output.Write(ctx, job.Entry)
			if err != nil && p.metrics != nil {
				atomic.AddInt64(&p.metrics.JobsFailed, 1)
			} else if p.metrics != nil {
				atomic.AddInt64(&p.metrics.JobsProcessed, 1)
			}
		case <-p.stopCh:
			// Process remaining jobs
			for {
				select {
				case job := <-p.jobs:
					p.output.Write(ctx, job.Entry)
				default:
					return
				}
			}
		}
	}
}

// ActiveWorkers returns the number of active workers
func (p *SimpleWorkerPool) ActiveWorkers() int {
	return p.size
}

// Stats returns worker pool statistics
func (p *SimpleWorkerPool) Stats() map[string]interface{} {
	return map[string]interface{}{
		"name":         p.name,
		"worker_count": p.size,
		"queue_size":   len(p.jobs),
		"queue_cap":    cap(p.jobs),
		"closed":       atomic.LoadInt32(&p.closed) == 1,
	}
}