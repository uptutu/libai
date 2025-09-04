package libai

import (
	"fmt"
	"time"
)

// LogLevel represents the severity level of a log entry
type LogLevel int

const (
	DebugLevel LogLevel = iota
	InfoLevel
	WarnLevel
	ErrorLevel
	FatalLevel
)

// String returns the string representation of the log level
func (l LogLevel) String() string {
	switch l {
	case DebugLevel:
		return "debug"
	case InfoLevel:
		return "info"
	case WarnLevel:
		return "warn"
	case ErrorLevel:
		return "error"
	case FatalLevel:
		return "fatal"
	default:
		return "unknown"
	}
}

// ParseLogLevel parses a string into a LogLevel
func ParseLogLevel(level string) (LogLevel, error) {
	switch level {
	case "debug":
		return DebugLevel, nil
	case "info":
		return InfoLevel, nil
	case "warn":
		return WarnLevel, nil
	case "error":
		return ErrorLevel, nil
	case "fatal":
		return FatalLevel, nil
	default:
		return InfoLevel, fmt.Errorf("unknown log level: %s", level)
	}
}

// DatabaseDriver represents supported database types
type DatabaseDriver int

const (
	MongoDB DatabaseDriver = iota
	MySQL
	PostgreSQL
)

// String returns the string representation of the database driver
func (d DatabaseDriver) String() string {
	switch d {
	case MongoDB:
		return "mongodb"
	case MySQL:
		return "mysql"
	case PostgreSQL:
		return "postgresql"
	default:
		return "unknown"
	}
}

// OutputType represents different output destination types
type OutputType int

const (
	ConsoleOutput OutputType = iota
	FileOutput
	DatabaseOutput
	MessageQueueOutput
)

// String returns the string representation of the output type
func (o OutputType) String() string {
	switch o {
	case ConsoleOutput:
		return "console"
	case FileOutput:
		return "file"
	case DatabaseOutput:
		return "database"
	case MessageQueueOutput:
		return "messagequeue"
	default:
		return "unknown"
	}
}

// LogEntry represents a single log entry with all its metadata
type LogEntry struct {
	Level      LogLevel               `json:"level" bson:"level"`
	Message    string                 `json:"message" bson:"message"`
	Timestamp  time.Time              `json:"timestamp" bson:"timestamp"`
	Fields     map[string]interface{} `json:"fields" bson:"fields"`
	Origin     string                 `json:"origin" bson:"origin"`
	Action     string                 `json:"action" bson:"action"`
	Flag       string                 `json:"flag" bson:"flag"`
	StackTrace string                 `json:"stack_trace,omitempty" bson:"stack_trace,omitempty"`
	Caller     string                 `json:"caller,omitempty" bson:"caller,omitempty"`
	Error      error                  `json:"error,omitempty" bson:"error,omitempty"`
	
	// Output targeting
	Targets    []OutputType           `json:"-" bson:"-"`
	Database   *DatabaseDriver       `json:"-" bson:"-"`
	Filename   string                `json:"-" bson:"-"`
	Topic      string                `json:"-" bson:"-"`
}

// NewLogEntry creates a new log entry with default values
func NewLogEntry() *LogEntry {
	return &LogEntry{
		Timestamp: time.Now(),
		Fields:    make(map[string]interface{}),
		Targets:   make([]OutputType, 0),
	}
}

// Copy creates a deep copy of the log entry
func (e *LogEntry) Copy() *LogEntry {
	fields := make(map[string]interface{})
	for k, v := range e.Fields {
		fields[k] = v
	}
	
	targets := make([]OutputType, len(e.Targets))
	copy(targets, e.Targets)
	
	var database *DatabaseDriver
	if e.Database != nil {
		dbCopy := *e.Database
		database = &dbCopy
	}
	
	return &LogEntry{
		Level:      e.Level,
		Message:    e.Message,
		Timestamp:  e.Timestamp,
		Fields:     fields,
		Origin:     e.Origin,
		Action:     e.Action,
		Flag:       e.Flag,
		StackTrace: e.StackTrace,
		Caller:     e.Caller,
		Error:      e.Error,
		Targets:    targets,
		Database:   database,
		Filename:   e.Filename,
		Topic:      e.Topic,
	}
}