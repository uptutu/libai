package main

import (
	"fmt"
	"os"
	"time"

	"local.git/libs/libai.git"
)

// DemoDatabaseOutput demonstrates the database output capabilities
func DemoDatabaseOutput() {
	fmt.Println("=== Database Output Demo ===")

	// Check if we should skip database demos
	if os.Getenv("LIBAI_SKIP_DB_DEMO") == "1" {
		fmt.Println("Database demo skipped (LIBAI_SKIP_DB_DEMO=1)")
		return
	}

	// Demo 1: MongoDB Output
	fmt.Println("\n1. MongoDB Output Demo:")
	demoMongoDBOutput()

	// Demo 2: MySQL Output
	fmt.Println("\n2. MySQL Output Demo:")
	demoMySQLOutput()

	// Demo 3: Batch Processing
	fmt.Println("\n3. Database Batch Processing Demo:")
	demoDatabaseBatching()

	// Demo 4: Context Logging with Database
	fmt.Println("\n4. Context Logging with Database:")
	demoDatabaseContextLogging()

	fmt.Println("\n=== Database Output Demo Complete ===")
}

// demoMongoDBOutput demonstrates MongoDB logging
func demoMongoDBOutput() {
	config := libai.DefaultConfig()
	config.Origin = "mongodb-demo"
	config.Console.Enabled = true // Keep console for demo visibility
	config.Database.Enabled = true
	config.Database.Driver = libai.MongoDB
	config.Database.Host = "localhost"
	config.Database.Port = 27017
	config.Database.Database = "libai_demo"
	config.Database.Collection = "demo_logs"
	config.Database.BatchSize = 5

	logger, err := libai.NewChainLogger(config)
	if err != nil {
		fmt.Printf("❌ Failed to create MongoDB logger: %v\n", err)
		fmt.Println("💡 Make sure MongoDB is running on localhost:27017")
		return
	}
	defer logger.Close()

	fmt.Printf("✅ MongoDB logger created successfully\n")
	fmt.Printf("   - Database: %s\n", config.Database.Database)
	fmt.Printf("   - Collection: %s\n", config.Database.Collection)
	fmt.Printf("   - Batch Size: %d\n", config.Database.BatchSize)

	// Log various types of entries
	logger.Info().
		Msg("Application started").
		Str("version", "1.0.0").
		Str("environment", "demo").
		Time("startup_time", time.Now()).
		Log()

	logger.Info().
		Msg("User authentication").
		Str("user_id", "user_123").
		Str("method", "oauth2").
		Bool("success", true).
		ToDB(). // Explicitly target database
		Log()

	logger.Warn().
		Msg("High memory usage detected").
		Float64("memory_mb", 1024.5).
		Str("component", "cache").
		Int("threshold_mb", 1000).
		Log()

	logger.Error().
		Msg("Database connection timeout").
		Str("host", "db.example.com").
		Dur("timeout", 30*time.Second).
		Str("retry_count", "3").
		Log()

	// Wait for batch processing
	time.Sleep(200 * time.Millisecond)
	fmt.Println("📊 MongoDB logs have been written (check your MongoDB collection)")
}

// demoMySQLOutput demonstrates MySQL logging
func demoMySQLOutput() {
	config := libai.DefaultConfig()
	config.Origin = "mysql-demo"
	config.Console.Enabled = true // Keep console for demo visibility
	config.Database.Enabled = true
	config.Database.Driver = libai.MySQL
	config.Database.Host = "localhost"
	config.Database.Port = 3306
	config.Database.Username = "root"
	config.Database.Password = "password" // You may need to adjust this
	config.Database.Database = "libai_demo"
	config.Database.Table = "demo_logs"
	config.Database.BatchSize = 3

	logger, err := libai.NewChainLogger(config)
	if err != nil {
		fmt.Printf("❌ Failed to create MySQL logger: %v\n", err)
		fmt.Println("💡 Make sure MySQL is running with correct credentials")
		return
	}
	defer logger.Close()

	fmt.Printf("✅ MySQL logger created successfully\n")
	fmt.Printf("   - Database: %s\n", config.Database.Database)
	fmt.Printf("   - Table: %s\n", config.Database.Table)
	fmt.Printf("   - Batch Size: %d\n", config.Database.BatchSize)

	// Log various types of entries
	logger.Info().
		Msg("Service deployment started").
		Str("service", "api-gateway").
		Str("version", "2.1.0").
		Str("environment", "production").
		Log()

	logger.Info().
		Msg("API request processed").
		Str("endpoint", "/api/v1/users").
		Str("method", "GET").
		Int("status_code", 200).
		Dur("response_time", 45*time.Millisecond).
		ToDB(). // Explicitly target database
		WithDatabase(libai.MySQL).
		Log()

	logger.Warn().
		Msg("Rate limit threshold approached").
		Str("client_ip", "192.168.1.100").
		Int("requests_per_minute", 950).
		Int("limit", 1000).
		Log()

	logger.Error().
		Msg("Payment processing failed").
		Str("transaction_id", "tx_abc123").
		Float64("amount", 99.99).
		Str("currency", "USD").
		Str("error_code", "CARD_DECLINED").
		Log()

	// Wait for batch processing
	time.Sleep(200 * time.Millisecond)
	fmt.Println("📊 MySQL logs have been written (check your MySQL table)")
}

// demoDatabaseBatching demonstrates batch processing with high volume
func demoDatabaseBatching() {
	config := libai.DefaultConfig()
	config.Origin = "batch-demo"
	config.Console.Enabled = false // Disable console to focus on database
	config.Database.Enabled = true
	config.Database.Driver = libai.MongoDB
	config.Database.Host = "localhost"
	config.Database.Port = 27017
	config.Database.Database = "libai_demo"
	config.Database.Collection = "batch_demo"
	config.Database.BatchSize = 10 // Process in batches of 10

	logger, err := libai.NewChainLogger(config)
	if err != nil {
		fmt.Printf("❌ Failed to create batch demo logger: %v\n", err)
		return
	}
	defer logger.Close()

	fmt.Printf("✅ Batch processing demo - writing 50 log entries\n")
	fmt.Printf("   - Batch Size: %d\n", config.Database.BatchSize)

	start := time.Now()

	// Generate high volume of logs to demonstrate batching
	for i := 1; i <= 50; i++ {
		logger.Info().
			Msgf("Batch processing entry %d", i).
			Str("batch_id", fmt.Sprintf("batch_%02d", (i-1)/10+1)).
			Int("entry_number", i).
			Str("process_type", "data_ingestion").
			Time("processed_at", time.Now()).
			Any("metadata", map[string]interface{}{
				"source":      "batch_processor",
				"priority":    "normal",
				"category":    "data_processing",
				"tags":        []string{"batch", "demo", "high-volume"},
				"file_size":   1024 * i,
				"chunk_index": i % 10,
			}).
			Log()

		// Small delay to simulate processing time
		time.Sleep(2 * time.Millisecond)
	}

	duration := time.Since(start)
	fmt.Printf("📊 Generated 50 entries in %v (%.2f entries/sec)\n",
		duration, float64(50)/duration.Seconds())

	// Wait for all batches to be processed
	time.Sleep(500 * time.Millisecond)
	fmt.Println("✅ All batches processed and written to database")
}

// demoDatabaseContextLogging demonstrates context logging with database output
func demoDatabaseContextLogging() {
	config := libai.DefaultConfig()
	config.Origin = "context-demo"
	config.Console.Enabled = true // Show on console too
	config.Database.Enabled = true
	config.Database.Driver = libai.MongoDB
	config.Database.Host = "localhost"
	config.Database.Port = 27017
	config.Database.Database = "libai_demo"
	config.Database.Collection = "context_demo"
	config.Database.BatchSize = 5

	logger, err := libai.NewChainLogger(config)
	if err != nil {
		fmt.Printf("❌ Failed to create context demo logger: %v\n", err)
		return
	}
	defer logger.Close()

	fmt.Println("✅ Context logging demo - simulating user session")

	// Create session-specific logger with context
	sessionLogger := logger.WithFields(map[string]interface{}{
		"session_id":   "sess_demo_123",
		"user_id":      "user_456",
		"request_id":   "req_789",
		"ip_address":   "192.168.1.100",
		"user_agent":   "libai-demo/1.0",
		"country_code": "US",
		"timezone":     "America/New_York",
	})

	// Simulate a user session with context
	sessionLogger.Info().
		Msg("User session started").
		Str("action", "login").
		Str("method", "password").
		Bool("remember_me", true).
		Time("login_time", time.Now()).
		Log()

	sessionLogger.Info().
		Msg("Page view").
		Str("page", "/dashboard").
		Str("referrer", "/login").
		Dur("load_time", 250*time.Millisecond).
		Log()

	sessionLogger.Info().
		Msg("API call").
		Str("endpoint", "/api/user/profile").
		Str("method", "GET").
		Int("status_code", 200).
		Dur("response_time", 125*time.Millisecond).
		ToDB(). // Ensure this goes to database
		Log()

	sessionLogger.Warn().
		Msg("Suspicious activity detected").
		Str("activity", "multiple_failed_attempts").
		Int("attempt_count", 3).
		Dur("time_window", 5*time.Minute).
		Str("action_taken", "account_locked").
		Log()

	sessionLogger.Info().
		Msg("User session ended").
		Str("action", "logout").
		Dur("session_duration", 15*time.Minute).
		Int("pages_viewed", 5).
		Int("api_calls", 12).
		Time("logout_time", time.Now()).
		Log()

	// Wait for processing
	time.Sleep(200 * time.Millisecond)
	fmt.Println("✅ Session context has been preserved across all log entries")
}

// DemoDatabaseHealth demonstrates database health monitoring
func DemoDatabaseHealth() {
	fmt.Println("\n=== Database Health Check Demo ===")

	// Test MongoDB health
	fmt.Println("\n1. MongoDB Health Check:")
	mongoConfig := libai.DatabaseConfig{
		Enabled:    true,
		Driver:     libai.MongoDB,
		Host:       "localhost",
		Port:       27017,
		Database:   "health_test",
		Collection: "test_logs",
	}

	plugin, err := libai.NewDatabaseOutputPlugin(mongoConfig)
	if err != nil {
		fmt.Printf("❌ MongoDB connection failed: %v\n", err)
	} else {
		dbPlugin := plugin.(*libai.DatabaseOutputPlugin)
		if err := dbPlugin.Ping(); err != nil {
			fmt.Printf("❌ MongoDB ping failed: %v\n", err)
		} else {
			fmt.Println("✅ MongoDB is healthy and responsive")
			stats := plugin.GetStats()
			fmt.Printf("   Stats: %+v\n", stats)
		}
		plugin.Close()
	}

	// Test MySQL health
	fmt.Println("\n2. MySQL Health Check:")
	mysqlConfig := libai.DatabaseConfig{
		Enabled:  true,
		Driver:   libai.MySQL,
		Host:     "localhost",
		Port:     3306,
		Username: "root",
		Password: "password",
		Database: "health_test",
		Table:    "test_logs",
	}

	plugin, err = libai.NewDatabaseOutputPlugin(mysqlConfig)
	if err != nil {
		fmt.Printf("❌ MySQL connection failed: %v\n", err)
	} else {
		dbPlugin := plugin.(*libai.DatabaseOutputPlugin)
		if err := dbPlugin.Ping(); err != nil {
			fmt.Printf("❌ MySQL ping failed: %v\n", err)
		} else {
			fmt.Println("✅ MySQL is healthy and responsive")
			stats := plugin.GetStats()
			fmt.Printf("   Stats: %+v\n", stats)
		}
		plugin.Close()
	}

	fmt.Println("\n=== Database Health Check Complete ===")
}
