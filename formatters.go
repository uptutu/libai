package libai

import (
	"context"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// ZapConsoleOutputPlugin implements the OutputPlugin interface for console output using zap
type ZapConsoleOutputPlugin struct {
	config ConsoleConfig
	logger *zap.Logger
}

// NewConsoleOutputPlugin creates a new console output plugin using zap
func NewConsoleOutputPlugin(config ConsoleConfig) OutputPlugin {
	zapConfig := createZapConfig(config)
	logger, err := zapConfig.Build()
	if err != nil {
		// Fallback to a basic logger if configuration fails
		logger = zap.NewNop()
	}
	
	return &ZapConsoleOutputPlugin{
		config: config,
		logger: logger,
	}
}

// Write writes a log entry to the console using zap
func (z *ZapConsoleOutputPlugin) Write(ctx context.Context, entry *LogEntry) error {
	// Convert LogEntry to zap fields
	fields := z.convertToZapFields(entry)
	
	// Log using appropriate zap level
	switch entry.Level {
	case DebugLevel:
		z.logger.Debug(entry.Message, fields...)
	case InfoLevel:
		z.logger.Info(entry.Message, fields...)
	case WarnLevel:
		z.logger.Warn(entry.Message, fields...)
	case ErrorLevel:
		z.logger.Error(entry.Message, fields...)
	case FatalLevel:
		z.logger.Fatal(entry.Message, fields...)
	default:
		z.logger.Info(entry.Message, fields...)
	}
	
	return nil
}

// Close closes the console output plugin
func (z *ZapConsoleOutputPlugin) Close() error {
	// Ignore sync errors for stdout/stderr as they're not real files
	_ = z.logger.Sync()
	return nil
}

// Name returns the name of the plugin
func (z *ZapConsoleOutputPlugin) Name() string {
	return "console"
}

// createZapConfig creates a zap configuration based on ConsoleConfig
func createZapConfig(config ConsoleConfig) zap.Config {
	var zapConfig zap.Config
	
	if strings.ToLower(config.Format) == "json" {
		zapConfig = zap.NewProductionConfig()
	} else {
		zapConfig = zap.NewDevelopmentConfig()
		if config.Colorized {
			zapConfig.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		}
	}
	
	// Configure output paths
	zapConfig.OutputPaths = []string{"stdout"}
	zapConfig.ErrorOutputPaths = []string{"stderr"}
	
	// Configure timestamp format
	if config.TimeFormat != "" {
		zapConfig.EncoderConfig.TimeKey = "timestamp"
		zapConfig.EncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout(config.TimeFormat)
	} else {
		zapConfig.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	}
	
	return zapConfig
}

// convertToZapFields converts LogEntry fields to zap fields
func (z *ZapConsoleOutputPlugin) convertToZapFields(entry *LogEntry) []zap.Field {
	fields := make([]zap.Field, 0)
	
	// Add origin
	if entry.Origin != "" {
		fields = append(fields, zap.String("origin", entry.Origin))
	}
	
	// Add custom fields
	for k, v := range entry.Fields {
		fields = append(fields, zap.Any(k, v))
	}
	
	// Add error if present
	if entry.Error != nil {
		fields = append(fields, zap.Error(entry.Error))
	}
	
	// Add caller if present
	if entry.Caller != "" {
		fields = append(fields, zap.String("caller", entry.Caller))
	}
	
	// Add stack trace if present
	if entry.StackTrace != "" {
		fields = append(fields, zap.String("stack_trace", entry.StackTrace))
	}
	
	// Add legacy fields for backward compatibility
	if entry.Action != "" {
		fields = append(fields, zap.String("action", entry.Action))
	}
	if entry.Flag != "" {
		fields = append(fields, zap.String("flag", entry.Flag))
	}
	
	return fields
}

// ZapFileOutputPlugin implements the OutputPlugin interface for file output using zap with rotation
type ZapFileOutputPlugin struct {
	config     FileConfig
	logger     *zap.Logger
	lumberjack *lumberjack.Logger
}

// NewFileOutputPlugin creates a new file output plugin using zap with lumberjack rotation
func NewFileOutputPlugin(config FileConfig) OutputPlugin {
	filename := config.Filename
	if filename == "" {
		filename = "app.log"
	}

	// Create lumberjack logger for rotation
	lumberjackLogger := &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    int(config.MaxSize / (1024 * 1024)), // Convert bytes to MB
		MaxAge:     config.MaxAge,                       // days
		MaxBackups: config.MaxBackups,                   // Number of old files to keep
		Compress:   config.Compress,                     // Compress rotated files
		LocalTime:  config.LocalTime,                    // Use local time for rotation
	}

	// Build logger with custom core and lumberjack writer
	logger := buildZapLoggerWithLumberjack(config, lumberjackLogger)

	return &ZapFileOutputPlugin{
		config:     config,
		logger:     logger,
		lumberjack: lumberjackLogger,
	}
}

// buildZapLoggerWithLumberjack builds a zap logger with lumberjack rotation
func buildZapLoggerWithLumberjack(config FileConfig, lumberjackLogger *lumberjack.Logger) *zap.Logger {
	// Create encoder config
	encoderConfig := zap.NewProductionEncoderConfig()
	
	// Configure timestamp format
	if config.TimeFormat != "" {
		encoderConfig.TimeKey = "timestamp"
		encoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout(config.TimeFormat)
	} else {
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	}
	
	// Configure other encoder settings
	encoderConfig.LevelKey = "level"
	encoderConfig.MessageKey = "message"
	encoderConfig.CallerKey = "caller"
	encoderConfig.StacktraceKey = "stacktrace"
	
	// Create appropriate encoder
	var encoder zapcore.Encoder
	if strings.ToLower(config.Format) == "json" {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}
	
	// Create core with lumberjack writer
	writeSyncer := zapcore.AddSync(lumberjackLogger)
	core := zapcore.NewCore(encoder, writeSyncer, zap.DebugLevel)
	
	// Build logger with caller info
	logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zap.ErrorLevel))
	
	return logger
}

// Write writes a log entry to file using zap
func (z *ZapFileOutputPlugin) Write(ctx context.Context, entry *LogEntry) error {
	// Convert LogEntry to zap fields
	fields := z.convertToZapFields(entry)
	
	// Log using appropriate zap level
	switch entry.Level {
	case DebugLevel:
		z.logger.Debug(entry.Message, fields...)
	case InfoLevel:
		z.logger.Info(entry.Message, fields...)
	case WarnLevel:
		z.logger.Warn(entry.Message, fields...)
	case ErrorLevel:
		z.logger.Error(entry.Message, fields...)
	case FatalLevel:
		z.logger.Fatal(entry.Message, fields...)
	default:
		z.logger.Info(entry.Message, fields...)
	}
	
	return nil
}

// convertToZapFields converts LogEntry fields to zap fields for file output
func (z *ZapFileOutputPlugin) convertToZapFields(entry *LogEntry) []zap.Field {
	fields := make([]zap.Field, 0)
	
	// Add origin
	if entry.Origin != "" {
		fields = append(fields, zap.String("origin", entry.Origin))
	}
	
	// Add custom fields
	for k, v := range entry.Fields {
		fields = append(fields, zap.Any(k, v))
	}
	
	// Add error if present
	if entry.Error != nil {
		fields = append(fields, zap.Error(entry.Error))
	}
	
	// Add caller if present
	if entry.Caller != "" {
		fields = append(fields, zap.String("caller", entry.Caller))
	}
	
	// Add stack trace if present
	if entry.StackTrace != "" {
		fields = append(fields, zap.String("stack_trace", entry.StackTrace))
	}
	
	// Add legacy fields for backward compatibility
	if entry.Action != "" {
		fields = append(fields, zap.String("action", entry.Action))
	}
	if entry.Flag != "" {
		fields = append(fields, zap.String("flag", entry.Flag))
	}
	
	// Add timestamp
	fields = append(fields, zap.Time("timestamp", entry.Timestamp))
	
	return fields
}

// Close closes the file output plugin and ensures all data is flushed
func (z *ZapFileOutputPlugin) Close() error {
	if z.logger != nil {
		// Sync the logger to ensure all data is written
		if err := z.logger.Sync(); err != nil {
			// Ignore sync errors for regular files, but log them
			// (sync errors on regular files are often not critical)
		}
	}
	
	if z.lumberjack != nil {
		// Close the lumberjack logger to finalize current log file
		return z.lumberjack.Close()
	}
	
	return nil
}

// Name returns the name of the plugin
func (z *ZapFileOutputPlugin) Name() string {
	return "file"
}

// Rotate manually rotates the log file (useful for external triggers)
func (z *ZapFileOutputPlugin) Rotate() error {
	if z.lumberjack == nil {
		return nil
	}
	return z.lumberjack.Rotate()
}

// GetCurrentLogFile returns the current log file path
func (z *ZapFileOutputPlugin) GetCurrentLogFile() string {
	if z.lumberjack == nil {
		return ""
	}
	return z.lumberjack.Filename
}