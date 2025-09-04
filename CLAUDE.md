# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is `libai`, a Go logging library that provides structured logging with multiple output destinations. The library now supports both a modern fluent/chain-style API and maintains full backward compatibility with the original logger interface. 

Key features:
- **Chain-style API**: Modern fluent interface like `logger.Info().Msg("test").Str("key", "value").Log()`
- **Multiple outputs**: Console, File, Database (MongoDB), Message Queue support
- **Zap integration**: Uses uber-go/zap as the underlying logging engine for console and file outputs
- **Backward compatibility**: Existing Logger API works unchanged
- **High performance**: Object pooling, async processing, level filtering

## Key Dependencies

- **MongoDB Driver**: `go.mongodb.org/mongo-driver v1.17.4` - For database persistence 
- **Zap Logger**: `go.uber.org/zap v1.27.0` - High-performance structured logging engine
- **Go Version**: 1.25.0

## Common Development Commands

```bash
# Build the library
go build .

# Run all tests
go test -v

# Run specific test groups
go test -v -run TestZapIntegration
go test -v -run TestChainLogger

# Format code
go fmt ./...

# Vet code for common mistakes
go vet ./...

# Get dependencies
go mod tidy

# Run complete demo
go run demo/main.go

# View example usage patterns
go run example_usage.go
```

## Architecture Overview

### New Chain-Style Architecture (v2)

1. **ChainLogger Interface** (`interfaces.go:8-27`): Main interface providing fluent API entry points
2. **LogChain Interface** (`interfaces.go:29-60`): Chainable methods for building log entries
3. **ChainLoggerImpl** (`chain_logger.go:12-21`): Core implementation with zap integration
4. **Output Plugin System** (`formatters.go`): Extensible output destinations using zap

### Legacy Architecture (v1 - Maintained)

1. **Logger** (`logger.go:26-35`): Original logger with MongoDB support
2. **LoggerBuilder** (`logger.go:148-160`): Builder pattern for configuration  
3. **LegacyWrapper** (`legacy_wrapper.go`): Adapter between old and new APIs

### Key Design Patterns

- **Fluent Interface**: Chain-style API for readable log construction
- **Plugin Architecture**: Extensible output system with zap backends
- **Adapter Pattern**: LegacyWrapper maintains API compatibility
- **Object Pooling**: Memory-efficient log entry reuse
- **Level Filtering**: NoOpLogChain for disabled levels (zero overhead)

### Output System (Zap-Powered)

**Console Output** (`formatters.go:11-66`):
- Text format with colors (development)
- JSON format (production)  
- Smart stderr/stdout routing
- Zap console encoder integration

**File Output** (`formatters.go:136-265`):
- JSON and text formats
- Zap file writer integration
- Configurable rotation (future)

**Database Output**: MongoDB persistence (existing, to be enhanced)
**Message Queue Output**: Async logging to MQ systems (future)

### Logging Levels & Performance

- **Debug/Info/Warn/Error/Fatal**: Standard log levels
- **Level Filtering**: Compile-time optimization with NoOpLogChain
- **Stack Traces**: Automatic for Error/Fatal levels when enabled
- **Caller Information**: Optional caller context

## Development Patterns

### Chain-Style API (Recommended)

```go
// Create chain logger
config := DefaultConfig()
config.Origin = "my-service"
logger, _ := NewChainLogger(config)

// Basic logging
logger.Info().Msg("User logged in").Str("user", "john").Int("id", 123).Log()

// Context logging  
sessionLogger := logger.WithFields(map[string]interface{}{
    "session_id": "sess_123",
    "request_id": "req_456",
})
sessionLogger.Error().Msg("Database error").Err(dbErr).Stack().Log()

// Output targeting
logger.Error().Msg("Critical error").ToConsole().ToDB().Log()
```

### Legacy API (Backward Compatible)

```go
// Original builder pattern still works
logger, _ := NewLoggerBuilder().
    SetOrigin("legacy-app").
    SetDebugMode().
    Build()

logger.Info("user_login", "success", map[string]interface{}{
    "user_id": 12345,
    "ip": "192.168.1.1",
})

// Or use legacy wrapper from chain logger
chainLogger, _ := NewChainLogger(config)
legacyLogger := chainLogger.Legacy()
legacyLogger.Error("db_error", "connection_failed", errorData)
```

### Configuration

```go
config := DefaultConfig()
config.Origin = "service-name"
config.Level = DebugLevel
config.EnableStack = true

// Console output (zap-powered)
config.Console.Enabled = true
config.Console.Format = "text"  // or "json"
config.Console.Colorized = true
config.Console.TimeFormat = time.RFC3339

// File output (zap-powered)  
config.File.Enabled = true
config.File.Filename = "app.log"
config.File.Format = "json"
config.File.TimeFormat = time.RFC3339
```

## File Structure

**Core Implementation:**
- `chain_logger.go`: Main chain logger implementation
- `interfaces.go`: Core interfaces (ChainLogger, LogChain, OutputPlugin)
- `types.go`: Type definitions (LogLevel, LogEntry, DatabaseDriver)
- `config.go`: Configuration system and object pooling

**Output System:**
- `formatters.go`: Zap-powered console and file output plugins
- `legacy_wrapper.go`: Backward compatibility adapter

**Original System (Maintained):**
- `logger.go`: Original logger with enhanced BuildChain() method

**Testing & Examples:**
- `*_test.go`: Comprehensive test suites  
- `zap_integration_test.go`: Zap-specific functionality tests
- `example_usage.go`: Usage patterns and demos
- `zap_demo.go`: Zap integration demonstration
- `demo/main.go`: Standalone demo application

## Testing Strategy

**Test Coverage Areas:**
- Chain API functionality (`chain_logger_test.go`)
- Zap integration (`zap_integration_test.go`)  
- Legacy compatibility (`TestLegacyWrapper`)
- Output plugins and formatting
- Object pooling and memory management
- Level filtering and performance
- Error handling and edge cases

**Running Tests:**
```bash
# All tests
go test -v

# Specific test groups
go test -v -run TestZapIntegration
go test -v -run TestChainLogger

# With coverage
go test -cover -v
```

## Performance Characteristics

**Optimizations:**
- **Object Pooling**: Reuses LogEntry and LogChain objects
- **Level Filtering**: NoOpLogChain eliminates disabled level overhead
- **Zap Backend**: High-performance structured logging
- **Smart Allocation**: Minimal memory allocation in hot paths
- **Concurrent Safe**: Thread-safe with efficient locking

**Memory Management:**
- Log entries recycled via sync.Pool
- Context fields shared between loggers  
- Efficient field storage and serialization

## Module Information

- **Module path**: `local.git/libs/libai.git`
- **Go version**: 1.25.0
- **Private module**: Local development only
- **Dependencies**: MongoDB driver, Zap logger

## Migration Guide

**From v1 to v2 (Chain API):**
1. Existing code works unchanged (100% compatible)
2. Gradually adopt chain style for new code
3. Use `BuildChain()` instead of `Build()` for new instances
4. Consider `WithFields()` for contextual logging

**Key Benefits of Migration:**
- Better readability with fluent interface
- Type-safe field methods  
- Enhanced output targeting
- Improved performance with zap backend