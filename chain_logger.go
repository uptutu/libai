package libai

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"
)

// ChainLoggerImpl is the main implementation of ChainLogger
type ChainLoggerImpl struct {
	config        *Config
	outputs       map[OutputType]OutputPlugin
	pool          *ObjectPool
	level         LogLevel
	contextFields map[string]interface{} // Context fields for this logger instance
	mu            sync.RWMutex
	closed        bool
}

// NewChainLogger creates a new ChainLogger instance
func NewChainLogger(config *Config) (ChainLogger, error) {
	if config == nil {
		config = DefaultConfig()
	}
	
	logger := &ChainLoggerImpl{
		config:        config,
		outputs:       make(map[OutputType]OutputPlugin),
		pool:          NewObjectPool(),
		level:         config.Level,
		contextFields: make(map[string]interface{}),
	}
	
	// Initialize enabled output plugins
	if err := logger.initializeOutputs(); err != nil {
		return nil, fmt.Errorf("failed to initialize outputs: %w", err)
	}
	
	return logger, nil
}

// initializeOutputs initializes all enabled output plugins
func (c *ChainLoggerImpl) initializeOutputs() error {
	// Initialize console output
	if c.config.Console.Enabled {
		consoleOutput := NewConsoleOutputPlugin(c.config.Console)
		c.outputs[ConsoleOutput] = consoleOutput
	}
	
	if c.config.File.Enabled {
		fileOutput := NewFileOutputPlugin(c.config.File)
		c.outputs[FileOutput] = fileOutput
	}
	
	if c.config.Database.Enabled {
		// Database output will be implemented in Phase 5
	}
	
	if c.config.MessageQueue.Enabled {
		// MQ output will be implemented in Phase 6
	}
	
	return nil
}

// Debug creates a debug level log chain
func (c *ChainLoggerImpl) Debug() LogChain {
	if c.level > DebugLevel {
		return &NoOpLogChain{}
	}
	return c.newLogChain(DebugLevel)
}

// Info creates an info level log chain
func (c *ChainLoggerImpl) Info() LogChain {
	if c.level > InfoLevel {
		return &NoOpLogChain{}
	}
	return c.newLogChain(InfoLevel)
}

// Warn creates a warn level log chain
func (c *ChainLoggerImpl) Warn() LogChain {
	if c.level > WarnLevel {
		return &NoOpLogChain{}
	}
	return c.newLogChain(WarnLevel)
}

// Error creates an error level log chain
func (c *ChainLoggerImpl) Error() LogChain {
	if c.level > ErrorLevel {
		return &NoOpLogChain{}
	}
	return c.newLogChain(ErrorLevel)
}

// Fatal creates a fatal level log chain
func (c *ChainLoggerImpl) Fatal() LogChain {
	if c.level > FatalLevel {
		return &NoOpLogChain{}
	}
	return c.newLogChain(FatalLevel)
}

// With creates a new log chain with context  
func (c *ChainLoggerImpl) With() LogChain {
	chain := c.newLogChain(InfoLevel)
	// Reset level to allow setting it later
	chain.(*LogChainImpl).entry.Level = InfoLevel
	return chain
}

// WithFields creates a context logger that preserves fields across calls
func (c *ChainLoggerImpl) WithFields(fields map[string]interface{}) ChainLogger {
	contextFields := make(map[string]interface{})
	// Copy existing context fields
	c.mu.RLock()
	for k, v := range c.contextFields {
		contextFields[k] = v
	}
	c.mu.RUnlock()
	
	// Add new fields
	for k, v := range fields {
		contextFields[k] = v
	}
	
	// Create new context logger
	contextLogger := &ChainLoggerImpl{
		config:        c.config,
		outputs:       c.outputs, // Share the same outputs
		pool:          c.pool,    // Share the same pool
		level:         c.level,
		contextFields: contextFields,
	}
	
	return contextLogger
}

// Legacy returns a legacy logger wrapper for backward compatibility
func (c *ChainLoggerImpl) Legacy() LegacyLogger {
	return &LegacyWrapper{chainLogger: c}
}

// SetLevel sets the minimum log level
func (c *ChainLoggerImpl) SetLevel(level LogLevel) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.level = level
}

// Close closes all output plugins and shuts down the logger
func (c *ChainLoggerImpl) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.closed {
		return nil
	}
	
	var errs []error
	for _, output := range c.outputs {
		if err := output.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	
	c.closed = true
	
	if len(errs) > 0 {
		return fmt.Errorf("errors closing outputs: %v", errs)
	}
	
	return nil
}

// newLogChain creates a new log chain
func (c *ChainLoggerImpl) newLogChain(level LogLevel) LogChain {
	entry := c.pool.GetLogEntry()
	entry.Level = level
	entry.Origin = c.config.Origin
	
	// Add context fields if any
	c.mu.RLock()
	if c.contextFields != nil {
		for k, v := range c.contextFields {
			entry.Fields[k] = v
		}
	}
	c.mu.RUnlock()
	
	// Add stack trace for non-info levels if enabled
	if c.config.EnableStack && level != InfoLevel {
		entry.StackTrace = c.getStackTrace()
	}
	
	return &LogChainImpl{
		logger: c,
		entry:  entry,
	}
}

// getStackTrace captures the current stack trace
func (c *ChainLoggerImpl) getStackTrace() string {
	buf := make([]byte, 1024*4)
	n := runtime.Stack(buf, false)
	stack := string(buf[:n])
	
	// Filter out the current function and log function calls
	lines := strings.Split(stack, "\n")
	var filteredLines []string
	skip := 0
	for i, line := range lines {
		if strings.Contains(line, "getStackTrace") || strings.Contains(line, ".log") || strings.Contains(line, "newLogChain") {
			skip = i + 2 // Skip function name and file:line
			continue
		}
		if i > skip {
			filteredLines = append(filteredLines, line)
		}
	}
	
	return strings.Join(filteredLines, "\n")
}

// LogChainImpl implements the LogChain interface
type LogChainImpl struct {
	logger *ChainLoggerImpl
	entry  *LogEntry
}

// Debug sets the log level to debug
func (lc *LogChainImpl) Debug() LogChain {
	if lc.logger.level > DebugLevel {
		return &NoOpLogChain{}
	}
	lc.entry.Level = DebugLevel
	return lc
}

// Info sets the log level to info
func (lc *LogChainImpl) Info() LogChain {
	if lc.logger.level > InfoLevel {
		return &NoOpLogChain{}
	}
	lc.entry.Level = InfoLevel
	return lc
}

// Warn sets the log level to warn
func (lc *LogChainImpl) Warn() LogChain {
	if lc.logger.level > WarnLevel {
		return &NoOpLogChain{}
	}
	lc.entry.Level = WarnLevel
	return lc
}

// Error sets the log level to error
func (lc *LogChainImpl) Error() LogChain {
	if lc.logger.level > ErrorLevel {
		return &NoOpLogChain{}
	}
	lc.entry.Level = ErrorLevel
	return lc
}

// Fatal sets the log level to fatal
func (lc *LogChainImpl) Fatal() LogChain {
	if lc.logger.level > FatalLevel {
		return &NoOpLogChain{}
	}
	lc.entry.Level = FatalLevel
	return lc
}

// Msg sets the log message
func (lc *LogChainImpl) Msg(msg string) LogChain {
	lc.entry.Message = msg
	return lc
}

// Msgf sets the log message using format string
func (lc *LogChainImpl) Msgf(format string, args ...interface{}) LogChain {
	lc.entry.Message = fmt.Sprintf(format, args...)
	return lc
}

// Str adds a string field
func (lc *LogChainImpl) Str(key, val string) LogChain {
	lc.entry.Fields[key] = val
	return lc
}

// Int adds an int field
func (lc *LogChainImpl) Int(key string, val int) LogChain {
	lc.entry.Fields[key] = val
	return lc
}

// Int64 adds an int64 field
func (lc *LogChainImpl) Int64(key string, val int64) LogChain {
	lc.entry.Fields[key] = val
	return lc
}

// Float64 adds a float64 field
func (lc *LogChainImpl) Float64(key string, val float64) LogChain {
	lc.entry.Fields[key] = val
	return lc
}

// Bool adds a boolean field
func (lc *LogChainImpl) Bool(key string, val bool) LogChain {
	lc.entry.Fields[key] = val
	return lc
}

// Time adds a time field
func (lc *LogChainImpl) Time(key string, val time.Time) LogChain {
	lc.entry.Fields[key] = val
	return lc
}

// Dur adds a duration field
func (lc *LogChainImpl) Dur(key string, val time.Duration) LogChain {
	lc.entry.Fields[key] = val.String()
	return lc
}

// Any adds a field of any type
func (lc *LogChainImpl) Any(key string, val interface{}) LogChain {
	lc.entry.Fields[key] = val
	return lc
}

// Err adds an error field
func (lc *LogChainImpl) Err(err error) LogChain {
	if err != nil {
		lc.entry.Error = err
		lc.entry.Fields["error"] = err.Error()
	}
	return lc
}

// Stack adds stack trace to the log entry
func (lc *LogChainImpl) Stack() LogChain {
	lc.entry.StackTrace = lc.logger.getStackTrace()
	return lc
}

// Caller adds caller information
func (lc *LogChainImpl) Caller() LogChain {
	if pc, file, line, ok := runtime.Caller(2); ok {
		if f := runtime.FuncForPC(pc); f != nil {
			lc.entry.Caller = fmt.Sprintf("%s:%d %s", file, line, f.Name())
		} else {
			lc.entry.Caller = fmt.Sprintf("%s:%d", file, line)
		}
	}
	return lc
}

// With adds a single field
func (lc *LogChainImpl) With(key string, val interface{}) LogChain {
	lc.entry.Fields[key] = val
	return lc
}

// WithFields adds multiple fields
func (lc *LogChainImpl) WithFields(fields map[string]interface{}) LogChain {
	for k, v := range fields {
		lc.entry.Fields[k] = v
	}
	return lc
}

// ToConsole targets console output
func (lc *LogChainImpl) ToConsole() LogChain {
	lc.entry.Targets = append(lc.entry.Targets, ConsoleOutput)
	return lc
}

// ToFile targets file output
func (lc *LogChainImpl) ToFile(filename string) LogChain {
	lc.entry.Targets = append(lc.entry.Targets, FileOutput)
	lc.entry.Filename = filename
	return lc
}

// ToDB targets database output
func (lc *LogChainImpl) ToDB() LogChain {
	lc.entry.Targets = append(lc.entry.Targets, DatabaseOutput)
	return lc
}

// ToMQ targets message queue output
func (lc *LogChainImpl) ToMQ(topic string) LogChain {
	lc.entry.Targets = append(lc.entry.Targets, MessageQueueOutput)
	lc.entry.Topic = topic
	return lc
}

// WithDatabase sets the database driver for database output
func (lc *LogChainImpl) WithDatabase(driver DatabaseDriver) LogChain {
	lc.entry.Database = &driver
	return lc
}

// Log executes the log entry
func (lc *LogChainImpl) Log() {
	defer func() {
		lc.logger.pool.PutLogEntry(lc.entry)
		lc.logger.pool.PutLogChain(lc)
	}()
	
	// If no specific targets are set, use default outputs
	if len(lc.entry.Targets) == 0 {
		lc.setDefaultTargets()
	}
	
	// Write to all targeted outputs
	ctx := context.Background()
	for _, target := range lc.entry.Targets {
		if output, exists := lc.logger.outputs[target]; exists {
			if err := output.Write(ctx, lc.entry); err != nil {
				// TODO: Handle errors with error handler
				// For now, just continue to next output
				continue
			}
		}
	}
	
	// Handle fatal level
	if lc.entry.Level == FatalLevel {
		panic(fmt.Sprintf("Fatal log: %s", lc.entry.Message))
	}
}

// Logf executes the log entry with formatted message
func (lc *LogChainImpl) Logf(format string, args ...interface{}) {
	lc.Msgf(format, args...).Log()
}

// setDefaultTargets sets default output targets based on configuration
func (lc *LogChainImpl) setDefaultTargets() {
	if lc.logger.config.Console.Enabled {
		lc.entry.Targets = append(lc.entry.Targets, ConsoleOutput)
	}
	if lc.logger.config.File.Enabled {
		lc.entry.Targets = append(lc.entry.Targets, FileOutput)
	}
	if lc.logger.config.Database.Enabled {
		lc.entry.Targets = append(lc.entry.Targets, DatabaseOutput)
	}
	if lc.logger.config.MessageQueue.Enabled {
		lc.entry.Targets = append(lc.entry.Targets, MessageQueueOutput)
	}
}

// reset resets the log chain for reuse in object pool
func (lc *LogChainImpl) reset() {
	lc.logger = nil
	lc.entry = nil
}

// NoOpLogChain is a no-operation log chain for disabled log levels
type NoOpLogChain struct{}

func (n *NoOpLogChain) Debug() LogChain                                     { return n }
func (n *NoOpLogChain) Info() LogChain                                      { return n }
func (n *NoOpLogChain) Warn() LogChain                                      { return n }
func (n *NoOpLogChain) Error() LogChain                                     { return n }
func (n *NoOpLogChain) Fatal() LogChain                                     { return n }
func (n *NoOpLogChain) Msg(msg string) LogChain                              { return n }
func (n *NoOpLogChain) Msgf(format string, args ...interface{}) LogChain    { return n }
func (n *NoOpLogChain) Str(key, val string) LogChain                        { return n }
func (n *NoOpLogChain) Int(key string, val int) LogChain                     { return n }
func (n *NoOpLogChain) Int64(key string, val int64) LogChain                 { return n }
func (n *NoOpLogChain) Float64(key string, val float64) LogChain             { return n }
func (n *NoOpLogChain) Bool(key string, val bool) LogChain                   { return n }
func (n *NoOpLogChain) Time(key string, val time.Time) LogChain              { return n }
func (n *NoOpLogChain) Dur(key string, val time.Duration) LogChain           { return n }
func (n *NoOpLogChain) Any(key string, val interface{}) LogChain             { return n }
func (n *NoOpLogChain) Err(err error) LogChain                               { return n }
func (n *NoOpLogChain) Stack() LogChain                                      { return n }
func (n *NoOpLogChain) Caller() LogChain                                     { return n }
func (n *NoOpLogChain) With(key string, val interface{}) LogChain            { return n }
func (n *NoOpLogChain) WithFields(fields map[string]interface{}) LogChain    { return n }
func (n *NoOpLogChain) ToConsole() LogChain                                  { return n }
func (n *NoOpLogChain) ToFile(filename string) LogChain                      { return n }
func (n *NoOpLogChain) ToDB() LogChain                                       { return n }
func (n *NoOpLogChain) ToMQ(topic string) LogChain                           { return n }
func (n *NoOpLogChain) WithDatabase(driver DatabaseDriver) LogChain          { return n }
func (n *NoOpLogChain) Log()                                                 {}
func (n *NoOpLogChain) Logf(format string, args ...interface{})             {}