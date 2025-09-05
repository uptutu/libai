# Libai - Enterprise Go Logging Library

[![Go Version](https://img.shields.io/badge/Go-1.25+-blue.svg)](https://golang.org)
[![Status](https://img.shields.io/badge/Status-Production%20Ready-brightgreen.svg)](https://github.com)
[![License](https://img.shields.io/badge/License-Private-red.svg)](https://github.com)

**Libai** is a comprehensive, high-performance Go logging library that combines modern fluent/chain-style APIs with enterprise-grade features. Built on top of uber-go/zap, it provides structured logging with multiple output destinations, async processing, and advanced reliability features.

## ✨ Key Features

- 🔗 **Modern Chain-Style API**: Fluent interface for readable, type-safe logging
- 🎯 **Multiple Output Destinations**: Console, File, Database (MongoDB), Message Queue
- ⚡ **High Performance**: Built on uber-go/zap with object pooling and async processing
- 🔄 **100% Backward Compatible**: Existing v1 API works unchanged
- 🚀 **Enterprise Features**: Load balancing, circuit breakers, health monitoring
- 📊 **Advanced Message Queue**: Extensible provider architecture with routing strategies
- 🛡️ **Reliability**: Fault tolerance, automatic recovery, and comprehensive monitoring
- 🏎️ **Optimized Performance**: Zero-copy paths, memory pooling, concurrent safety

## 🚀 Quick Start

### Installation

```bash
# Clone the repository (private module)
git clone <repository-url>
cd libai

# Initialize module
go mod tidy
```

### Basic Usage

```go
package main

import (
    "local.git/libs/libai.git"
)

func main() {
    // Create chain logger
    config := libai.DefaultConfig()
    config.Origin = "my-app"
    logger, _ := libai.NewChainLogger(config)
    defer logger.Close()

    // Chain-style logging
    logger.Info().
        Msg("User logged in").
        Str("user_id", "12345").
        Str("ip", "192.168.1.1").
        Int("session_duration", 3600).
        Log()

    // Error logging with stack trace
    logger.Error().
        Msg("Database connection failed").
        Err(err).
        Stack().
        Log()
}
```

## 📖 Documentation

### Chain-Style API Examples

```go
// Basic logging with fields
logger.Info().
    Msg("Processing request").
    Str("method", "POST").
    Str("path", "/api/users").
    Int("status", 200).
    Dur("duration", time.Since(start)).
    Log()

// Context-aware logging
userLogger := logger.WithFields(map[string]interface{}{
    "user_id": "12345",
    "session_id": "sess_abc123",
})

userLogger.Warn().
    Msg("Rate limit approaching").
    Int("requests_count", 95).
    Int("limit", 100).
    Log()

// Targeted output
logger.Error().
    Msg("Critical system error").
    ToConsole().  // Always show in console
    ToDB().       // Store in database
    ToMQ("alerts"). // Send to alerts queue
    Log()
```

### Legacy API (Backward Compatible)

```go
// Existing v1 code works unchanged
logger, _ := libai.NewLoggerBuilder().
    SetOrigin("legacy-app").
    SetDebugMode().
    Build()

logger.Info("user_action", "login", map[string]interface{}{
    "user_id": 12345,
    "method": "password",
})

// Or use legacy wrapper from chain logger
chainLogger, _ := libai.NewChainLogger(config)
legacyLogger := chainLogger.Legacy()
legacyLogger.Error("db_error", "connection_failed", errorData)
```

## ⚙️ Configuration

### Complete Configuration Example

```go
config := libai.DefaultConfig()
config.Origin = "my-service"
config.Level = libai.DebugLevel
config.EnableStack = true

// Console output (Zap-powered)
config.Console.Enabled = true
config.Console.Format = "text"  // or "json"
config.Console.Colorized = true

// File output with rotation (Lumberjack)
config.File.Enabled = true
config.File.Filename = "app.log"
config.File.MaxSize = 100    // MB
config.File.MaxBackups = 5
config.File.MaxAge = 30      // days
config.File.Compress = true

// Database output (MongoDB)
config.Database.Enabled = true
config.Database.Driver = libai.MongoDB
config.Database.ConnectionString = "mongodb://localhost:27017"
config.Database.DatabaseName = "logs"

// Message Queue output
config.MessageQueue.Enabled = true
config.MessageQueue.Provider = "kafka"  // or "rabbitmq", "mock"
config.MessageQueue.Topic = "app-logs"
config.MessageQueue.BatchSize = 10
config.MessageQueue.WorkerCount = 3
```

### Enterprise Configuration

```go
// Enable async processing
config.AsyncDispatcher.Enabled = true
config.AsyncDispatcher.BufferSize = 1000
config.AsyncDispatcher.WorkerCount = 5

// Load balancing
config.LoadBalancer.Strategy = "round_robin"
config.LoadBalancer.HealthCheckInterval = 5000

// Circuit breaker
config.CircuitBreaker.Enabled = true
config.CircuitBreaker.FailureThreshold = 5
config.CircuitBreaker.RecoveryTimeout = 30000
```

## 🏗️ Architecture

### Core Components

- **ChainLogger**: Main fluent API interface
- **OutputPlugin**: Extensible output system (Console, File, Database, MQ)
- **AsyncOutputDispatcher**: Enterprise async processing with worker pools
- **LoadBalancer**: Multi-strategy load balancing
- **CircuitBreaker**: Fault tolerance and isolation
- **HealthMonitor**: Real-time component health tracking

### Output Plugins

1. **Console Output**: Zap-powered with colorization and JSON support
2. **File Output**: Lumberjack integration with automatic rotation
3. **Database Output**: MongoDB with connection pooling
4. **Message Queue Output**: Extensible provider architecture

### Message Queue Features

- **Provider Architecture**: Pluggable MQ implementations (Kafka, RabbitMQ, etc.)
- **Routing Strategies**: Default, level-based, field-based routing
- **Batch Processing**: Configurable batch sizes and worker pools
- **Failover Support**: Automatic provider switching on failures
- **Health Monitoring**: Real-time provider health checking

## 🧪 Testing

```bash
# Run all tests
go test -v

# Run specific test suites
go test -v -run TestChainLogger
go test -v -run TestMQOutput
go test -v -run TestDatabaseOutput
go test -v -run TestAsyncDispatcher

# Performance benchmarks
go test -bench=. -benchmem

# Test coverage
go test -cover -v
```

## 📊 Performance

### Benchmarks

```
BenchmarkChainLogger-8           1000000    1050 ns/op    248 B/op    3 allocs/op
BenchmarkAsyncDispatcher-8       2000000     850 ns/op    184 B/op    2 allocs/op
BenchmarkObjectPool-8           10000000     105 ns/op      0 B/op    0 allocs/op
```

### Performance Features

- **Object Pooling**: Zero-allocation log entry reuse
- **Async Processing**: Non-blocking log output
- **Level Filtering**: Compile-time optimization
- **Batch Processing**: Reduced I/O overhead
- **Circuit Breaking**: Prevents cascading failures

## 🔧 Examples and Demos

Run the comprehensive demo suite:

```bash
# Main demo (overview of all features)
go run demo/main.go

# Specific feature demos
go run demo/zap_demo.go        # Zap integration
go run demo/mq_demo.go         # Message queue features
go run demo/database_demo.go   # Database output
go run demo/file_demo.go       # File output and rotation
```

## 📋 Migration Guide

### From v1 to v2

**Phase 1: Zero Changes (100% Compatible)**
```go
// Existing code works unchanged
logger, _ := NewLoggerBuilder().SetOrigin("app").Build()
logger.Info("event", "data", fields)
```

**Phase 2: Hybrid Approach**
```go
// Use BuildChain() for new instances
chainLogger, _ := NewLoggerBuilder().SetOrigin("app").BuildChain()

// Mix APIs as needed
legacyLogger := chainLogger.Legacy()
legacyLogger.Info("old_style", "message", data)

chainLogger.Info().Msg("New style").Str("key", "value").Log()
```

**Phase 3: Full Chain API**
```go
config := DefaultConfig()
logger, _ := NewChainLogger(config)

logger.Info().
    Msg("Fully migrated").
    Str("api", "v2").
    Bool("ready", true).
    Log()
```

## 🛠️ Extending Libai

### Custom MQ Provider

```go
// Implement MQProvider interface
type CustomProvider struct{}

func (p *CustomProvider) Initialize(config map[string]interface{}) error {
    // Initialize your MQ client
    return nil
}

func (p *CustomProvider) Publish(ctx context.Context, topic string, message *MQMessage) error {
    // Publish to your MQ system
    return nil
}

// Register provider
factory := &CustomProviderFactory{}
libai.RegisterMQProvider("custom", factory)

// Use in configuration
config.MessageQueue.Provider = "custom"
```

### Custom Output Plugin

```go
// Implement OutputPlugin interface
type CustomOutput struct{}

func (c *CustomOutput) Write(ctx context.Context, entry *LogEntry) error {
    // Custom output logic
    return nil
}

func (c *CustomOutput) GetStats() map[string]interface{} {
    return map[string]interface{}{"status": "active"}
}
```

## 📄 License

This is a private/internal library. All rights reserved.

## 🤝 Contributing

This is a private module. For internal development guidelines, see `CLAUDE.md`.

## 📞 Support

For questions and support, please refer to the internal documentation and examples provided in the `demo/` directory.

---

**Libai v2.0** - Enterprise Go Logging Library  
*Built for high-performance, reliable, and scalable logging needs.*