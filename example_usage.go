package libai

import (
	"errors"
	"fmt"
	"log"
	"time"
)

func main() {
	fmt.Println("=== Libai Chain-Style Logging Library Demo ===")
	fmt.Println()
	
	// Test 1: Legacy Logger (Backward Compatibility)
	fmt.Println("=== 1. Legacy Logger (Existing API) ===")
	testLegacyLogger()
	
	// Test 2: New Chain Logger
	fmt.Println("\n=== 2. New Chain Logger API ===")
	testChainLogger()
	
	// Test 3: Using Legacy API through Chain Logger
	fmt.Println("\n=== 3. Legacy API via Chain Logger ===")
	testLegacyWrapperFromChainLogger()
	
	// Test 4: JSON Formatting
	fmt.Println("\n=== 4. JSON Formatting ===")
	testJSONFormatting()
	
	// Test 5: Advanced Chain Features
	fmt.Println("\n=== 5. Advanced Chain Features ===")
	testAdvancedChainFeatures()
	
	// Test 6: Zap Integration Demo
	fmt.Println("\n=== 6. Zap Integration Demo ===")
	DemoZapIntegration()
}

func testLegacyLogger() {
	// Create legacy logger using existing API
	logger, err := NewLoggerBuilder().
		SetOrigin("legacy-app").
		SetDebugMode().
		SetStackTrace(true).
		Build()

	if err != nil {
		log.Fatal("Failed to create legacy logger:", err)
	}

	// Test different log levels (existing API)
	logger.Info("user_login", "success", map[string]interface{}{
		"user_id": 12345,
		"ip":      "192.168.1.1",
	})

	logger.Warn("performance_warning", "slow_query", map[string]interface{}{
		"query_time": "2.5s",
		"query":      "SELECT * FROM users",
	})

	// Test Zap wrapper
	zapLogger := logger.Z()
	zapLogger.Info("zap_info", "test", "This is zap-style info")
}

func testChainLogger() {
	// Create new chain logger
	chainLogger, err := NewLoggerBuilder().
		SetOrigin("chain-app").
		SetDebugMode().
		SetStackTrace(true).
		BuildChain()

	if err != nil {
		log.Fatal("Failed to create chain logger:", err)
	}
	defer chainLogger.Close()

	// Basic chain API usage
	chainLogger.Info().
		Msg("User logged in successfully").
		Str("user", "john_doe").
		Int("user_id", 12345).
		Str("ip", "192.168.1.1").
		Time("login_time", time.Now()).
		Log()

	// Chain with error
	chainLogger.Error().
		Msg("Database connection failed").
		Str("host", "localhost:27017").
		Err(errors.New("connection timeout")).
		Int("retry_count", 3).
		Log()

	// Chain with different field types
	chainLogger.Warn().
		Msg("High memory usage detected").
		Float64("memory_usage_mb", 512.75).
		Bool("auto_cleanup", true).
		Dur("uptime", 2*time.Hour+30*time.Minute).
		Any("metadata", map[string]string{"service": "api", "version": "1.0"}).
		Log()

	// Debug with caller info
	chainLogger.Debug().
		Msg("Processing request").
		Str("endpoint", "/api/users").
		Str("method", "GET").
		Caller().
		Log()
}

func testLegacyWrapperFromChainLogger() {
	// Create chain logger and get legacy wrapper
	chainLogger, err := NewLoggerBuilder().
		SetOrigin("hybrid-app").
		SetDebugMode().
		BuildChain()

	if err != nil {
		log.Fatal("Failed to create chain logger:", err)
	}
	defer chainLogger.Close()

	// Use legacy API through chain logger
	legacyLogger := chainLogger.Legacy()
	
	legacyLogger.Info("user_action", "page_view", map[string]interface{}{
		"page":    "/dashboard",
		"user_id": 67890,
	})

	legacyLogger.Error("api_error", "validation_failed", map[string]interface{}{
		"field": "email",
		"value": "invalid-email",
	})

	// Use zap wrapper
	zapWrapper := legacyLogger.Z()
	zapWrapper.Warn("system_warning", "disk_space", "Disk usage at 85%")
}

func testJSONFormatting() {
	// Create config with JSON formatting
	config := DefaultConfig()
	config.Origin = "json-app"
	config.Console.Format = "json"
	config.Console.Colorized = false // JSON output shouldn't be colorized

	chainLogger, err := NewChainLogger(config)
	if err != nil {
		log.Fatal("Failed to create JSON logger:", err)
	}
	defer chainLogger.Close()

	chainLogger.Info().
		Msg("JSON formatted log entry").
		Str("environment", "production").
		Int("request_id", 12345).
		Bool("success", true).
		Log()

	chainLogger.Error().
		Msg("JSON error log").
		Err(errors.New("something went wrong")).
		Str("component", "user-service").
		Log()
}

func testAdvancedChainFeatures() {
	chainLogger, err := NewLoggerBuilder().
		SetOrigin("advanced-app").
		SetDebugMode().
		SetStackTrace(true).
		BuildChain()

	if err != nil {
		log.Fatal("Failed to create advanced logger:", err)
	}
	defer chainLogger.Close()

	// Context with predefined fields
	userContext := chainLogger.WithFields(map[string]interface{}{
		"user_id":    12345,
		"session_id": "abc123",
		"ip":         "192.168.1.100",
	})

	// Log with context
	userContext.
		Info().
		Msg("User performed action").
		Str("action", "file_upload").
		Str("filename", "document.pdf").
		Int64("file_size", 1024*1024*5). // 5MB
		Log()

	// Chain with stack trace
	chainLogger.Error().
		Msg("Critical error occurred").
		Str("component", "payment-processor").
		Err(errors.New("payment gateway timeout")).
		Stack().
		Log()

	// Different log levels with filtering
	chainLogger.SetLevel(WarnLevel) // Only warn and above will be logged

	chainLogger.Debug().Msg("This won't be logged").Log()                     // Filtered out
	chainLogger.Info().Msg("This won't be logged either").Log()               // Filtered out
	chainLogger.Warn().Msg("This will be logged").Log()                       // Will log
	chainLogger.Error().Msg("This will definitely be logged").Log()           // Will log

	// Reset to debug level
	chainLogger.SetLevel(DebugLevel)

	// Using With() for single field context
	chainLogger.With().
		With("request_id", "req-789").
		Info().
		Msg("Request processed").
		Str("endpoint", "/api/data").
		Dur("duration", 150*time.Millisecond).
		Log()
}

// Demonstrate creating logger without MongoDB
func createSimpleLogger() {
	// Simple console-only logger
	logger, err := NewLoggerBuilder().
		SetOrigin("simple-app").
		BuildChain()

	if err != nil {
		log.Fatal("Failed to create simple logger:", err)
	}
	defer logger.Close()

	logger.Info().Msg("Simple logger example").Log()
}

// Demonstrate error handling
func demonstrateErrorHandling() {
	// Try to create logger with invalid config
	_, err := NewLoggerBuilder().BuildChain() // Missing origin

	if err != nil {
		fmt.Printf("Expected error: %v\n", err)
	}
}
