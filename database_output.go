package libai

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// DatabaseOutputPlugin implements the OutputPlugin interface for database output
type DatabaseOutputPlugin struct {
	config     DatabaseConfig
	driver     DatabaseDriver
	mongoDB    *mongo.Client
	sqlDB      *sql.DB
	batchQueue []*LogEntry
	batchMutex sync.Mutex
	flushTimer *time.Timer
	closed     bool
	closeCh    chan struct{}
}

// NewDatabaseOutputPlugin creates a new database output plugin
func NewDatabaseOutputPlugin(config DatabaseConfig) (OutputPlugin, error) {
	plugin := &DatabaseOutputPlugin{
		config:     config,
		driver:     config.Driver,
		batchQueue: make([]*LogEntry, 0, config.BatchSize),
		closeCh:    make(chan struct{}),
	}

	// Initialize the appropriate database connection
	switch config.Driver {
	case MongoDB:
		if err := plugin.initMongoDB(); err != nil {
			return nil, fmt.Errorf("failed to initialize MongoDB: %w", err)
		}
	case MySQL:
		if err := plugin.initMySQL(); err != nil {
			return nil, fmt.Errorf("failed to initialize MySQL: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported database driver: %v", config.Driver)
	}

	// Start the batch processor
	go plugin.batchProcessor()

	return plugin, nil
}

// initMongoDB initializes MongoDB connection
func (d *DatabaseOutputPlugin) initMongoDB() error {
	// Build connection string
	var uri string
	if d.config.Username != "" && d.config.Password != "" {
		uri = fmt.Sprintf("mongodb://%s:%s@%s:%d/%s",
			d.config.Username, d.config.Password,
			d.config.Host, d.config.Port, d.config.Database)
	} else {
		uri = fmt.Sprintf("mongodb://%s:%d/%s",
			d.config.Host, d.config.Port, d.config.Database)
	}

	// Create client options
	clientOptions := options.Client().ApplyURI(uri)

	// Set connection pool options
	maxPoolSize := uint64(100)
	clientOptions.SetMaxPoolSize(maxPoolSize)

	// Create client
	client, err := mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		return fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	// Ping to verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx, nil); err != nil {
		return fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	d.mongoDB = client
	return nil
}

// initMySQL initializes MySQL connection
func (d *DatabaseOutputPlugin) initMySQL() error {
	// Build DSN (Data Source Name)
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		d.config.Username, d.config.Password,
		d.config.Host, d.config.Port, d.config.Database)

	// Open connection
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("failed to open MySQL connection: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(time.Hour)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to ping MySQL: %w", err)
	}

	// Create logs table if it doesn't exist
	if err := d.createMySQLTable(db); err != nil {
		return fmt.Errorf("failed to create MySQL table: %w", err)
	}

	d.sqlDB = db
	return nil
}

// createMySQLTable creates the logs table if it doesn't exist
func (d *DatabaseOutputPlugin) createMySQLTable(db *sql.DB) error {
	tableName := d.config.Table
	if tableName == "" {
		tableName = "logs"
	}

	createTableSQL := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			level VARCHAR(20) NOT NULL,
			message TEXT,
			origin VARCHAR(100),
			action VARCHAR(100),
			flag VARCHAR(100),
			fields JSON,
			error_msg TEXT,
			caller VARCHAR(500),
			stack_trace TEXT,
			timestamp DATETIME(3) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_level (level),
			INDEX idx_origin (origin),
			INDEX idx_timestamp (timestamp),
			INDEX idx_created_at (created_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
	`, tableName)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := db.ExecContext(ctx, createTableSQL)
	return err
}

// Write writes a log entry to the database (queues for batch processing)
func (d *DatabaseOutputPlugin) Write(ctx context.Context, entry *LogEntry) error {
	if d.closed {
		return fmt.Errorf("database output plugin is closed")
	}

	// Add to batch queue
	d.batchMutex.Lock()
	defer d.batchMutex.Unlock()

	d.batchQueue = append(d.batchQueue, entry.Copy())

	// Flush if batch is full
	if len(d.batchQueue) >= d.config.BatchSize {
		go d.flushBatch()
	}

	return nil
}

// batchProcessor handles periodic batch flushing
func (d *DatabaseOutputPlugin) batchProcessor() {
	ticker := time.NewTicker(5 * time.Second) // Flush every 5 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			d.flushBatch()
		case <-d.closeCh:
			d.flushBatch() // Final flush
			return
		}
	}
}

// flushBatch flushes the current batch to the database
func (d *DatabaseOutputPlugin) flushBatch() {
	d.batchMutex.Lock()
	if len(d.batchQueue) == 0 {
		d.batchMutex.Unlock()
		return
	}

	batch := make([]*LogEntry, len(d.batchQueue))
	copy(batch, d.batchQueue)
	d.batchQueue = d.batchQueue[:0] // Clear the queue
	d.batchMutex.Unlock()

	// Write batch to database
	switch d.driver {
	case MongoDB:
		d.flushMongoDBBatch(batch)
	case MySQL:
		d.flushMySQLBatch(batch)
	}
}

// flushMongoDBBatch writes a batch of entries to MongoDB
func (d *DatabaseOutputPlugin) flushMongoDBBatch(entries []*LogEntry) {
	if d.mongoDB == nil {
		return
	}

	collection := d.mongoDB.Database(d.config.Database).Collection(d.config.Collection)

	// Convert entries to MongoDB documents
	docs := make([]interface{}, len(entries))
	for i, entry := range entries {
		// Serialize fields to JSON string for compatibility with existing schema
		fieldsJSON, _ := json.Marshal(entry.Fields)

		docs[i] = map[string]interface{}{
			"level":       entry.Level.String(),
			"origin":      entry.Origin,
			"action":      entry.Action,
			"flag":        entry.Flag,
			"create_time": int(entry.Timestamp.Unix()),
			"content":     string(fieldsJSON),
			"stack_trace": entry.StackTrace,
			"message":     entry.Message,
			"caller":      entry.Caller,
			"error": func() string {
				if entry.Error != nil {
					return entry.Error.Error()
				}
				return ""
			}(),
		}
	}

	// Insert batch
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := collection.InsertMany(ctx, docs)
	if err != nil {
		// TODO: Handle error with error handler
		fmt.Printf("Failed to insert MongoDB batch: %v\n", err)
	}
}

// flushMySQLBatch writes a batch of entries to MySQL
func (d *DatabaseOutputPlugin) flushMySQLBatch(entries []*LogEntry) {
	if d.sqlDB == nil {
		return
	}

	tableName := d.config.Table
	if tableName == "" {
		tableName = "logs"
	}

	// Prepare batch insert statement
	valueStrings := make([]string, 0, len(entries))
	valueArgs := make([]interface{}, 0, len(entries)*10)

	for _, entry := range entries {
		valueStrings = append(valueStrings, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")

		// Serialize fields to JSON
		fieldsJSON, _ := json.Marshal(entry.Fields)

		errorMsg := ""
		if entry.Error != nil {
			errorMsg = entry.Error.Error()
		}

		valueArgs = append(valueArgs,
			entry.Level.String(), // level
			entry.Message,        // message
			entry.Origin,         // origin
			entry.Action,         // action
			entry.Flag,           // flag
			string(fieldsJSON),   // fields (JSON)
			errorMsg,             // error_msg
			entry.Caller,         // caller
			entry.StackTrace,     // stack_trace
			entry.Timestamp,      // timestamp
		)
	}

	stmt := fmt.Sprintf("INSERT INTO %s (level, message, origin, action, flag, fields, error_msg, caller, stack_trace, timestamp) VALUES %s",
		tableName, strings.Join(valueStrings, ","))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := d.sqlDB.ExecContext(ctx, stmt, valueArgs...)
	if err != nil {
		// TODO: Handle error with error handler
		fmt.Printf("Failed to insert MySQL batch: %v\n", err)
	}
}

// Close closes the database connection and flushes any remaining entries
func (d *DatabaseOutputPlugin) Close() error {
	if d.closed {
		return nil
	}

	d.closed = true
	close(d.closeCh) // Signal batch processor to stop

	// Wait a moment for final flush
	time.Sleep(100 * time.Millisecond)

	// Close connections
	var errs []error
	if d.mongoDB != nil {
		if err := d.mongoDB.Disconnect(context.Background()); err != nil {
			errs = append(errs, fmt.Errorf("MongoDB disconnect error: %w", err))
		}
	}
	if d.sqlDB != nil {
		if err := d.sqlDB.Close(); err != nil {
			errs = append(errs, fmt.Errorf("MySQL close error: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("database close errors: %v", errs)
	}

	return nil
}

// Name returns the name of the plugin
func (d *DatabaseOutputPlugin) Name() string {
	return fmt.Sprintf("database_%s", d.driver.String())
}

// GetStats returns statistics about the database plugin
func (d *DatabaseOutputPlugin) GetStats() map[string]interface{} {
	d.batchMutex.Lock()
	queueSize := len(d.batchQueue)
	d.batchMutex.Unlock()

	return map[string]interface{}{
		"driver":     d.driver.String(),
		"queue_size": queueSize,
		"batch_size": d.config.BatchSize,
		"database":   d.config.Database,
		"collection_table": func() string {
			if d.driver == MongoDB {
				return d.config.Collection
			}
			return d.config.Table
		}(),
	}
}

// Ping tests the database connection
func (d *DatabaseOutputPlugin) Ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	switch d.driver {
	case MongoDB:
		if d.mongoDB == nil {
			return fmt.Errorf("MongoDB client is nil")
		}
		return d.mongoDB.Ping(ctx, nil)
	case MySQL:
		if d.sqlDB == nil {
			return fmt.Errorf("MySQL connection is nil")
		}
		return d.sqlDB.PingContext(ctx)
	default:
		return fmt.Errorf("unsupported driver: %v", d.driver)
	}
}
