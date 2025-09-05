package libai

import (
	"context"
	"time"
)

// ChainLogger is the main interface for chain-style logging
type ChainLogger interface {
	// Chain methods for different log levels
	Debug() LogChain
	Info() LogChain
	Warn() LogChain
	Error() LogChain
	Fatal() LogChain

	// Context methods
	With() LogChain
	WithFields(fields map[string]interface{}) ChainLogger

	// Legacy compatibility
	Legacy() LegacyLogger

	// Configuration and control
	SetLevel(level LogLevel)
	Close() error
}

// LogChain represents a chainable logging context
type LogChain interface {
	// Level methods for setting log level on existing chain
	Debug() LogChain
	Info() LogChain
	Warn() LogChain
	Error() LogChain
	Fatal() LogChain

	// Message methods
	Msg(msg string) LogChain
	Msgf(format string, args ...interface{}) LogChain

	// Field methods
	Str(key, val string) LogChain
	Int(key string, val int) LogChain
	Int64(key string, val int64) LogChain
	Float64(key string, val float64) LogChain
	Bool(key string, val bool) LogChain
	Time(key string, val time.Time) LogChain
	Dur(key string, val time.Duration) LogChain
	Any(key string, val interface{}) LogChain
	Err(err error) LogChain

	// Context and metadata
	Stack() LogChain
	Caller() LogChain
	With(key string, val interface{}) LogChain
	WithFields(fields map[string]interface{}) LogChain

	// Output targeting
	ToConsole() LogChain
	ToFile(filename string) LogChain
	ToDB() LogChain
	ToMQ(topic string) LogChain
	WithDatabase(driver DatabaseDriver) LogChain

	// Execution
	Log()
	Logf(format string, args ...interface{})
}

// OutputPlugin defines the interface for output destinations
type OutputPlugin interface {
	Write(ctx context.Context, entry *LogEntry) error
	Close() error
	Name() string
	GetStats() map[string]interface{}
}

// Formatter defines the interface for log formatting
type Formatter interface {
	Format(entry *LogEntry) ([]byte, error)
}

// LegacyLogger provides backward compatibility with the existing Logger API
type LegacyLogger interface {
	Debug(action string, flag string, content any)
	Info(action string, flag string, content any)
	Warn(action string, flag string, content any)
	Error(action string, flag string, content any)
	Fatal(action string, flag string, content any)
	Z() *LoggerZapWrapper
	Zap() interface{} // Returns *zap.Logger but kept as interface{} to avoid import cycles
}

// HealthChecker defines the interface for health monitoring
type HealthChecker interface {
	IsHealthy() bool
	LastError() error
	Check(ctx context.Context) error
}

// ErrorHandler defines the interface for error handling strategies
type ErrorHandler interface {
	HandleError(err error, entry *LogEntry) error
}
