package libai

import (
	"testing"
	"time"
)

func TestChainLoggerCreation(t *testing.T) {
	config := DefaultConfig()
	config.Origin = "test-app"

	logger, err := NewChainLogger(config)
	if err != nil {
		t.Fatalf("Failed to create chain logger: %v", err)
	}

	if logger == nil {
		t.Fatal("Logger should not be nil")
	}

	// Clean up
	if err := logger.Close(); err != nil {
		t.Errorf("Failed to close logger: %v", err)
	}
}

func TestLogLevels(t *testing.T) {
	config := DefaultConfig()
	config.Origin = "test-app"
	config.Level = DebugLevel

	logger, err := NewChainLogger(config)
	if err != nil {
		t.Fatalf("Failed to create chain logger: %v", err)
	}
	defer logger.Close()

	// Test that each log level returns a valid chain
	debugChain := logger.Debug()
	if debugChain == nil {
		t.Error("Debug chain should not be nil")
	}

	infoChain := logger.Info()
	if infoChain == nil {
		t.Error("Info chain should not be nil")
	}

	warnChain := logger.Warn()
	if warnChain == nil {
		t.Error("Warn chain should not be nil")
	}

	errorChain := logger.Error()
	if errorChain == nil {
		t.Error("Error chain should not be nil")
	}

	fatalChain := logger.Fatal()
	if fatalChain == nil {
		t.Error("Fatal chain should not be nil")
	}
}

func TestChainMethods(t *testing.T) {
	config := DefaultConfig()
	config.Origin = "test-app"
	config.Console.Enabled = true

	logger, err := NewChainLogger(config)
	if err != nil {
		t.Fatalf("Failed to create chain logger: %v", err)
	}
	defer logger.Close()

	// Test chaining methods
	chain := logger.Info().
		Msg("Test message").
		Str("string_field", "test_value").
		Int("int_field", 42).
		Bool("bool_field", true).
		Float64("float_field", 3.14).
		Time("time_field", time.Now())

	if chain == nil {
		t.Error("Chain should not be nil after method calls")
	}

	// Test that Log() doesn't panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Log() should not panic: %v", r)
		}
	}()

	chain.Log()
}

func TestLogEntry(t *testing.T) {
	entry := NewLogEntry()

	if entry == nil {
		t.Fatal("NewLogEntry should not return nil")
	}

	if entry.Fields == nil {
		t.Error("Fields should be initialized")
	}

	if entry.Targets == nil {
		t.Error("Targets should be initialized")
	}

	if entry.Timestamp.IsZero() {
		t.Error("Timestamp should be set")
	}
}

func TestLogEntryCopy(t *testing.T) {
	original := NewLogEntry()
	original.Level = ErrorLevel
	original.Message = "test message"
	original.Fields["key"] = "value"
	original.Origin = "test-origin"

	copy := original.Copy()

	if copy == nil {
		t.Fatal("Copy should not return nil")
	}

	if copy == original {
		t.Error("Copy should not be the same instance")
	}

	if copy.Level != original.Level {
		t.Error("Level should be copied")
	}

	if copy.Message != original.Message {
		t.Error("Message should be copied")
	}

	if copy.Origin != original.Origin {
		t.Error("Origin should be copied")
	}

	if copy.Fields["key"] != "value" {
		t.Error("Fields should be copied")
	}

	// Test that modifying copy doesn't affect original
	copy.Fields["new_key"] = "new_value"
	if _, exists := original.Fields["new_key"]; exists {
		t.Error("Modifying copy should not affect original")
	}
}

func TestLogLevelParsing(t *testing.T) {
	tests := []struct {
		input    string
		expected LogLevel
		hasError bool
	}{
		{"debug", DebugLevel, false},
		{"info", InfoLevel, false},
		{"warn", WarnLevel, false},
		{"error", ErrorLevel, false},
		{"fatal", FatalLevel, false},
		{"invalid", InfoLevel, true},
	}

	for _, test := range tests {
		level, err := ParseLogLevel(test.input)

		if test.hasError {
			if err == nil {
				t.Errorf("Expected error for input %s", test.input)
			}
		} else {
			if err != nil {
				t.Errorf("Unexpected error for input %s: %v", test.input, err)
			}
			if level != test.expected {
				t.Errorf("Expected %v for input %s, got %v", test.expected, test.input, level)
			}
		}
	}
}

func TestLogLevelFiltering(t *testing.T) {
	config := DefaultConfig()
	config.Origin = "test-app"
	config.Level = WarnLevel // Only warn and above should log

	logger, err := NewChainLogger(config)
	if err != nil {
		t.Fatalf("Failed to create chain logger: %v", err)
	}
	defer logger.Close()

	// Debug and Info should return NoOpLogChain
	debugChain := logger.Debug()
	infoChain := logger.Info()

	// Check if they're NoOpLogChain by calling methods and ensuring they don't panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("NoOpLogChain methods should not panic: %v", r)
		}
	}()

	debugChain.Msg("debug message").Log()
	infoChain.Msg("info message").Log()

	// Warn and Error should work normally
	logger.Warn().Msg("warn message").Log()
	logger.Error().Msg("error message").Log()
}

func TestBackwardCompatibilityBuilder(t *testing.T) {
	// Test that existing LoggerBuilder.Build() still works
	builder := NewLoggerBuilder().
		SetOrigin("test-app").
		SetDebugMode()

	legacyLogger, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to create legacy logger: %v", err)
	}

	if legacyLogger == nil {
		t.Fatal("Legacy logger should not be nil")
	}

	// Test that new BuildChain method works
	chainLogger, err := builder.BuildChain()
	if err != nil {
		t.Fatalf("Failed to create chain logger: %v", err)
	}

	if chainLogger == nil {
		t.Fatal("Chain logger should not be nil")
	}

	chainLogger.Close()
}

func TestLegacyWrapper(t *testing.T) {
	config := DefaultConfig()
	config.Origin = "test-app"
	config.Console.Enabled = true

	chainLogger, err := NewChainLogger(config)
	if err != nil {
		t.Fatalf("Failed to create chain logger: %v", err)
	}
	defer chainLogger.Close()

	// Get legacy wrapper
	legacyLogger := chainLogger.Legacy()
	if legacyLogger == nil {
		t.Fatal("Legacy wrapper should not be nil")
	}

	// Test legacy methods don't panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Legacy methods should not panic: %v", r)
		}
	}()

	legacyLogger.Info("test_action", "test_flag", "test content")
	legacyLogger.Warn("test_action", "test_flag", map[string]string{"key": "value"})
	legacyLogger.Error("test_action", "test_flag", 42)
	legacyLogger.Debug("test_action", "test_flag", true)

	// Test Z() wrapper
	zapWrapper := legacyLogger.Z()
	if zapWrapper == nil {
		t.Error("Z() should not return nil")
	}

	zapWrapper.Info("test_action", "test_flag", "zap test")
}
