package libai

import (
	"sync"
	"time"
)

// Config represents the configuration for the ChainLogger
type Config struct {
	// General settings
	Origin      string   `json:"origin" yaml:"origin"`
	Level       LogLevel `json:"level" yaml:"level"`
	EnableStack bool     `json:"enable_stack" yaml:"enable_stack"`
	
	// Console output settings
	Console ConsoleConfig `json:"console" yaml:"console"`
	
	// File output settings
	File FileConfig `json:"file" yaml:"file"`
	
	// Database settings
	Database DatabaseConfig `json:"database" yaml:"database"`
	
	// Message queue settings
	MessageQueue MessageQueueConfig `json:"message_queue" yaml:"message_queue"`
	
	// Performance settings
	Performance PerformanceConfig `json:"performance" yaml:"performance"`
}

// ConsoleConfig contains console output configuration
type ConsoleConfig struct {
	Enabled    bool   `json:"enabled" yaml:"enabled"`
	Format     string `json:"format" yaml:"format"` // "text" or "json"
	Colorized  bool   `json:"colorized" yaml:"colorized"`
	TimeFormat string `json:"time_format" yaml:"time_format"`
}

// FileConfig contains file output configuration
type FileConfig struct {
	Enabled    bool   `json:"enabled" yaml:"enabled"`
	Filename   string `json:"filename" yaml:"filename"`
	Format     string `json:"format" yaml:"format"` // "text" or "json"
	TimeFormat string `json:"time_format" yaml:"time_format"`
	
	// Rotation settings
	MaxSize    int64 `json:"max_size" yaml:"max_size"`       // Maximum size in bytes before rotation
	MaxAge     int   `json:"max_age" yaml:"max_age"`         // Maximum age in days before deletion
	MaxBackups int   `json:"max_backups" yaml:"max_backups"` // Maximum number of old files to keep
	Compress   bool  `json:"compress" yaml:"compress"`       // Whether to compress rotated files
	LocalTime  bool  `json:"local_time" yaml:"local_time"`   // Use local time for rotation
}

// DatabaseConfig contains database output configuration
type DatabaseConfig struct {
	Enabled    bool           `json:"enabled" yaml:"enabled"`
	Driver     DatabaseDriver `json:"driver" yaml:"driver"`
	Host       string         `json:"host" yaml:"host"`
	Port       int            `json:"port" yaml:"port"`
	Username   string         `json:"username" yaml:"username"`
	Password   string         `json:"password" yaml:"password"`
	Database   string         `json:"database" yaml:"database"`
	Collection string         `json:"collection" yaml:"collection"` // for MongoDB
	Table      string         `json:"table" yaml:"table"`           // for SQL databases
	BatchSize  int            `json:"batch_size" yaml:"batch_size"`
}

// MessageQueueConfig contains message queue configuration
type MessageQueueConfig struct {
	Enabled   bool              `json:"enabled" yaml:"enabled"`
	Type      string            `json:"type" yaml:"type"` // "kafka", "rabbitmq", etc.
	Brokers   []string          `json:"brokers" yaml:"brokers"`
	Topic     string            `json:"topic" yaml:"topic"`
	Config    map[string]string `json:"config" yaml:"config"`
	BatchSize int               `json:"batch_size" yaml:"batch_size"`
}

// PerformanceConfig contains performance-related settings
type PerformanceConfig struct {
	AsyncOutput   bool `json:"async_output" yaml:"async_output"`
	BufferSize    int  `json:"buffer_size" yaml:"buffer_size"`
	WorkerCount   int  `json:"worker_count" yaml:"worker_count"`
	EnablePooling bool `json:"enable_pooling" yaml:"enable_pooling"`
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		Level:       InfoLevel,
		EnableStack: false,
		Console: ConsoleConfig{
			Enabled:    true,
			Format:     "text",
			Colorized:  true,
			TimeFormat: time.RFC3339,
		},
		File: FileConfig{
			Enabled:    false,
			Format:     "json",
			MaxSize:    100 * 1024 * 1024, // 100MB
			MaxAge:     7,                  // 7 days
			MaxBackups: 5,                  // Keep 5 old files
			Compress:   true,               // Compress rotated files
			LocalTime:  true,               // Use local time
		},
		Database: DatabaseConfig{
			Enabled:   false,
			BatchSize: 100,
		},
		MessageQueue: MessageQueueConfig{
			Enabled:   false,
			BatchSize: 100,
		},
		Performance: PerformanceConfig{
			AsyncOutput:   true,
			BufferSize:    1000,
			WorkerCount:   2,
			EnablePooling: true,
		},
	}
}

// ObjectPool manages reusable objects to reduce allocations
type ObjectPool struct {
	entryPool *sync.Pool
	chainPool *sync.Pool
}

// NewObjectPool creates a new object pool
func NewObjectPool() *ObjectPool {
	return &ObjectPool{
		entryPool: &sync.Pool{
			New: func() interface{} {
				return NewLogEntry()
			},
		},
		chainPool: &sync.Pool{
			New: func() interface{} {
				return &LogChainImpl{}
			},
		},
	}
}

// GetLogEntry gets a log entry from the pool
func (p *ObjectPool) GetLogEntry() *LogEntry {
	entry := p.entryPool.Get().(*LogEntry)
	// Reset the entry
	entry.Level = InfoLevel
	entry.Message = ""
	entry.Timestamp = time.Now()
	if entry.Fields == nil {
		entry.Fields = make(map[string]interface{})
	} else {
		for k := range entry.Fields {
			delete(entry.Fields, k)
		}
	}
	entry.Origin = ""
	entry.Action = ""
	entry.Flag = ""
	entry.StackTrace = ""
	entry.Caller = ""
	entry.Error = nil
	entry.Targets = entry.Targets[:0]
	entry.Database = nil
	entry.Filename = ""
	entry.Topic = ""
	return entry
}

// PutLogEntry returns a log entry to the pool
func (p *ObjectPool) PutLogEntry(entry *LogEntry) {
	if entry != nil {
		p.entryPool.Put(entry)
	}
}

// GetLogChain gets a log chain from the pool
func (p *ObjectPool) GetLogChain() *LogChainImpl {
	chain := p.chainPool.Get().(*LogChainImpl)
	// Reset will be done by the chain itself
	return chain
}

// PutLogChain returns a log chain to the pool
func (p *ObjectPool) PutLogChain(chain *LogChainImpl) {
	if chain != nil {
		chain.reset()
		p.chainPool.Put(chain)
	}
}