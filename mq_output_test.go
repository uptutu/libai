package libai

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// TestMQProviderRegistry tests the MQ provider registration system
func TestMQProviderRegistry(t *testing.T) {
	// Test listing providers
	t.Run("ListProviders", func(t *testing.T) {
		providers := ListMQProviders()
		
		// Should include the mock provider
		found := false
		for _, name := range providers {
			if name == "mock" {
				found = true
				break
			}
		}
		
		if !found {
			t.Error("Mock provider should be registered by default")
		}
	})
	
	// Test getting provider
	t.Run("GetProvider", func(t *testing.T) {
		factory, err := GetMQProvider("mock")
		if err != nil {
			t.Fatalf("Failed to get mock provider: %v", err)
		}
		
		if factory.Name() != "mock" {
			t.Errorf("Expected provider name 'mock', got '%s'", factory.Name())
		}
	})
	
	// Test getting non-existent provider
	t.Run("GetNonExistentProvider", func(t *testing.T) {
		_, err := GetMQProvider("nonexistent")
		if err == nil {
			t.Error("Expected error for non-existent provider")
		}
	})
	
	// Test registering custom provider
	t.Run("RegisterCustomProvider", func(t *testing.T) {
		customFactory := NewMockMQProviderFactory("custom", "Custom test provider")
		
		err := RegisterMQProvider("custom", customFactory)
		if err != nil {
			t.Fatalf("Failed to register custom provider: %v", err)
		}
		
		// Verify it was registered
		factory, err := GetMQProvider("custom")
		if err != nil {
			t.Fatalf("Failed to get custom provider: %v", err)
		}
		
		if factory.Name() != "custom" {
			t.Errorf("Expected provider name 'custom', got '%s'", factory.Name())
		}
		
		// Clean up
		UnregisterMQProvider("custom")
	})
	
	// Test duplicate registration
	t.Run("DuplicateRegistration", func(t *testing.T) {
		customFactory := NewMockMQProviderFactory("duplicate", "Duplicate test provider")
		
		// First registration should succeed
		err := RegisterMQProvider("duplicate", customFactory)
		if err != nil {
			t.Fatalf("First registration failed: %v", err)
		}
		
		// Second registration should fail
		err = RegisterMQProvider("duplicate", customFactory)
		if err == nil {
			t.Error("Expected error for duplicate registration")
		}
		
		// Clean up
		UnregisterMQProvider("duplicate")
	})
}

// TestMockMQProvider tests the mock MQ provider functionality
func TestMockMQProvider(t *testing.T) {
	t.Run("BasicFunctionality", func(t *testing.T) {
		factory := NewMockMQProviderFactory("test", "Test provider")
		
		provider, err := factory.CreateProvider(map[string]interface{}{
			"test_param": "test_value",
		})
		if err != nil {
			t.Fatalf("Failed to create provider: %v", err)
		}
		
		mockProvider := provider.(*MockMQProvider)
		
		// Test initialization
		err = provider.Initialize(map[string]interface{}{
			"test_config": "config_value",
		})
		if err != nil {
			t.Fatalf("Failed to initialize provider: %v", err)
		}
		
		// Test ping
		err = provider.Ping(context.Background())
		if err != nil {
			t.Fatalf("Ping failed: %v", err)
		}
		
		// Test publishing
		message := &MQMessage{
			Topic:     "test_topic",
			Key:       "test_key",
			Payload:   []byte("test payload"),
			Timestamp: time.Now(),
		}
		
		err = provider.Publish(context.Background(), "test_topic", message)
		if err != nil {
			t.Fatalf("Publish failed: %v", err)
		}
		
		// Verify message was received
		messages := mockProvider.GetMessages()
		if len(messages) != 1 {
			t.Fatalf("Expected 1 message, got %d", len(messages))
		}
		
		if messages[0].Topic != "test_topic" {
			t.Errorf("Expected topic 'test_topic', got '%s'", messages[0].Topic)
		}
		
		// Test batch publishing
		batch := &MQMessageBatch{
			Topic: "batch_topic",
			Messages: []*MQMessage{
				{
					Topic:   "batch_topic",
					Key:     "key1",
					Payload: []byte("payload1"),
				},
				{
					Topic:   "batch_topic",
					Key:     "key2",
					Payload: []byte("payload2"),
				},
			},
		}
		
		err = provider.PublishBatch(context.Background(), []*MQMessageBatch{batch})
		if err != nil {
			t.Fatalf("Batch publish failed: %v", err)
		}
		
		// Verify batch was received
		batches := mockProvider.GetBatches()
		if len(batches) != 1 {
			t.Fatalf("Expected 1 batch, got %d", len(batches))
		}
		
		if len(batches[0].Messages) != 2 {
			t.Errorf("Expected 2 messages in batch, got %d", len(batches[0].Messages))
		}
		
		// Test stats
		stats := provider.GetStats()
		if stats["publish_calls"].(int64) != 1 {
			t.Errorf("Expected 1 publish call, got %v", stats["publish_calls"])
		}
		
		if stats["batch_calls"].(int64) != 1 {
			t.Errorf("Expected 1 batch call, got %v", stats["batch_calls"])
		}
		
		// Test close
		err = provider.Close()
		if err != nil {
			t.Fatalf("Close failed: %v", err)
		}
	})
	
	// Test error handling
	t.Run("ErrorHandling", func(t *testing.T) {
		factory := NewMockMQProviderFactory("error_test", "Error test provider")
		provider, _ := factory.CreateProvider(nil)
		mockProvider := provider.(*MockMQProvider)
		
		provider.Initialize(map[string]interface{}{})
		
		// Set publish error
		testError := fmt.Errorf("test publish error")
		mockProvider.SetPublishError(testError)
		
		message := &MQMessage{
			Topic:   "error_topic",
			Payload: []byte("error payload"),
		}
		
		err := provider.Publish(context.Background(), "error_topic", message)
		if err == nil {
			t.Error("Expected publish error")
		}
		
		if err.Error() != testError.Error() {
			t.Errorf("Expected error '%v', got '%v'", testError, err)
		}
		
		// Test batch error
		batch := &MQMessageBatch{
			Topic: "error_batch",
			Messages: []*MQMessage{message},
		}
		
		err = provider.PublishBatch(context.Background(), []*MQMessageBatch{batch})
		if err == nil {
			t.Error("Expected batch publish error")
		}
	})
}

// TestMessageQueueOutputPlugin tests the MQ output plugin
func TestMessageQueueOutputPlugin(t *testing.T) {
	t.Run("PluginCreation", func(t *testing.T) {
		config := MessageQueueConfig{
			Enabled:       true,
			Provider:      "mock",
			Topic:         "test_logs",
			Format:        "json",
			BatchSize:     5,
			BufferSize:    10,
			WorkerCount:   1,
			FlushInterval: 100, // 100ms for fast testing
			Config:        map[string]interface{}{"test": "value"},
			Routing: RoutingConfig{
				Strategy: "default",
			},
		}
		
		plugin, err := NewMessageQueueOutputPlugin(config)
		if err != nil {
			t.Fatalf("Failed to create MQ plugin: %v", err)
		}
		defer plugin.Close()
		
		if plugin.Name() != "mq_mock" {
			t.Errorf("Expected plugin name 'mq_mock', got '%s'", plugin.Name())
		}
		
		// Test stats
		stats := plugin.GetStats()
		if stats["provider"] != "mock" {
			t.Errorf("Expected provider 'mock', got '%v'", stats["provider"])
		}
		
		if stats["worker_count"] != 1 {
			t.Errorf("Expected 1 worker, got %v", stats["worker_count"])
		}
	})
	
	t.Run("MessagePublishing", func(t *testing.T) {
		config := MessageQueueConfig{
			Enabled:       true,
			Provider:      "mock",
			Topic:         "publish_test",
			Format:        "json",
			BatchSize:     3,
			BufferSize:    5,
			WorkerCount:   1,
			FlushInterval: 50, // 50ms
			Config:        map[string]interface{}{},
			Routing: RoutingConfig{
				Strategy: "default",
			},
		}
		
		plugin, err := NewMessageQueueOutputPlugin(config)
		if err != nil {
			t.Fatalf("Failed to create MQ plugin: %v", err)
		}
		defer plugin.Close()
		
		// Create test log entry
		entry := NewLogEntry()
		entry.Level = InfoLevel
		entry.Message = "Test MQ message"
		entry.Origin = "mq-test"
		entry.Fields["test_field"] = "test_value"
		entry.Fields["count"] = 42
		
		// Write message
		ctx := context.Background()
		err = plugin.Write(ctx, entry)
		if err != nil {
			t.Fatalf("Failed to write message: %v", err)
		}
		
		// Wait for processing
		time.Sleep(100 * time.Millisecond)
		
		// Get provider and check messages
		mqPlugin := plugin.(*MessageQueueOutputPlugin)
		provider := mqPlugin.providers["mock"].(*MockMQProvider)
		
		// Check that message was processed
		stats := plugin.GetStats()
		if stats["messages_queued"].(int64) < 1 && stats["messages_sent"].(int64) < 1 {
			t.Error("Expected at least 1 message to be queued or sent")
		}
		
		// Give more time for async processing
		time.Sleep(200 * time.Millisecond)
		
		messages := provider.GetMessages()
		if len(messages) == 0 {
			t.Error("Expected at least 1 message to be published")
		}
	})
	
	t.Run("BatchProcessing", func(t *testing.T) {
		config := MessageQueueConfig{
			Enabled:       true,
			Provider:      "mock",
			Topic:         "batch_test",
			Format:        "json",
			BatchSize:     2, // Small batch size for testing
			BufferSize:    10,
			WorkerCount:   1,
			FlushInterval: 50, // Fast flush for testing
			Config:        map[string]interface{}{},
			Routing: RoutingConfig{
				Strategy: "default",
			},
		}
		
		plugin, err := NewMessageQueueOutputPlugin(config)
		if err != nil {
			t.Fatalf("Failed to create MQ plugin: %v", err)
		}
		defer plugin.Close()
		
		ctx := context.Background()
		
		// Send multiple messages to trigger batching
		for i := 0; i < 5; i++ {
			entry := NewLogEntry()
			entry.Level = InfoLevel
			entry.Message = fmt.Sprintf("Batch message %d", i)
			entry.Origin = "batch-test"
			entry.Fields["batch_id"] = "batch_001"
			entry.Fields["message_index"] = i
			
			err = plugin.Write(ctx, entry)
			if err != nil {
				t.Fatalf("Failed to write message %d: %v", i, err)
			}
		}
		
		// Wait for batch processing
		time.Sleep(200 * time.Millisecond)
		
		// Check stats
		stats := plugin.GetStats()
		if stats["messages_queued"].(int64) == 0 && stats["messages_sent"].(int64) == 0 {
			t.Error("Expected messages to be processed")
		}
	})
	
	t.Run("ProviderFailover", func(t *testing.T) {
		// Register a second mock provider for fallback testing
		fallbackFactory := NewMockMQProviderFactory("fallback", "Fallback test provider")
		RegisterMQProvider("fallback", fallbackFactory)
		defer UnregisterMQProvider("fallback")
		
		config := MessageQueueConfig{
			Enabled:       true,
			Provider:      "mock",
			Topic:         "failover_test",
			Format:        "json",
			BatchSize:     5,
			BufferSize:    10,
			WorkerCount:   1,
			FlushInterval: 100,
			Config:        map[string]interface{}{},
			Routing: RoutingConfig{
				Strategy: "default",
			},
			Fallbacks: []FallbackProviderConfig{
				{
					Provider: "fallback",
					Config:   map[string]interface{}{"fallback": true},
				},
			},
		}
		
		plugin, err := NewMessageQueueOutputPlugin(config)
		if err != nil {
			t.Fatalf("Failed to create MQ plugin with fallback: %v", err)
		}
		defer plugin.Close()
		
		// Verify both providers are initialized
		mqPlugin := plugin.(*MessageQueueOutputPlugin)
		if len(mqPlugin.providers) != 2 {
			t.Errorf("Expected 2 providers, got %d", len(mqPlugin.providers))
		}
		
		if _, exists := mqPlugin.providers["mock"]; !exists {
			t.Error("Primary provider not found")
		}
		
		if _, exists := mqPlugin.providers["fallback"]; !exists {
			t.Error("Fallback provider not found")
		}
	})
}