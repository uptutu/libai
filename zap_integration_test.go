package libai

import (
	"fmt"
	"testing"
	"time"
)

func TestZapIntegration(t *testing.T) {
	// Test console output with zap
	t.Run("ConsoleOutput", func(t *testing.T) {
		config := DefaultConfig()
		config.Origin = "test-app"
		config.Level = DebugLevel // Enable debug level
		config.Console.Enabled = true
		config.Console.Format = "text"
		config.Console.Colorized = true

		logger, err := NewChainLogger(config)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}

		// Test different log levels
		logger.Debug().Msg("Debug message").Str("key", "debug").Log()
		logger.Info().Msg("Info message").Str("key", "info").Int("count", 42).Log()
		logger.Warn().Msg("Warning message").Str("key", "warn").Bool("flag", true).Log()
		logger.Error().Msg("Error message").Str("key", "error").Float64("value", 3.14).Log()

		if err := logger.Close(); err != nil {
			t.Errorf("Failed to close logger: %v", err)
		}
	})

	// Test JSON console output
	t.Run("ConsoleJSONOutput", func(t *testing.T) {
		config := DefaultConfig()
		config.Origin = "test-app-json"
		config.Console.Enabled = true
		config.Console.Format = "json"
		config.Console.Colorized = false

		logger, err := NewChainLogger(config)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}

		logger.Info().
			Msg("JSON formatted message").
			Str("service", "test").
			Int("user_id", 123).
			Time("timestamp", time.Now()).
			Any("metadata", map[string]string{"key": "value"}).
			Log()

		if err := logger.Close(); err != nil {
			t.Errorf("Failed to close logger: %v", err)
		}
	})

	// Test file output with zap
	t.Run("FileOutput", func(t *testing.T) {
		config := DefaultConfig()
		config.Origin = "test-app-file"
		config.Console.Enabled = false
		config.File.Enabled = true
		config.File.Filename = "/tmp/test-libai.log"
		config.File.Format = "json"

		logger, err := NewChainLogger(config)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}

		logger.Info().
			Msg("File output test").
			Str("output", "file").
			Int("test_id", 456).
			Log()

		if err := logger.Close(); err != nil {
			t.Errorf("Failed to close logger: %v", err)
		}
	})

	// Test with context fields
	t.Run("WithContextFields", func(t *testing.T) {
		config := DefaultConfig()
		config.Origin = "test-app-context"
		config.Console.Enabled = true

		logger, err := NewChainLogger(config)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}

		// Test With and WithFields
		logger.WithFields(map[string]interface{}{
			"session_id": "sess_123",
			"user_id":    789,
		}).Info().Msg("User action").Str("action", "login").Log()

		if err := logger.Close(); err != nil {
			t.Errorf("Failed to close logger: %v", err)
		}
	})

	// Test error logging
	t.Run("ErrorLogging", func(t *testing.T) {
		config := DefaultConfig()
		config.Origin = "test-app-error"
		config.Console.Enabled = true
		config.EnableStack = true

		logger, err := NewChainLogger(config)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}

		testErr := fmt.Errorf("test error message")

		logger.Error().
			Msg("Something went wrong").
			Err(testErr).
			Str("component", "database").
			Stack().
			Caller().
			Log()

		if err := logger.Close(); err != nil {
			t.Errorf("Failed to close logger: %v", err)
		}
	})

	// Test backward compatibility
	t.Run("BackwardCompatibility", func(t *testing.T) {
		config := DefaultConfig()
		config.Origin = "test-app-legacy"
		config.Console.Enabled = true

		chainLogger, err := NewChainLogger(config)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}

		legacyLogger := chainLogger.Legacy()

		// Test legacy API still works
		legacyLogger.Info("user_login", "success", map[string]interface{}{
			"user_id": 999,
			"ip":      "192.168.1.1",
		})

		legacyLogger.Error("database_error", "connection_failed", map[string]interface{}{
			"error": "timeout",
			"host":  "localhost",
		})

		if err := chainLogger.Close(); err != nil {
			t.Errorf("Failed to close logger: %v", err)
		}
	})
}

func TestZapFormats(t *testing.T) {
	// Test text format
	t.Run("TextFormat", func(t *testing.T) {
		config := DefaultConfig()
		config.Origin = "format-test"
		config.Console.Enabled = true
		config.Console.Format = "text"
		config.Console.Colorized = true

		logger, err := NewChainLogger(config)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}

		logger.Info().Msg("Text format test").Str("format", "text").Log()
		logger.Warn().Msg("Text format warning").Str("format", "text").Log()
		logger.Error().Msg("Text format error").Str("format", "text").Log()

		logger.Close()
	})

	// Test JSON format
	t.Run("JSONFormat", func(t *testing.T) {
		config := DefaultConfig()
		config.Origin = "format-test"
		config.Console.Enabled = true
		config.Console.Format = "json"

		logger, err := NewChainLogger(config)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}

		logger.Info().Msg("JSON format test").Str("format", "json").Log()
		logger.Warn().Msg("JSON format warning").Str("format", "json").Log()
		logger.Error().Msg("JSON format error").Str("format", "json").Log()

		logger.Close()
	})
}
