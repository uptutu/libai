package libai

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// DemoFileOutputEnhancements demonstrates the enhanced file output capabilities
func DemoFileOutputEnhancements() {
	fmt.Println("=== File Output Enhancement Demo ===")

	// Create a temporary directory for demo logs
	tempDir, err := os.MkdirTemp("", "libai_file_demo_")
	if err != nil {
		fmt.Printf("Failed to create temp directory: %v\n", err)
		return
	}
	defer os.RemoveAll(tempDir)

	fmt.Printf("Demo logs will be written to: %s\n", tempDir)

	// Demo 1: Basic file output with JSON format
	fmt.Println("\n1. Basic JSON File Output:")
	jsonConfig := DefaultConfig()
	jsonConfig.Origin = "json-file-demo"
	jsonConfig.Console.Enabled = false
	jsonConfig.File.Enabled = true
	jsonConfig.File.Filename = filepath.Join(tempDir, "app.json.log")
	jsonConfig.File.Format = "json"
	jsonConfig.File.TimeFormat = time.RFC3339

	jsonLogger, _ := NewChainLogger(jsonConfig)

	jsonLogger.Info().
		Msg("Application started").
		Str("version", "1.0.0").
		Int("port", 8080).
		Bool("debug", false).
		Time("start_time", time.Now()).
		Log()

	jsonLogger.Warn().
		Msg("High memory usage detected").
		Float64("memory_usage_mb", 1024.5).
		Str("component", "cache").
		Log()

	jsonLogger.Close()
	showFileContent(filepath.Join(tempDir, "app.json.log"), "JSON")

	// Demo 2: Text format file output
	fmt.Println("\n2. Text Format File Output:")
	textConfig := DefaultConfig()
	textConfig.Origin = "text-file-demo"
	textConfig.Console.Enabled = false
	textConfig.File.Enabled = true
	textConfig.File.Filename = filepath.Join(tempDir, "app.text.log")
	textConfig.File.Format = "text"
	textConfig.File.TimeFormat = "2006-01-02 15:04:05"

	textLogger, _ := NewChainLogger(textConfig)

	textLogger.Info().Msg("User authentication successful").Str("user", "alice").Log()
	textLogger.Error().Msg("Database connection failed").Str("host", "localhost").Int("port", 5432).Log()

	textLogger.Close()
	showFileContent(filepath.Join(tempDir, "app.text.log"), "Text")

	// Demo 3: File rotation configuration
	fmt.Println("\n3. File Output with Rotation Settings:")
	rotationConfig := DefaultConfig()
	rotationConfig.Origin = "rotation-demo"
	rotationConfig.Console.Enabled = false
	rotationConfig.File.Enabled = true
	rotationConfig.File.Filename = filepath.Join(tempDir, "rotating.log")
	rotationConfig.File.Format = "json"
	rotationConfig.File.MaxSize = 1024 * 10 // 10KB for demo
	rotationConfig.File.MaxAge = 7          // 7 days
	rotationConfig.File.MaxBackups = 3      // Keep 3 old files
	rotationConfig.File.Compress = true     // Compress rotated files
	rotationConfig.File.LocalTime = true    // Use local time

	rotLogger, _ := NewChainLogger(rotationConfig)

	fmt.Printf("Configuration:\n")
	fmt.Printf("  - Max Size: %d bytes (%.1f KB)\n", rotationConfig.File.MaxSize, float64(rotationConfig.File.MaxSize)/1024)
	fmt.Printf("  - Max Age: %d days\n", rotationConfig.File.MaxAge)
	fmt.Printf("  - Max Backups: %d files\n", rotationConfig.File.MaxBackups)
	fmt.Printf("  - Compression: %v\n", rotationConfig.File.Compress)
	fmt.Printf("  - Local Time: %v\n", rotationConfig.File.LocalTime)

	// Generate some logs to potentially trigger rotation
	for i := 0; i < 50; i++ {
		rotLogger.Info().
			Msgf("Processing batch %d", i).
			Str("batch_id", fmt.Sprintf("batch_%03d", i)).
			Int("items_count", i*10).
			Time("processed_at", time.Now()).
			Any("metadata", map[string]interface{}{
				"source":      "batch_processor",
				"priority":    "high",
				"retry_count": 0,
				"tags":        []string{"processing", "batch", "data"},
			}).
			Log()
	}

	rotLogger.Close()
	showFileContent(filepath.Join(tempDir, "rotating.log"), "Rotation Demo")

	// Demo 4: Context logging with file output
	fmt.Println("\n4. Context Logging to File:")
	contextConfig := DefaultConfig()
	contextConfig.Origin = "context-demo"
	contextConfig.Console.Enabled = false
	contextConfig.File.Enabled = true
	contextConfig.File.Filename = filepath.Join(tempDir, "context.log")
	contextConfig.File.Format = "json"

	contextLogger, _ := NewChainLogger(contextConfig)

	// Create session-specific logger with context
	sessionLogger := contextLogger.WithFields(map[string]interface{}{
		"session_id": "sess_abc123",
		"user_id":    12345,
		"request_id": "req_xyz789",
		"ip_address": "192.168.1.100",
		"user_agent": "libai-demo/1.0",
	})

	sessionLogger.Info().Msg("User session started").Str("action", "login").Log()
	sessionLogger.Info().Msg("Page accessed").Str("page", "/dashboard").Time("access_time", time.Now()).Log()
	sessionLogger.Warn().Msg("Slow query detected").Str("query", "SELECT * FROM users").Dur("duration", 2*time.Second).Log()
	sessionLogger.Info().Msg("User session ended").Str("action", "logout").Log()

	contextLogger.Close()
	showFileContent(filepath.Join(tempDir, "context.log"), "Context Logging")

	fmt.Println("\n=== Demo Complete ===")
	fmt.Printf("All demo log files are available in: %s\n", tempDir)
}

// showFileContent displays the content of a log file
func showFileContent(filename, title string) {
	content, err := os.ReadFile(filename)
	if err != nil {
		fmt.Printf("Failed to read %s: %v\n", filename, err)
		return
	}

	fmt.Printf("%s file content (first 500 chars):\n", title)
	contentStr := string(content)
	if len(contentStr) > 500 {
		contentStr = contentStr[:500] + "..."
	}
	fmt.Printf("%s\n", contentStr)

	// Show file info
	if info, err := os.Stat(filename); err == nil {
		fmt.Printf("File size: %d bytes\n", info.Size())
	}
}
