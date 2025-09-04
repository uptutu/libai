package libai

import (
	"context"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
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

// ZapFileOutputPlugin implements the OutputPlugin interface for file output using zap
type ZapFileOutputPlugin struct {
	config FileConfig
	logger *zap.Logger
}

// NewFileOutputPlugin creates a new file output plugin using zap
func NewFileOutputPlugin(config FileConfig) OutputPlugin {
	zapConfig := createZapFileConfig(config)
	logger, err := zapConfig.Build()
	if err != nil {
		// Fallback to a basic logger if configuration fails
		logger = zap.NewNop()
	}
	
	return &ZapFileOutputPlugin{
		config: config,
		logger: logger,
	}
}

// createZapFileConfig creates a zap configuration for file output
func createZapFileConfig(config FileConfig) zap.Config {
	zapConfig := zap.NewProductionConfig()
	
	// Configure file output
	filename := config.Filename
	if filename == "" {
		filename = "app.log"
	}
	
	zapConfig.OutputPaths = []string{filename}
	zapConfig.ErrorOutputPaths = []string{filename}
	
	// Configure format
	if strings.ToLower(config.Format) == "json" {
		zapConfig.Encoding = "json"
	} else {
		zapConfig.Encoding = "console"
	}
	
	// Configure timestamp format
	if config.TimeFormat != "" {
		zapConfig.EncoderConfig.TimeKey = "timestamp"
		zapConfig.EncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout(config.TimeFormat)
	} else {
		zapConfig.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	}
	
	return zapConfig
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

// Close closes the file output plugin
func (z *ZapFileOutputPlugin) Close() error {
	// For file outputs, sync is important but may fail, so we'll log but not return error
	_ = z.logger.Sync()
	return nil
}

// Name returns the name of the plugin
func (z *ZapFileOutputPlugin) Name() string {
	return "file"
}