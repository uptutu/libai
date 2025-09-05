package libai

import (
	"context"
	"fmt"
	"time"
)

// DemoMessageQueueOutput demonstrates the message queue output capabilities
func DemoMessageQueueOutput() {
	fmt.Println("=== Message Queue Output Demo ===")
	
	// Demo 1: Basic MQ Output with Mock Provider
	fmt.Println("\n1. Basic MQ Output with Mock Provider:")
	demoBasicMQOutput()
	
	// Demo 2: Custom Provider Registration
	fmt.Println("\n2. Custom Provider Registration:")
	demoCustomProviderRegistration()
	
	// Demo 3: Message Routing Strategies
	fmt.Println("\n3. Message Routing Strategies:")
	demoMessageRouting()
	
	// Demo 4: Batch Processing and Performance
	fmt.Println("\n4. Batch Processing and Performance:")
	demoBatchProcessing()
	
	// Demo 5: Provider Failover
	fmt.Println("\n5. Provider Failover:")
	demoProviderFailover()
	
	fmt.Println("\n=== Message Queue Output Demo Complete ===")
}

// demoBasicMQOutput demonstrates basic MQ functionality
func demoBasicMQOutput() {
	config := DefaultConfig()
	config.Origin = "mq-demo"
	config.Console.Enabled = false // Disable console for cleaner demo
	config.MessageQueue.Enabled = true
	config.MessageQueue.Provider = "mock"
	config.MessageQueue.Topic = "demo_logs"
	config.MessageQueue.Format = "json"
	config.MessageQueue.BatchSize = 3
	config.MessageQueue.WorkerCount = 2
	config.MessageQueue.FlushInterval = 1000 // 1 second
	
	logger, err := NewChainLogger(config)
	if err != nil {
		fmt.Printf("❌ Failed to create MQ logger: %v\n", err)
		return
	}
	defer logger.Close()
	
	fmt.Printf("✅ MQ logger created successfully\n")
	fmt.Printf("   - Provider: %s\n", config.MessageQueue.Provider)
	fmt.Printf("   - Topic: %s\n", config.MessageQueue.Topic)
	fmt.Printf("   - Batch Size: %d\n", config.MessageQueue.BatchSize)
	fmt.Printf("   - Workers: %d\n", config.MessageQueue.WorkerCount)
	
	// Log various types of messages
	logger.Info().
		Msg("Application started").
		Str("version", "1.0.0").
		Str("environment", "demo").
		ToMQ("app_events").
		Log()
	
	logger.Info().
		Msg("User authentication").
		Str("user_id", "user_123").
		Str("method", "oauth2").
		Bool("success", true).
		Log()
	
	logger.Warn().
		Msg("High CPU usage detected").
		Float64("cpu_percent", 85.5).
		Str("component", "worker").
		Int("threshold", 80).
		ToMQ("alerts").
		Log()
	
	logger.Error().
		Msg("Database connection timeout").
		Str("host", "db.example.com").
		Dur("timeout", 30*time.Second).
		Int("retry_count", 3).
		Log()
	
	// Wait for message processing
	time.Sleep(500 * time.Millisecond)
	
	// Get stats from the MQ output plugin
	mqOutput := logger.(*ChainLoggerImpl).outputs[MessageQueueOutput]
	if mqOutput != nil {
		stats := mqOutput.GetStats()
		fmt.Printf("📊 MQ Output Stats:\n")
		fmt.Printf("   - Messages Sent: %v\n", stats["messages_sent"])
		fmt.Printf("   - Messages Queued: %v\n", stats["messages_queued"])
		fmt.Printf("   - Worker Count: %v\n", stats["worker_count"])
		
		// Show provider-specific stats
		for key, value := range stats {
			if fmt.Sprintf("%v", key)[:9] == "provider_" {
				fmt.Printf("   - %s: %v\n", key, value)
			}
		}
	}
	
	fmt.Println("✅ Basic MQ output completed")
}

// demoCustomProviderRegistration demonstrates registering custom providers
func demoCustomProviderRegistration() {
	fmt.Println("Available MQ Providers:")
	providers := ListMQProviders()
	for i, provider := range providers {
		factory, _ := GetMQProvider(provider)
		fmt.Printf("  %d. %s - %s\n", i+1, provider, factory.Description())
	}
	
	// Register a custom mock provider
	customFactory := NewMockMQProviderFactory("custom-demo", "Custom demo provider")
	err := RegisterMQProvider("custom-demo", customFactory)
	if err != nil {
		fmt.Printf("❌ Failed to register custom provider: %v\n", err)
		return
	}
	
	fmt.Printf("✅ Registered custom provider: custom-demo\n")
	
	// Test the custom provider
	config := MessageQueueConfig{
		Enabled:       true,
		Provider:      "custom-demo",
		Topic:         "custom_logs",
		Format:        "json",
		BatchSize:     2,
		BufferSize:    5,
		WorkerCount:   1,
		FlushInterval: 500,
		Config: map[string]interface{}{
			"custom_param": "demo_value",
			"timeout":      30,
		},
		Routing: RoutingConfig{
			Strategy:    "default",
			TopicPrefix: "libai",
		},
	}
	
	plugin, err := NewMessageQueueOutputPlugin(config)
	if err != nil {
		fmt.Printf("❌ Failed to create custom provider plugin: %v\n", err)
		UnregisterMQProvider("custom-demo")
		return
	}
	
	fmt.Printf("✅ Custom provider plugin created successfully\n")
	
	// Test message publishing with custom provider
	entry := NewLogEntry()
	entry.Level = InfoLevel
	entry.Message = "Custom provider test message"
	entry.Origin = "custom-demo"
	entry.Fields["test"] = "custom_provider"
	
	ctx := context.Background()
	err = plugin.Write(ctx, entry)
	if err != nil {
		fmt.Printf("❌ Failed to write to custom provider: %v\n", err)
	} else {
		fmt.Printf("✅ Message sent to custom provider\n")
	}
	
	time.Sleep(200 * time.Millisecond)
	
	// Show stats
	stats := plugin.GetStats()
	fmt.Printf("📊 Custom Provider Stats: %+v\n", stats)
	
	plugin.Close()
	UnregisterMQProvider("custom-demo")
	fmt.Printf("✅ Custom provider demo completed\n")
}

// demoMessageRouting demonstrates different routing strategies
func demoMessageRouting() {
	fmt.Println("Testing different message routing strategies...")
	
	// Default routing
	fmt.Println("\n- Default Routing Strategy:")
	testRouting(RoutingConfig{
		Strategy:    "default",
		TopicPrefix: "app",
		Partitions:  3,
	})
	
	// Level-based routing
	fmt.Println("\n- Level-based Routing Strategy:")
	testRouting(RoutingConfig{
		Strategy:    "level",
		TopicPrefix: "logs",
		LevelTopics: map[string]string{
			"error": "error_logs",
			"warn":  "warning_logs",
			"info":  "info_logs",
		},
	})
	
	// Field-based routing
	fmt.Println("\n- Field-based Routing Strategy:")
	testRouting(RoutingConfig{
		Strategy: "field",
		FieldRules: []FieldRoutingRule{
			{
				Field:     "service",
				Value:     "auth",
				Topic:     "auth_service_logs",
				Condition: "equals",
			},
			{
				Field:     "service",
				Value:     "api",
				Topic:     "api_service_logs",
				Condition: "equals",
			},
		},
	})
}

func testRouting(routingConfig RoutingConfig) {
	// Create router
	router, err := NewMessageRouter(routingConfig)
	if err != nil {
		fmt.Printf("❌ Failed to create router: %v\n", err)
		return
	}
	
	// Test different log entries
	testEntries := []*LogEntry{
		{
			Level:   InfoLevel,
			Message: "Info message",
			Origin:  "test-app",
			Fields:  map[string]interface{}{"service": "auth"},
		},
		{
			Level:   ErrorLevel,
			Message: "Error message",
			Origin:  "test-app",
			Fields:  map[string]interface{}{"service": "api"},
		},
		{
			Level:   WarnLevel,
			Message: "Warning message",
			Origin:  "test-app",
			Fields:  map[string]interface{}{"component": "database"},
		},
	}
	
	for i, entry := range testEntries {
		message, err := router.Route(entry, "default_topic")
		if err != nil {
			fmt.Printf("  ❌ Failed to route message %d: %v\n", i+1, err)
			continue
		}
		
		fmt.Printf("  ✅ Message %d -> Topic: %s, Key: %s\n", 
			i+1, message.Topic, message.Key)
		
		// Show partition assignment
		partition := router.GetPartition(message, routingConfig.Partitions)
		fmt.Printf("     Partition: %d\n", partition)
	}
}

// demoBatchProcessing demonstrates batch processing and performance
func demoBatchProcessing() {
	config := MessageQueueConfig{
		Enabled:       true,
		Provider:      "mock",
		Topic:         "batch_demo",
		Format:        "json",
		BatchSize:     10,  // Process in batches of 10
		BufferSize:    50,  // Large buffer
		WorkerCount:   3,   // Multiple workers
		FlushInterval: 200, // Fast flush
		Config:        map[string]interface{}{},
		Routing: RoutingConfig{
			Strategy:    "default",
			TopicPrefix: "batch",
		},
	}
	
	plugin, err := NewMessageQueueOutputPlugin(config)
	if err != nil {
		fmt.Printf("❌ Failed to create batch plugin: %v\n", err)
		return
	}
	defer plugin.Close()
	
	fmt.Printf("✅ Batch processing plugin created\n")
	fmt.Printf("   - Batch Size: %d\n", config.BatchSize)
	fmt.Printf("   - Buffer Size: %d\n", config.BufferSize)
	fmt.Printf("   - Workers: %d\n", config.WorkerCount)
	
	start := time.Now()
	messageCount := 50
	
	// Send many messages quickly
	ctx := context.Background()
	for i := 0; i < messageCount; i++ {
		entry := NewLogEntry()
		entry.Level = InfoLevel
		entry.Message = fmt.Sprintf("Batch message %d", i)
		entry.Origin = "batch-demo"
		entry.Fields["batch_id"] = "batch_001"
		entry.Fields["message_index"] = i
		entry.Fields["timestamp"] = time.Now().Unix()
		entry.Fields["data"] = map[string]interface{}{
			"processing_id": fmt.Sprintf("proc_%d", i),
			"category":      "batch_processing",
			"priority":      "normal",
		}
		
		err = plugin.Write(ctx, entry)
		if err != nil {
			fmt.Printf("❌ Failed to write message %d: %v\n", i, err)
			break
		}
	}
	
	duration := time.Since(start)
	fmt.Printf("📊 Generated %d messages in %v (%.2f msgs/sec)\n",
		messageCount, duration, float64(messageCount)/duration.Seconds())
	
	// Wait for processing
	time.Sleep(1 * time.Second)
	
	// Show final stats
	stats := plugin.GetStats()
	fmt.Printf("📊 Final Batch Processing Stats:\n")
	fmt.Printf("   - Messages Sent: %v\n", stats["messages_sent"])
	fmt.Printf("   - Messages Queued: %v\n", stats["messages_queued"])
	fmt.Printf("   - Messages Failed: %v\n", stats["messages_failed"])
	fmt.Printf("   - Batches Sent: %v\n", stats["batches_sent"])
	
	fmt.Println("✅ Batch processing demo completed")
}

// demoProviderFailover demonstrates provider failover functionality
func demoProviderFailover() {
	// Register primary and fallback providers
	primaryFactory := NewMockMQProviderFactory("primary", "Primary provider")
	fallbackFactory := NewMockMQProviderFactory("fallback", "Fallback provider")
	
	RegisterMQProvider("primary", primaryFactory)
	RegisterMQProvider("fallback", fallbackFactory)
	defer func() {
		UnregisterMQProvider("primary")
		UnregisterMQProvider("fallback")
	}()
	
	config := MessageQueueConfig{
		Enabled:       true,
		Provider:      "primary",
		Topic:         "failover_test",
		Format:        "json",
		BatchSize:     3,
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
				Config:   map[string]interface{}{"role": "fallback"},
			},
		},
	}
	
	plugin, err := NewMessageQueueOutputPlugin(config)
	if err != nil {
		fmt.Printf("❌ Failed to create failover plugin: %v\n", err)
		return
	}
	defer plugin.Close()
	
	fmt.Printf("✅ Failover plugin created with primary and fallback providers\n")
	
	// Get access to providers for testing
	mqPlugin := plugin.(*MessageQueueOutputPlugin)
	primaryProvider := mqPlugin.providers["primary"].(*MockMQProvider)
	fallbackProvider := mqPlugin.providers["fallback"].(*MockMQProvider)
	
	fmt.Printf("   - Primary Provider: %s\n", primaryProvider.Name())
	fmt.Printf("   - Fallback Provider: %s\n", fallbackProvider.Name())
	
	ctx := context.Background()
	
	// Send normal messages (should go to primary)
	fmt.Println("\n📤 Sending normal messages (should use primary):")
	for i := 0; i < 3; i++ {
		entry := NewLogEntry()
		entry.Level = InfoLevel
		entry.Message = fmt.Sprintf("Normal message %d", i)
		entry.Origin = "failover-test"
		
		err = plugin.Write(ctx, entry)
		if err != nil {
			fmt.Printf("❌ Failed to write normal message %d: %v\n", i, err)
		}
	}
	
	time.Sleep(200 * time.Millisecond)
	
	fmt.Printf("   - Primary messages: %d\n", primaryProvider.GetMessageCount())
	fmt.Printf("   - Fallback messages: %d\n", fallbackProvider.GetMessageCount())
	
	// Simulate primary provider failure
	fmt.Println("\n🔥 Simulating primary provider failure:")
	primaryProvider.SetPublishError(fmt.Errorf("simulated primary failure"))
	
	// Send more messages (should failover to fallback)
	fmt.Println("📤 Sending messages with primary failed (should use fallback):")
	for i := 0; i < 2; i++ {
		entry := NewLogEntry()
		entry.Level = ErrorLevel
		entry.Message = fmt.Sprintf("Failover message %d", i)
		entry.Origin = "failover-test"
		entry.Fields["failover"] = true
		
		err = plugin.Write(ctx, entry)
		if err != nil {
			fmt.Printf("❌ Failed to write failover message %d: %v\n", i, err)
		}
	}
	
	time.Sleep(200 * time.Millisecond)
	
	fmt.Printf("   - Primary messages: %d\n", primaryProvider.GetMessageCount())
	fmt.Printf("   - Fallback messages: %d\n", fallbackProvider.GetMessageCount())
	
	// Show stats
	stats := plugin.GetStats()
	fmt.Printf("\n📊 Failover Demo Stats:\n")
	fmt.Printf("   - Messages Sent: %v\n", stats["messages_sent"])
	fmt.Printf("   - Messages Failed: %v\n", stats["messages_failed"])
	
	fmt.Println("✅ Provider failover demo completed")
}

// DemoMQProviderImplementation shows how to implement a custom MQ provider
func DemoMQProviderImplementation() {
	fmt.Println("\n=== Custom MQ Provider Implementation Demo ===")
	
	fmt.Println(`
To implement a custom MQ provider for libai, you need to:

1. Implement the MQProvider interface:
   type CustomProvider struct {
       // your fields
   }
   
   func (p *CustomProvider) Initialize(config map[string]interface{}) error {
       // Initialize your MQ client/connection
       return nil
   }
   
   func (p *CustomProvider) Publish(ctx context.Context, topic string, message *MQMessage) error {
       // Publish single message to your MQ system
       return nil
   }
   
   func (p *CustomProvider) PublishBatch(ctx context.Context, batches []*MQMessageBatch) error {
       // Publish batch of messages (optional, can fallback to single publishes)
       return nil
   }
   
   func (p *CustomProvider) Ping(ctx context.Context) error {
       // Health check for your MQ system
       return nil
   }
   
   func (p *CustomProvider) Close() error {
       // Cleanup resources
       return nil
   }
   
   func (p *CustomProvider) GetStats() map[string]interface{} {
       // Return provider-specific statistics
       return map[string]interface{}{"status": "connected"}
   }
   
   func (p *CustomProvider) Name() string {
       return "custom"
   }

2. Implement the MQProviderFactory interface:
   type CustomProviderFactory struct{}
   
   func (f *CustomProviderFactory) CreateProvider(config map[string]interface{}) (MQProvider, error) {
       return &CustomProvider{}, nil
   }
   
   func (f *CustomProviderFactory) Name() string {
       return "custom"
   }
   
   func (f *CustomProviderFactory) Description() string {
       return "Custom MQ provider description"
   }

3. Register your provider:
   factory := &CustomProviderFactory{}
   err := RegisterMQProvider("custom", factory)
   if err != nil {
       // handle error
   }

4. Use your provider in configuration:
   config.MessageQueue.Provider = "custom"
   config.MessageQueue.Config = map[string]interface{}{
       "broker_url": "your-mq-url",
       "auth_token": "your-token",
   }

Example providers you could implement:
- Kafka (using Sarama or Confluent Go client)
- RabbitMQ (using amqp091-go)
- Redis Streams (using go-redis)
- Apache Pulsar
- NATS
- AWS SQS/SNS
- Google Pub/Sub
- Azure Service Bus

The interface is designed to be simple yet flexible enough for any MQ system!`)
	
	fmt.Println("=== Custom MQ Provider Demo Complete ===")
}