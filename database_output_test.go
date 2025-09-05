package libai

import (
	"os"
	"testing"
	"time"
)

// TestDatabaseOutputPlugin tests the database output plugin functionality
func TestDatabaseOutputPlugin(t *testing.T) {
	// Test MongoDB configuration
	t.Run("MongoDBConfiguration", func(t *testing.T) {
		config := DatabaseConfig{
			Enabled:    true,
			Driver:     MongoDB,
			Host:       "localhost",
			Port:       27017,
			Username:   "",
			Password:   "",
			Database:   "test_logs",
			Collection: "test_collection",
			BatchSize:  10,
		}

		plugin, err := NewDatabaseOutputPlugin(config)
		if err != nil {
			t.Logf("MongoDB plugin creation failed (expected if no MongoDB running): %v", err)
			return // Skip test if MongoDB not available
		}
		defer plugin.Close()

		if plugin.Name() != "database_mongodb" {
			t.Errorf("Expected plugin name 'database_mongodb', got '%s'", plugin.Name())
		}

		stats := plugin.GetStats()
		if stats["driver"] != "mongodb" {
			t.Errorf("Expected driver 'mongodb', got '%v'", stats["driver"])
		}
	})

	// Test MySQL configuration
	t.Run("MySQLConfiguration", func(t *testing.T) {
		config := DatabaseConfig{
			Enabled:   true,
			Driver:    MySQL,
			Host:      "localhost",
			Port:      3306,
			Username:  "test",
			Password:  "test",
			Database:  "test_logs",
			Table:     "test_table",
			BatchSize: 5,
		}

		plugin, err := NewDatabaseOutputPlugin(config)
		if err != nil {
			t.Logf("MySQL plugin creation failed (expected if no MySQL running): %v", err)
			return // Skip test if MySQL not available
		}
		defer plugin.Close()

		if plugin.Name() != "database_mysql" {
			t.Errorf("Expected plugin name 'database_mysql', got '%s'", plugin.Name())
		}

		stats := plugin.GetStats()
		if stats["driver"] != "mysql" {
			t.Errorf("Expected driver 'mysql', got '%v'", stats["driver"])
		}
	})

	// Test unsupported driver
	t.Run("UnsupportedDriver", func(t *testing.T) {
		config := DatabaseConfig{
			Enabled: true,
			Driver:  DatabaseDriver(999), // Invalid driver
		}

		_, err := NewDatabaseOutputPlugin(config)
		if err == nil {
			t.Error("Expected error for unsupported driver")
		}
	})
}

// TestDatabaseOutputWithLogger tests database output integration with chain logger
func TestDatabaseOutputWithLogger(t *testing.T) {
	// Test MongoDB integration
	t.Run("MongoDBIntegration", func(t *testing.T) {
		config := DefaultConfig()
		config.Origin = "test-mongo-integration"
		config.Console.Enabled = false
		config.File.Enabled = false
		config.Database.Enabled = true
		config.Database.Driver = MongoDB
		config.Database.Database = "test_integration"
		config.Database.Collection = "test_logs"
		config.Database.BatchSize = 5

		logger, err := NewChainLogger(config)
		if err != nil {
			t.Logf("MongoDB logger creation failed (expected if no MongoDB running): %v", err)
			return // Skip test if MongoDB not available
		}
		defer logger.Close()

		// Test basic logging
		logger.Info().
			Msg("Test MongoDB integration").
			Str("component", "database_test").
			Int("test_id", 123).
			Log()

		// Test logging with database targeting
		logger.Info().
			Msg("Direct MongoDB log").
			Str("target", "mongodb").
			ToDB().
			WithDatabase(MongoDB).
			Log()

		// Give time for batch processing
		time.Sleep(100 * time.Millisecond)
	})

	// Test MySQL integration
	t.Run("MySQLIntegration", func(t *testing.T) {
		config := DefaultConfig()
		config.Origin = "test-mysql-integration"
		config.Console.Enabled = false
		config.File.Enabled = false
		config.Database.Enabled = true
		config.Database.Driver = MySQL
		config.Database.Host = "localhost"
		config.Database.Port = 3306
		config.Database.Username = "root"
		config.Database.Password = "password"
		config.Database.Database = "test_integration"
		config.Database.Table = "test_logs"
		config.Database.BatchSize = 3

		logger, err := NewChainLogger(config)
		if err != nil {
			t.Logf("MySQL logger creation failed (expected if no MySQL running): %v", err)
			return // Skip test if MySQL not available
		}
		defer logger.Close()

		// Test basic logging
		logger.Warn().
			Msg("Test MySQL integration").
			Str("component", "database_test").
			Int("warning_code", 456).
			Log()

		// Test error logging with database targeting
		logger.Error().
			Msg("Database error test").
			Str("error_type", "connection").
			ToDB().
			WithDatabase(MySQL).
			Log()

		// Give time for batch processing
		time.Sleep(100 * time.Millisecond)
	})
}

// TestDatabaseOutputBatching tests batch processing functionality
func TestDatabaseOutputBatching(t *testing.T) {
	t.Run("BatchProcessing", func(t *testing.T) {
		config := DefaultConfig()
		config.Origin = "batch-test"
		config.Console.Enabled = false
		config.File.Enabled = false
		config.Database.Enabled = true
		config.Database.Driver = MongoDB
		config.Database.Database = "batch_test"
		config.Database.Collection = "batch_logs"
		config.Database.BatchSize = 3 // Small batch size for testing

		logger, err := NewChainLogger(config)
		if err != nil {
			t.Logf("Batch test logger creation failed (expected if no MongoDB running): %v", err)
			return // Skip test if MongoDB not available
		}
		defer logger.Close()

		// Log multiple entries to trigger batching
		for i := 0; i < 10; i++ {
			logger.Info().
				Msgf("Batch test entry %d", i).
				Str("batch_id", "batch_001").
				Int("entry_number", i).
				Log()
		}

		// Wait for batch processing
		time.Sleep(200 * time.Millisecond)

		// Verify that the database plugin exists and has processed entries
		dbOutput := logger.(*ChainLoggerImpl).outputs[DatabaseOutput]
		if dbOutput == nil {
			t.Error("Database output plugin not found")
			return
		}

		stats := dbOutput.GetStats()
		t.Logf("Database plugin stats: %+v", stats)
	})
}

// TestDatabaseOutputContext tests context logging with database output
func TestDatabaseOutputContext(t *testing.T) {
	t.Run("ContextLogging", func(t *testing.T) {
		config := DefaultConfig()
		config.Origin = "context-test"
		config.Console.Enabled = false
		config.File.Enabled = false
		config.Database.Enabled = true
		config.Database.Driver = MongoDB
		config.Database.Database = "context_test"
		config.Database.Collection = "context_logs"

		logger, err := NewChainLogger(config)
		if err != nil {
			t.Logf("Context test logger creation failed (expected if no MongoDB running): %v", err)
			return // Skip test if MongoDB not available
		}
		defer logger.Close()

		// Create context logger with session information
		sessionLogger := logger.WithFields(map[string]interface{}{
			"session_id": "sess_12345",
			"user_id":    789,
			"ip_address": "192.168.1.100",
		})

		// Log with context
		sessionLogger.Info().
			Msg("User login successful").
			Str("action", "login").
			Bool("success", true).
			Time("login_time", time.Now()).
			Log()

		sessionLogger.Warn().
			Msg("Password strength warning").
			Str("reason", "weak_password").
			Int("strength_score", 2).
			Log()

		// Wait for processing
		time.Sleep(100 * time.Millisecond)
	})
}

// TestDatabaseOutputLevels tests different log levels with database output
func TestDatabaseOutputLevels(t *testing.T) {
	t.Run("LogLevels", func(t *testing.T) {
		config := DefaultConfig()
		config.Origin = "levels-test"
		config.Level = DebugLevel // Enable all levels
		config.Console.Enabled = false
		config.File.Enabled = false
		config.Database.Enabled = true
		config.Database.Driver = MongoDB
		config.Database.Database = "levels_test"
		config.Database.Collection = "level_logs"

		logger, err := NewChainLogger(config)
		if err != nil {
			t.Logf("Levels test logger creation failed (expected if no MongoDB running): %v", err)
			return // Skip test if MongoDB not available
		}
		defer logger.Close()

		// Test all log levels
		logger.Debug().Msg("Debug message for testing").Str("level", "debug").Log()
		logger.Info().Msg("Info message for testing").Str("level", "info").Log()
		logger.Warn().Msg("Warning message for testing").Str("level", "warn").Log()
		logger.Error().Msg("Error message for testing").Str("level", "error").Log()

		// Wait for processing
		time.Sleep(100 * time.Millisecond)
	})
}

// TestDatabaseOutputHealthCheck tests database connection health checking
func TestDatabaseOutputHealthCheck(t *testing.T) {
	t.Run("HealthCheck", func(t *testing.T) {
		config := DatabaseConfig{
			Enabled:    true,
			Driver:     MongoDB,
			Host:       "localhost",
			Port:       27017,
			Database:   "health_test",
			Collection: "test_logs",
		}

		plugin, err := NewDatabaseOutputPlugin(config)
		if err != nil {
			t.Logf("Health check plugin creation failed (expected if no MongoDB running): %v", err)
			return // Skip test if MongoDB not available
		}
		defer plugin.Close()

		// Test ping functionality
		dbPlugin := plugin.(*DatabaseOutputPlugin)
		err = dbPlugin.Ping()
		if err != nil {
			t.Logf("Database ping failed (expected if database not accessible): %v", err)
		}
	})
}

// TestDatabaseOutputPerformance tests performance aspects of database output
func TestDatabaseOutputPerformance(t *testing.T) {
	t.Run("PerformanceTest", func(t *testing.T) {
		// Skip performance test unless specifically requested
		if os.Getenv("LIBAI_PERF_TEST") != "1" {
			t.Skip("Skipping performance test (set LIBAI_PERF_TEST=1 to run)")
		}

		config := DefaultConfig()
		config.Origin = "perf-test"
		config.Console.Enabled = false
		config.File.Enabled = false
		config.Database.Enabled = true
		config.Database.Driver = MongoDB
		config.Database.Database = "perf_test"
		config.Database.Collection = "perf_logs"
		config.Database.BatchSize = 50

		logger, err := NewChainLogger(config)
		if err != nil {
			t.Logf("Performance test logger creation failed: %v", err)
			return
		}
		defer logger.Close()

		start := time.Now()
		const numLogs = 1000

		// Log many entries to test performance
		for i := 0; i < numLogs; i++ {
			logger.Info().
				Msgf("Performance test log entry %d", i).
				Str("test_type", "performance").
				Int("entry_id", i).
				Time("created_at", time.Now()).
				Any("metadata", map[string]interface{}{
					"batch":    i / 50,
					"index":    i % 50,
					"category": "performance_test",
				}).
				Log()
		}

		duration := time.Since(start)
		t.Logf("Logged %d entries in %v (%.2f entries/sec)",
			numLogs, duration, float64(numLogs)/duration.Seconds())

		// Wait for all batches to process
		time.Sleep(500 * time.Millisecond)
	})
}

// BenchmarkDatabaseOutput benchmarks database output performance
func BenchmarkDatabaseOutput(b *testing.B) {
	config := DefaultConfig()
	config.Origin = "benchmark"
	config.Console.Enabled = false
	config.File.Enabled = false
	config.Database.Enabled = true
	config.Database.Driver = MongoDB
	config.Database.Database = "benchmark_test"
	config.Database.Collection = "benchmark_logs"
	config.Database.BatchSize = 100

	logger, err := NewChainLogger(config)
	if err != nil {
		b.Logf("Benchmark logger creation failed: %v", err)
		b.Skip("Skipping benchmark (database not available)")
		return
	}
	defer logger.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			logger.Info().
				Msgf("Benchmark log entry %d", i).
				Str("benchmark", "database_output").
				Int("iteration", i).
				Log()
			i++
		}
	})
	b.StopTimer()

	// Wait for final batch processing
	time.Sleep(100 * time.Millisecond)
}
