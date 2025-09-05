package libai

import (
	"container/heap"
	"fmt"
	"sync"
	"time"
)

// PriorityQueue implements a thread-safe priority queue for dispatch jobs
type PriorityQueue struct {
	config PriorityQueueConfig
	queue  *priorityQueueHeap
	mu     sync.RWMutex
	closed bool
}

// priorityQueueHeap implements heap.Interface for DispatchJob
type priorityQueueHeap []*DispatchJob

// NewPriorityQueue creates a new priority queue
func NewPriorityQueue(config PriorityQueueConfig) (*PriorityQueue, error) {
	if config.MaxSize <= 0 {
		config.MaxSize = 10000
	}
	if config.HighPriorityRatio <= 0 || config.HighPriorityRatio > 1.0 {
		config.HighPriorityRatio = 0.3 // 30% for high priority by default
	}
	if config.ProcessingTimeout <= 0 {
		config.ProcessingTimeout = 30 * time.Second
	}

	pq := &PriorityQueue{
		config: config,
		queue:  &priorityQueueHeap{},
	}

	heap.Init(pq.queue)
	return pq, nil
}

// Push adds a job to the priority queue
func (pq *PriorityQueue) Push(job *DispatchJob) error {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if pq.closed {
		return fmt.Errorf("priority queue is closed")
	}

	// Check if queue is full
	if pq.queue.Len() >= pq.config.MaxSize {
		if pq.config.DropOldestOnFull {
			// Remove the oldest (lowest priority, oldest timestamp) job
			if pq.queue.Len() > 0 {
				heap.Pop(pq.queue)
			}
		} else {
			return fmt.Errorf("priority queue is full (max size: %d)", pq.config.MaxSize)
		}
	}

	// Add job to heap
	heap.Push(pq.queue, job)
	return nil
}

// Pop removes and returns the highest priority job
func (pq *PriorityQueue) Pop() (*DispatchJob, error) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if pq.closed {
		return nil, fmt.Errorf("priority queue is closed")
	}

	if pq.queue.Len() == 0 {
		return nil, nil // Queue is empty
	}

	job := heap.Pop(pq.queue).(*DispatchJob)
	
	// Check if job has expired
	if pq.config.ProcessingTimeout > 0 {
		if time.Since(job.CreatedAt) > pq.config.ProcessingTimeout {
			// Job has expired, try to get the next one
			return pq.Pop()
		}
	}

	return job, nil
}

// Peek returns the highest priority job without removing it
func (pq *PriorityQueue) Peek() (*DispatchJob, error) {
	pq.mu.RLock()
	defer pq.mu.RUnlock()

	if pq.closed {
		return nil, fmt.Errorf("priority queue is closed")
	}

	if pq.queue.Len() == 0 {
		return nil, nil // Queue is empty
	}

	return (*pq.queue)[0], nil
}

// Size returns the current size of the queue
func (pq *PriorityQueue) Size() int {
	pq.mu.RLock()
	defer pq.mu.RUnlock()
	return pq.queue.Len()
}

// IsEmpty returns true if the queue is empty
func (pq *PriorityQueue) IsEmpty() bool {
	pq.mu.RLock()
	defer pq.mu.RUnlock()
	return pq.queue.Len() == 0
}

// IsFull returns true if the queue is at maximum capacity
func (pq *PriorityQueue) IsFull() bool {
	pq.mu.RLock()
	defer pq.mu.RUnlock()
	return pq.queue.Len() >= pq.config.MaxSize
}

// Clear removes all jobs from the queue
func (pq *PriorityQueue) Clear() []*DispatchJob {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	jobs := make([]*DispatchJob, pq.queue.Len())
	for i := 0; pq.queue.Len() > 0; i++ {
		jobs[i] = heap.Pop(pq.queue).(*DispatchJob)
	}

	return jobs
}

// Close closes the priority queue
func (pq *PriorityQueue) Close() {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	pq.closed = true
}

// Stats returns priority queue statistics
func (pq *PriorityQueue) Stats() map[string]interface{} {
	pq.mu.RLock()
	defer pq.mu.RUnlock()

	stats := map[string]interface{}{
		"size":          pq.queue.Len(),
		"max_size":      pq.config.MaxSize,
		"is_full":       pq.queue.Len() >= pq.config.MaxSize,
		"is_empty":      pq.queue.Len() == 0,
		"closed":        pq.closed,
	}

	// Calculate priority distribution
	if pq.queue.Len() > 0 {
		priorityCounts := make(map[DispatchPriority]int)
		for _, job := range *pq.queue {
			priorityCounts[job.Priority]++
		}

		stats["priority_distribution"] = map[string]interface{}{
			"critical": priorityCounts[CriticalPriority],
			"high":     priorityCounts[HighPriority],
			"normal":   priorityCounts[NormalPriority],
			"low":      priorityCounts[LowPriority],
		}
	}

	return stats
}

// GetExpiredJobs returns and removes all expired jobs
func (pq *PriorityQueue) GetExpiredJobs() []*DispatchJob {
	if pq.config.ProcessingTimeout <= 0 {
		return nil // No timeout configured
	}

	pq.mu.Lock()
	defer pq.mu.Unlock()

	var expiredJobs []*DispatchJob
	now := time.Now()

	// Create a new heap for non-expired jobs
	newQueue := &priorityQueueHeap{}
	heap.Init(newQueue)

	// Check all jobs
	for pq.queue.Len() > 0 {
		job := heap.Pop(pq.queue).(*DispatchJob)
		if now.Sub(job.CreatedAt) > pq.config.ProcessingTimeout {
			expiredJobs = append(expiredJobs, job)
		} else {
			heap.Push(newQueue, job)
		}
	}

	// Replace the queue with non-expired jobs
	pq.queue = newQueue

	return expiredJobs
}

// Heap interface implementation for priorityQueueHeap

func (h priorityQueueHeap) Len() int { return len(h) }

func (h priorityQueueHeap) Less(i, j int) bool {
	// Higher priority first
	if h[i].Priority != h[j].Priority {
		return h[i].Priority > h[j].Priority
	}

	// If same priority, older jobs first (FIFO within same priority)
	return h[i].CreatedAt.Before(h[j].CreatedAt)
}

func (h priorityQueueHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

func (h *priorityQueueHeap) Push(x interface{}) {
	*h = append(*h, x.(*DispatchJob))
}

func (h *priorityQueueHeap) Pop() interface{} {
	old := *h
	n := len(old)
	job := old[n-1]
	*h = old[0 : n-1]
	return job
}

// PriorityQueueBatch provides batch operations for the priority queue
type PriorityQueueBatch struct {
	pq   *PriorityQueue
	jobs []*DispatchJob
}

// NewPriorityQueueBatch creates a new batch processor
func NewPriorityQueueBatch(pq *PriorityQueue) *PriorityQueueBatch {
	return &PriorityQueueBatch{
		pq:   pq,
		jobs: make([]*DispatchJob, 0),
	}
}

// Add adds a job to the batch
func (b *PriorityQueueBatch) Add(job *DispatchJob) {
	b.jobs = append(b.jobs, job)
}

// PushBatch pushes all jobs in the batch to the priority queue
func (b *PriorityQueueBatch) PushBatch() error {
	b.pq.mu.Lock()
	defer b.pq.mu.Unlock()

	if b.pq.closed {
		return fmt.Errorf("priority queue is closed")
	}

	// Check available space
	availableSpace := b.pq.config.MaxSize - b.pq.queue.Len()
	if len(b.jobs) > availableSpace {
		if b.pq.config.DropOldestOnFull {
			// Remove oldest jobs to make space
			jobsToRemove := len(b.jobs) - availableSpace
			for i := 0; i < jobsToRemove && b.pq.queue.Len() > 0; i++ {
				heap.Pop(b.pq.queue)
			}
		} else {
			return fmt.Errorf("not enough space in priority queue for batch of %d jobs", len(b.jobs))
		}
	}

	// Add all jobs to the heap
	for _, job := range b.jobs {
		heap.Push(b.pq.queue, job)
	}

	// Clear the batch
	b.jobs = b.jobs[:0]
	return nil
}

// PopBatch pops multiple jobs from the priority queue
func (pq *PriorityQueue) PopBatch(maxJobs int) ([]*DispatchJob, error) {
	if maxJobs <= 0 {
		maxJobs = 10 // Default batch size
	}

	pq.mu.Lock()
	defer pq.mu.Unlock()

	if pq.closed {
		return nil, fmt.Errorf("priority queue is closed")
	}

	jobs := make([]*DispatchJob, 0, maxJobs)
	now := time.Now()

	for len(jobs) < maxJobs && pq.queue.Len() > 0 {
		job := heap.Pop(pq.queue).(*DispatchJob)
		
		// Check if job has expired
		if pq.config.ProcessingTimeout > 0 && now.Sub(job.CreatedAt) > pq.config.ProcessingTimeout {
			continue // Skip expired job
		}

		jobs = append(jobs, job)
	}

	return jobs, nil
}