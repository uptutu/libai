package libai

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MessageFormat represents different message serialization formats
type MessageFormat string

const (
	JSONFormat       MessageFormat = "json"
	AvroFormat       MessageFormat = "avro"
	ProtobufFormat   MessageFormat = "protobuf"
	MessagePackFormat MessageFormat = "msgpack"
)

// String returns the string representation of the format
func (f MessageFormat) String() string {
	return string(f)
}

// MQProvider interface defines the contract for message queue providers
type MQProvider interface {
	// Initialize the provider with configuration
	Initialize(config map[string]interface{}) error
	
	// Publish a message to the specified topic/queue
	Publish(ctx context.Context, topic string, message *MQMessage) error
	
	// PublishBatch publishes multiple messages efficiently
	PublishBatch(ctx context.Context, messages []*MQMessageBatch) error
	
	// Health check for the provider
	Ping(ctx context.Context) error
	
	// Close and cleanup resources
	Close() error
	
	// Get provider-specific statistics
	GetStats() map[string]interface{}
	
	// Get provider name
	Name() string
}

// MQMessage represents a message to be sent to message queue
type MQMessage struct {
	Topic       string                 `json:"topic"`
	Key         string                 `json:"key,omitempty"`
	Headers     map[string]string      `json:"headers,omitempty"`
	Payload     []byte                 `json:"payload"`
	Timestamp   time.Time              `json:"timestamp"`
	Partition   int32                  `json:"partition,omitempty"`
	Priority    int                    `json:"priority,omitempty"`
	TTL         time.Duration          `json:"ttl,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// MQMessageBatch represents a batch of messages with the same topic
type MQMessageBatch struct {
	Topic    string       `json:"topic"`
	Messages []*MQMessage `json:"messages"`
}

// MessageQueueOutputPlugin implements the OutputPlugin interface for message queue output
type MessageQueueOutputPlugin struct {
	config        MessageQueueConfig
	providers     map[string]MQProvider
	serializer    MessageSerializer
	router        MessageRouter
	batchQueue    map[string][]*MQMessage // topic -> messages
	batchMutex    sync.RWMutex
	workers       []*MQWorker
	workerPool    chan *MQMessage
	stopCh        chan struct{}
	wg            sync.WaitGroup
	closed        bool
	stats         *MQStats
}

// MQWorker represents a worker that processes messages
type MQWorker struct {
	ID       int
	plugin   *MessageQueueOutputPlugin
	stopCh   chan struct{}
	doneCh   chan struct{}
}

// MQStats tracks message queue statistics
type MQStats struct {
	mu                sync.RWMutex
	MessagesSent      int64     `json:"messages_sent"`
	MessagesQueued    int64     `json:"messages_queued"`
	MessagesFailed    int64     `json:"messages_failed"`
	BatchesSent       int64     `json:"batches_sent"`
	LastMessageTime   time.Time `json:"last_message_time"`
	LastErrorTime     time.Time `json:"last_error_time"`
	LastError         string    `json:"last_error,omitempty"`
	ProviderStats     map[string]interface{} `json:"provider_stats,omitempty"`
}

// MessageSerializer interface for different serialization formats
type MessageSerializer interface {
	Serialize(entry *LogEntry) ([]byte, error)
	GetFormat() MessageFormat
}

// MessageRouter interface for message routing logic
type MessageRouter interface {
	Route(entry *LogEntry, topic string) (*MQMessage, error)
	GetPartition(message *MQMessage, partitionCount int) int32
}

// NewMessageQueueOutputPlugin creates a new message queue output plugin
func NewMessageQueueOutputPlugin(config MessageQueueConfig) (OutputPlugin, error) {
	plugin := &MessageQueueOutputPlugin{
		config:     config,
		providers:  make(map[string]MQProvider),
		batchQueue: make(map[string][]*MQMessage),
		workerPool: make(chan *MQMessage, config.BufferSize),
		stopCh:     make(chan struct{}),
		stats:      &MQStats{ProviderStats: make(map[string]interface{})},
	}
	
	// Initialize serializer
	serializer, err := NewMessageSerializer(MessageFormat(config.Format))
	if err != nil {
		return nil, fmt.Errorf("failed to create serializer: %w", err)
	}
	plugin.serializer = serializer
	
	// Initialize router
	router, err := NewMessageRouter(config.Routing)
	if err != nil {
		return nil, fmt.Errorf("failed to create router: %w", err)
	}
	plugin.router = router
	
	// Initialize providers
	if err := plugin.initializeProviders(); err != nil {
		return nil, fmt.Errorf("failed to initialize providers: %w", err)
	}
	
	// Start workers
	plugin.startWorkers()
	
	// Start batch processor
	go plugin.batchProcessor()
	
	return plugin, nil
}

// initializeProviders initializes the configured MQ providers
func (m *MessageQueueOutputPlugin) initializeProviders() error {
	// Initialize primary provider
	provider, err := CreateMQProvider(m.config.Provider, m.config.Config)
	if err != nil {
		return fmt.Errorf("failed to create primary provider %s: %w", m.config.Provider, err)
	}
	
	if err := provider.Initialize(m.config.Config); err != nil {
		return fmt.Errorf("failed to initialize provider %s: %w", m.config.Provider, err)
	}
	
	m.providers[m.config.Provider] = provider
	
	// Initialize fallback providers if configured
	for _, fallbackConfig := range m.config.Fallbacks {
		fallbackProvider, err := CreateMQProvider(fallbackConfig.Provider, fallbackConfig.Config)
		if err != nil {
			// Log warning but don't fail - fallbacks are optional
			continue
		}
		
		if err := fallbackProvider.Initialize(fallbackConfig.Config); err != nil {
			// Log warning but don't fail
			continue
		}
		
		m.providers[fallbackConfig.Provider] = fallbackProvider
	}
	
	return nil
}

// startWorkers starts the worker goroutines for async message processing
func (m *MessageQueueOutputPlugin) startWorkers() {
	workerCount := m.config.WorkerCount
	if workerCount <= 0 {
		workerCount = 2 // Default worker count
	}
	
	m.workers = make([]*MQWorker, workerCount)
	
	for i := 0; i < workerCount; i++ {
		worker := &MQWorker{
			ID:     i,
			plugin: m,
			stopCh: make(chan struct{}),
			doneCh: make(chan struct{}),
		}
		
		m.workers[i] = worker
		m.wg.Add(1)
		go worker.run()
	}
}

// Write writes a log entry to the message queue
func (m *MessageQueueOutputPlugin) Write(ctx context.Context, entry *LogEntry) error {
	if m.closed {
		return fmt.Errorf("message queue output plugin is closed")
	}
	
	// Determine the topic
	topic := m.config.Topic
	if entry.Topic != "" {
		topic = entry.Topic
	}
	
	// Route the message
	message, err := m.router.Route(entry, topic)
	if err != nil {
		m.updateStats(func(stats *MQStats) {
			stats.MessagesFailed++
			stats.LastErrorTime = time.Now()
			stats.LastError = err.Error()
		})
		return fmt.Errorf("failed to route message: %w", err)
	}
	
	// Serialize the log entry
	payload, err := m.serializer.Serialize(entry)
	if err != nil {
		m.updateStats(func(stats *MQStats) {
			stats.MessagesFailed++
			stats.LastErrorTime = time.Now()
			stats.LastError = err.Error()
		})
		return fmt.Errorf("failed to serialize message: %w", err)
	}
	
	message.Payload = payload
	message.Timestamp = entry.Timestamp
	
	// Add to queue for async processing
	select {
	case m.workerPool <- message:
		m.updateStats(func(stats *MQStats) {
			stats.MessagesQueued++
		})
		return nil
	default:
		// Buffer is full, try to publish directly (blocking)
		return m.publishMessage(ctx, message)
	}
}

// publishMessage publishes a single message to the primary provider
func (m *MessageQueueOutputPlugin) publishMessage(ctx context.Context, message *MQMessage) error {
	// Try primary provider first
	primaryProvider := m.config.Provider
	if provider, exists := m.providers[primaryProvider]; exists {
		if err := provider.Publish(ctx, message.Topic, message); err == nil {
			m.updateStats(func(stats *MQStats) {
				stats.MessagesSent++
				stats.LastMessageTime = time.Now()
			})
			return nil
		}
	}
	
	// Try fallback providers
	for providerName, provider := range m.providers {
		if providerName == primaryProvider {
			continue // Already tried
		}
		
		if err := provider.Publish(ctx, message.Topic, message); err == nil {
			m.updateStats(func(stats *MQStats) {
				stats.MessagesSent++
				stats.LastMessageTime = time.Now()
			})
			return nil
		}
	}
	
	// All providers failed
	m.updateStats(func(stats *MQStats) {
		stats.MessagesFailed++
		stats.LastErrorTime = time.Now()
		stats.LastError = "all providers failed"
	})
	
	return fmt.Errorf("failed to publish message to any provider")
}

// batchProcessor processes batched messages periodically
func (m *MessageQueueOutputPlugin) batchProcessor() {
	ticker := time.NewTicker(time.Duration(m.config.FlushInterval) * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			m.flushBatches()
		case <-m.stopCh:
			m.flushBatches() // Final flush
			return
		}
	}
}

// flushBatches flushes all pending batches
func (m *MessageQueueOutputPlugin) flushBatches() {
	m.batchMutex.Lock()
	defer m.batchMutex.Unlock()
	
	if len(m.batchQueue) == 0 {
		return
	}
	
	ctx := context.Background()
	batches := make([]*MQMessageBatch, 0, len(m.batchQueue))
	
	for topic, messages := range m.batchQueue {
		if len(messages) > 0 {
			batches = append(batches, &MQMessageBatch{
				Topic:    topic,
				Messages: messages,
			})
		}
	}
	
	// Clear the queue
	m.batchQueue = make(map[string][]*MQMessage)
	
	// Publish batches
	for _, batch := range batches {
		m.publishBatch(ctx, batch)
	}
}

// publishBatch publishes a batch of messages
func (m *MessageQueueOutputPlugin) publishBatch(ctx context.Context, batch *MQMessageBatch) {
	// Try primary provider first
	primaryProvider := m.config.Provider
	if provider, exists := m.providers[primaryProvider]; exists {
		if err := provider.PublishBatch(ctx, []*MQMessageBatch{batch}); err == nil {
			m.updateStats(func(stats *MQStats) {
				stats.BatchesSent++
				stats.MessagesSent += int64(len(batch.Messages))
				stats.LastMessageTime = time.Now()
			})
			return
		}
	}
	
	// Fallback to individual message publishing
	for _, message := range batch.Messages {
		m.publishMessage(ctx, message)
	}
}

// Close closes the message queue output plugin
func (m *MessageQueueOutputPlugin) Close() error {
	if m.closed {
		return nil
	}
	
	m.closed = true
	close(m.stopCh)
	
	// Stop workers
	for _, worker := range m.workers {
		close(worker.stopCh)
		<-worker.doneCh
	}
	
	// Wait for all workers to finish
	m.wg.Wait()
	
	// Final flush
	m.flushBatches()
	
	// Close providers
	var errs []error
	for _, provider := range m.providers {
		if err := provider.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	
	if len(errs) > 0 {
		return fmt.Errorf("errors closing providers: %v", errs)
	}
	
	return nil
}

// Name returns the name of the plugin
func (m *MessageQueueOutputPlugin) Name() string {
	return fmt.Sprintf("mq_%s", m.config.Provider)
}

// GetStats returns statistics about the message queue plugin
func (m *MessageQueueOutputPlugin) GetStats() map[string]interface{} {
	m.stats.mu.RLock()
	defer m.stats.mu.RUnlock()
	
	stats := map[string]interface{}{
		"provider":         m.config.Provider,
		"format":          m.config.Format,
		"messages_sent":    m.stats.MessagesSent,
		"messages_queued":  m.stats.MessagesQueued,
		"messages_failed":  m.stats.MessagesFailed,
		"batches_sent":     m.stats.BatchesSent,
		"worker_count":     len(m.workers),
		"buffer_size":      m.config.BufferSize,
		"batch_size":       m.config.BatchSize,
	}
	
	if !m.stats.LastMessageTime.IsZero() {
		stats["last_message_time"] = m.stats.LastMessageTime
	}
	
	if !m.stats.LastErrorTime.IsZero() {
		stats["last_error_time"] = m.stats.LastErrorTime
		stats["last_error"] = m.stats.LastError
	}
	
	// Add provider-specific stats
	for providerName, provider := range m.providers {
		providerStats := provider.GetStats()
		stats[fmt.Sprintf("provider_%s", providerName)] = providerStats
	}
	
	return stats
}

// updateStats safely updates the statistics
func (m *MessageQueueOutputPlugin) updateStats(fn func(*MQStats)) {
	m.stats.mu.Lock()
	defer m.stats.mu.Unlock()
	fn(m.stats)
}

// run executes the worker loop
func (w *MQWorker) run() {
	defer func() {
		w.plugin.wg.Done()
		close(w.doneCh)
	}()
	
	for {
		select {
		case message := <-w.plugin.workerPool:
			ctx := context.Background()
			if err := w.plugin.publishMessage(ctx, message); err != nil {
				// Handle error - could implement retry logic here
			}
		case <-w.stopCh:
			return
		}
	}
}