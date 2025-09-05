package libai

import (
	"context"
	"fmt"
	"sync"
)

// MQProviderRegistry manages registered MQ providers
type MQProviderRegistry struct {
	providers map[string]MQProviderFactory
	mu        sync.RWMutex
}

// MQProviderFactory creates new instances of MQ providers
type MQProviderFactory interface {
	CreateProvider(config map[string]interface{}) (MQProvider, error)
	Name() string
	Description() string
}

// Global registry instance
var globalMQRegistry = &MQProviderRegistry{
	providers: make(map[string]MQProviderFactory),
}

// RegisterMQProvider registers a new MQ provider factory
func RegisterMQProvider(name string, factory MQProviderFactory) error {
	globalMQRegistry.mu.Lock()
	defer globalMQRegistry.mu.Unlock()
	
	if _, exists := globalMQRegistry.providers[name]; exists {
		return fmt.Errorf("MQ provider %s is already registered", name)
	}
	
	globalMQRegistry.providers[name] = factory
	return nil
}

// GetMQProvider retrieves a registered MQ provider factory
func GetMQProvider(name string) (MQProviderFactory, error) {
	globalMQRegistry.mu.RLock()
	defer globalMQRegistry.mu.RUnlock()
	
	factory, exists := globalMQRegistry.providers[name]
	if !exists {
		return nil, fmt.Errorf("MQ provider %s is not registered", name)
	}
	
	return factory, nil
}

// ListMQProviders returns all registered MQ provider names
func ListMQProviders() []string {
	globalMQRegistry.mu.RLock()
	defer globalMQRegistry.mu.RUnlock()
	
	names := make([]string, 0, len(globalMQRegistry.providers))
	for name := range globalMQRegistry.providers {
		names = append(names, name)
	}
	
	return names
}

// CreateMQProvider creates a new MQ provider instance using registered factory
func CreateMQProvider(providerName string, config map[string]interface{}) (MQProvider, error) {
	factory, err := GetMQProvider(providerName)
	if err != nil {
		return nil, err
	}
	
	return factory.CreateProvider(config)
}

// UnregisterMQProvider removes a MQ provider factory from registry
func UnregisterMQProvider(name string) {
	globalMQRegistry.mu.Lock()
	defer globalMQRegistry.mu.Unlock()
	
	delete(globalMQRegistry.providers, name)
}

// MockMQProvider for testing purposes - this is the only concrete implementation
type MockMQProvider struct {
	name         string
	config       map[string]interface{}
	initialized  bool
	messages     []*MQMessage
	batches      []*MQMessageBatch
	publishError error
	stats        MockMQStats
	mu           sync.RWMutex
}

type MockMQStats struct {
	PublishCalls     int64
	BatchCalls       int64
	MessagesReceived int64
	LastMessage      *MQMessage
}

// MockMQProviderFactory creates mock MQ providers for testing
type MockMQProviderFactory struct {
	name        string
	description string
}

func NewMockMQProviderFactory(name, description string) *MockMQProviderFactory {
	return &MockMQProviderFactory{
		name:        name,
		description: description,
	}
}

func (f *MockMQProviderFactory) CreateProvider(config map[string]interface{}) (MQProvider, error) {
	return &MockMQProvider{
		name:     f.name,
		config:   config,
		messages: make([]*MQMessage, 0),
		batches:  make([]*MQMessageBatch, 0),
	}, nil
}

func (f *MockMQProviderFactory) Name() string {
	return f.name
}

func (f *MockMQProviderFactory) Description() string {
	return f.description
}

// MockMQProvider implementation
func (m *MockMQProvider) Initialize(config map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.config = config
	m.initialized = true
	return nil
}

func (m *MockMQProvider) Publish(ctx context.Context, topic string, message *MQMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if !m.initialized {
		return fmt.Errorf("mock provider not initialized")
	}
	
	if m.publishError != nil {
		return m.publishError
	}
	
	// Make a copy of the message to avoid race conditions
	msgCopy := *message
	m.messages = append(m.messages, &msgCopy)
	m.stats.PublishCalls++
	m.stats.MessagesReceived++
	m.stats.LastMessage = &msgCopy
	
	return nil
}

func (m *MockMQProvider) PublishBatch(ctx context.Context, batches []*MQMessageBatch) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if !m.initialized {
		return fmt.Errorf("mock provider not initialized")
	}
	
	if m.publishError != nil {
		return m.publishError
	}
	
	// Make copies of batches to avoid race conditions
	for _, batch := range batches {
		batchCopy := &MQMessageBatch{
			Topic:    batch.Topic,
			Messages: make([]*MQMessage, len(batch.Messages)),
		}
		
		for i, msg := range batch.Messages {
			msgCopy := *msg
			batchCopy.Messages[i] = &msgCopy
		}
		
		m.batches = append(m.batches, batchCopy)
		m.stats.MessagesReceived += int64(len(batch.Messages))
		
		if len(batch.Messages) > 0 {
			lastMsg := *batch.Messages[len(batch.Messages)-1]
			m.stats.LastMessage = &lastMsg
		}
	}
	
	m.stats.BatchCalls++
	return nil
}

func (m *MockMQProvider) Ping(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if !m.initialized {
		return fmt.Errorf("mock provider not initialized")
	}
	return nil
}

func (m *MockMQProvider) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.initialized = false
	return nil
}

func (m *MockMQProvider) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return map[string]interface{}{
		"provider_type":      m.name,
		"publish_calls":      m.stats.PublishCalls,
		"batch_calls":        m.stats.BatchCalls,
		"messages_received":  m.stats.MessagesReceived,
		"messages_count":     len(m.messages),
		"batches_count":      len(m.batches),
		"initialized":        m.initialized,
		"config":            m.config,
	}
}

func (m *MockMQProvider) Name() string {
	return m.name
}

// Testing helper methods
func (m *MockMQProvider) SetPublishError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.publishError = err
}

func (m *MockMQProvider) GetMessages() []*MQMessage {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	messages := make([]*MQMessage, len(m.messages))
	copy(messages, m.messages)
	return messages
}

func (m *MockMQProvider) GetBatches() []*MQMessageBatch {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	batches := make([]*MQMessageBatch, len(m.batches))
	copy(batches, m.batches)
	return batches
}

func (m *MockMQProvider) GetMessageCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.messages)
}

func (m *MockMQProvider) GetBatchCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.batches)
}

func (m *MockMQProvider) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.messages = make([]*MQMessage, 0)
	m.batches = make([]*MQMessageBatch, 0)
	m.publishError = nil
	m.stats = MockMQStats{}
}

func init() {
	// Register the mock provider factory for testing
	mockFactory := NewMockMQProviderFactory("mock", "Mock MQ provider for testing")
	RegisterMQProvider("mock", mockFactory)
}