package libai

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFileOutputEnhancements(t *testing.T) {
	// Test enhanced file output with rotation
	t.Run("FileOutputWithRotation", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "libai_test_")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		logFile := filepath.Join(tempDir, "test_rotation.log")

		config := DefaultConfig()
		config.Origin = "test-file-rotation"
		config.Console.Enabled = false
		config.File.Enabled = true
		config.File.Filename = logFile
		config.File.Format = "json"
		config.File.MaxSize = 1024 // 1KB for testing
		config.File.MaxAge = 1     // 1 day
		config.File.MaxBackups = 3
		config.File.Compress = true
		config.File.LocalTime = true

		logger, err := NewChainLogger(config)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}

		// Write multiple entries to trigger rotation
		for i := 0; i < 100; i++ {
			logger.Info().
				Msgf("Test log entry number %d", i).
				Str("entry_id", fmt.Sprintf("entry_%d", i)).
				Int("iteration", i).
				Time("timestamp", time.Now()).
				Any("data", map[string]interface{}{
					"test":   "data",
					"number": i,
					"array":  []string{"a", "b", "c"},
				}).
				Log()
		}

		if err := logger.Close(); err != nil {
			t.Errorf("Failed to close logger: %v", err)
		}

		// Check if log file was created
		if _, err := os.Stat(logFile); os.IsNotExist(err) {
			t.Errorf("Log file was not created: %s", logFile)
		}

		// Check if the file contains valid JSON logs
		content, err := os.ReadFile(logFile)
		if err != nil {
			t.Fatalf("Failed to read log file: %v", err)
		}

		if len(content) == 0 {
			t.Error("Log file is empty")
		}

		// Verify JSON format by checking for expected fields
		logContent := string(content)
		if !strings.Contains(logContent, `"level":"info"`) {
			t.Error("Log file doesn't contain expected level field")
		}
		if !strings.Contains(logContent, `"origin":"test-file-rotation"`) {
			t.Error("Log file doesn't contain expected origin field")
		}
	})

	// Test different file formats
	t.Run("FileOutputFormats", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "libai_formats_")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Test JSON format
		t.Run("JSONFormat", func(t *testing.T) {
			logFile := filepath.Join(tempDir, "test.json.log")
			config := DefaultConfig()
			config.Origin = "json-test"
			config.Console.Enabled = false
			config.File.Enabled = true
			config.File.Filename = logFile
			config.File.Format = "json"

			logger, _ := NewChainLogger(config)
			logger.Info().Msg("JSON format test").Str("format", "json").Log()
			logger.Close()

			content, _ := os.ReadFile(logFile)
			contentStr := string(content)
			if !strings.Contains(contentStr, `"message":"JSON format test"`) {
				t.Errorf("JSON format not working correctly. Content: %s", contentStr)
			}
		})

		// Test console/text format
		t.Run("TextFormat", func(t *testing.T) {
			logFile := filepath.Join(tempDir, "test.text.log")
			config := DefaultConfig()
			config.Origin = "text-test"
			config.Console.Enabled = false
			config.File.Enabled = true
			config.File.Filename = logFile
			config.File.Format = "text"

			logger, _ := NewChainLogger(config)
			logger.Info().Msg("Text format test").Str("format", "text").Log()
			logger.Close()

			content, _ := os.ReadFile(logFile)
			contentStr := string(content)
			if !strings.Contains(contentStr, "Text format test") {
				t.Error("Text format not working correctly")
			}
		})
	})

	// Test file output configuration
	t.Run("FileOutputConfiguration", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "libai_config_")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		config := DefaultConfig()
		config.File.Enabled = true
		config.File.Filename = filepath.Join(tempDir, "config_test.log")
		config.File.MaxSize = 10 * 1024 * 1024 // 10MB
		config.File.MaxAge = 30                // 30 days
		config.File.MaxBackups = 10
		config.File.Compress = false
		config.File.LocalTime = false

		logger, err := NewChainLogger(config)
		if err != nil {
			t.Fatalf("Failed to create logger with custom config: %v", err)
		}

		// Verify we can get the file output plugin
		if len(logger.(*ChainLoggerImpl).outputs) == 0 {
			t.Error("No output plugins configured")
		}

		fileOutput, exists := logger.(*ChainLoggerImpl).outputs[FileOutput]
		if !exists {
			t.Error("File output plugin not configured")
		}

		zapFileOutput, ok := fileOutput.(*ZapFileOutputPlugin)
		if !ok {
			t.Error("File output plugin is not ZapFileOutputPlugin")
		}

		if zapFileOutput.Name() != "file" {
			t.Error("File output plugin name incorrect")
		}

		logger.Close()
	})

	// Test log levels with file output
	t.Run("LogLevelsFileOutput", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "libai_levels_")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		logFile := filepath.Join(tempDir, "levels.log")
		config := DefaultConfig()
		config.Origin = "level-test"
		config.Level = DebugLevel
		config.Console.Enabled = false
		config.File.Enabled = true
		config.File.Filename = logFile
		config.File.Format = "json"

		logger, _ := NewChainLogger(config)

		logger.Debug().Msg("Debug message").Log()
		logger.Info().Msg("Info message").Log()
		logger.Warn().Msg("Warn message").Log()
		logger.Error().Msg("Error message").Log()

		logger.Close()

		content, _ := os.ReadFile(logFile)
		logContent := string(content)

		// Check all levels are present
		levels := []string{"debug", "info", "warn", "error"}
		for _, level := range levels {
			if !strings.Contains(logContent, fmt.Sprintf(`"level":"%s"`, level)) {
				t.Errorf("Level %s not found in log file", level)
			}
		}
	})

	// Test file output with fields and context
	t.Run("FileOutputWithFields", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "libai_fields_")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		logFile := filepath.Join(tempDir, "fields.log")
		config := DefaultConfig()
		config.Origin = "fields-test"
		config.Console.Enabled = false
		config.File.Enabled = true
		config.File.Filename = logFile
		config.File.Format = "json"

		logger, _ := NewChainLogger(config)

		// Test with context
		contextLogger := logger.WithFields(map[string]interface{}{
			"session_id": "sess_123",
			"user_id":    456,
		})

		contextLogger.Info().
			Msg("User action").
			Str("action", "login").
			Bool("success", true).
			Time("timestamp", time.Now()).
			Log()

		logger.Close()

		content, _ := os.ReadFile(logFile)
		logContent := string(content)

		// Check context fields
		if !strings.Contains(logContent, `"session_id":"sess_123"`) {
			t.Error("Context session_id not found")
		}
		if !strings.Contains(logContent, `"user_id":456`) {
			t.Error("Context user_id not found")
		}
		if !strings.Contains(logContent, `"action":"login"`) {
			t.Error("Chain field action not found")
		}
		if !strings.Contains(logContent, `"success":true`) {
			t.Error("Chain field success not found")
		}
	})
}

func TestFileRotationUtilities(t *testing.T) {
	// Test file rotation utilities
	t.Run("FilePluginUtilities", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "libai_utils_")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		logFile := filepath.Join(tempDir, "utils_test.log")
		config := DefaultConfig()
		config.Console.Enabled = false
		config.File.Enabled = true
		config.File.Filename = logFile

		logger, _ := NewChainLogger(config)

		// Get the file output plugin
		fileOutput := logger.(*ChainLoggerImpl).outputs[FileOutput].(*ZapFileOutputPlugin)

		// Test GetCurrentLogFile
		currentFile := fileOutput.GetCurrentLogFile()
		if currentFile != logFile {
			t.Errorf("Expected current file %s, got %s", logFile, currentFile)
		}

		// Test manual rotation (should not error)
		if err := fileOutput.Rotate(); err != nil {
			t.Errorf("Manual rotation failed: %v", err)
		}

		logger.Close()
	})
}