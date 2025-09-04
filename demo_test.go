package libai

import (
	"fmt"
	"testing"
)

func TestCompleteDemo(t *testing.T) {
	fmt.Println("=== Libai Chain-Style Logging Library Demo ===")
	fmt.Println()
	
	// Test the new chain API
	chainLogger, err := NewLoggerBuilder().
		SetOrigin("demo-app").
		SetDebugMode().
		BuildChain()

	if err != nil {
		t.Fatalf("Failed to create chain logger: %v", err)
	}
	defer chainLogger.Close()

	// Demonstrate the new chain API
	fmt.Println("=== Chain API Demo ===")
	chainLogger.Info().
		Msg("User logged in successfully").
		Str("user", "john_doe").
		Int("user_id", 12345).
		Log()

	chainLogger.Warn().
		Msg("High memory usage").
		Float64("memory_mb", 512.75).
		Bool("auto_cleanup", true).
		Log()

	// Test legacy compatibility
	fmt.Println("\n=== Legacy API through Chain Logger ===")
	legacyLogger := chainLogger.Legacy()
	legacyLogger.Info("user_action", "login", map[string]interface{}{
		"user": "jane_doe",
		"ip":   "192.168.1.1",
	})
	
	// Test With() method
	fmt.Println("\n=== Context Demo ===")
	chainLogger.With().
		With("request_id", "req-123").
		Info().
		Msg("Request processed").
		Str("endpoint", "/api/users").
		Log()

	fmt.Println("\n=== Demo completed successfully! ===")
}