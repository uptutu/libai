package libai

import (
	"encoding/json"
	"fmt"
)

// LegacyWrapper provides backward compatibility with the existing Logger API
type LegacyWrapper struct {
	chainLogger *ChainLoggerImpl
}

// Debug implements the legacy Debug method
func (l *LegacyWrapper) Debug(action string, flag string, content any) {
	if l.chainLogger.level > DebugLevel {
		return
	}

	// Convert content to JSON string for backward compatibility
	contentStr := l.convertContent(content)

	l.chainLogger.Debug().
		Str("action", action).
		Str("flag", flag).
		Str("content", contentStr).
		Msg(action).
		Log()
}

// Info implements the legacy Info method
func (l *LegacyWrapper) Info(action string, flag string, content any) {
	if l.chainLogger.level > InfoLevel {
		return
	}

	// Convert content to JSON string for backward compatibility
	contentStr := l.convertContent(content)

	l.chainLogger.Info().
		Str("action", action).
		Str("flag", flag).
		Str("content", contentStr).
		Msg(action).
		Log()
}

// Warn implements the legacy Warn method
func (l *LegacyWrapper) Warn(action string, flag string, content any) {
	if l.chainLogger.level > WarnLevel {
		return
	}

	// Convert content to JSON string for backward compatibility
	contentStr := l.convertContent(content)

	l.chainLogger.Warn().
		Str("action", action).
		Str("flag", flag).
		Str("content", contentStr).
		Msg(action).
		Log()
}

// Error implements the legacy Error method
func (l *LegacyWrapper) Error(action string, flag string, content any) {
	if l.chainLogger.level > ErrorLevel {
		return
	}

	// Convert content to JSON string for backward compatibility
	contentStr := l.convertContent(content)

	l.chainLogger.Error().
		Str("action", action).
		Str("flag", flag).
		Str("content", contentStr).
		Msg(action).
		Log()
}

// Fatal implements the legacy Fatal method
func (l *LegacyWrapper) Fatal(action string, flag string, content any) {
	// Convert content to JSON string for backward compatibility
	contentStr := l.convertContent(content)

	l.chainLogger.Fatal().
		Str("action", action).
		Str("flag", flag).
		Str("content", contentStr).
		Msg(action).
		Log()
}

// Z returns a LoggerZapWrapper for backward compatibility
func (l *LegacyWrapper) Z() *LoggerZapWrapper {
	return &LoggerZapWrapper{
		chainLogger: l.chainLogger,
	}
}

// Zap returns a zap logger interface for backward compatibility
func (l *LegacyWrapper) Zap() interface{} {
	// For backward compatibility, we'll create a minimal zap-like interface
	// This is a simplified implementation to avoid direct zap dependency
	return &ZapCompatibilityWrapper{
		chainLogger: l.chainLogger,
	}
}

// convertContent converts the content parameter to a JSON string
func (l *LegacyWrapper) convertContent(content any) string {
	if content == nil {
		return ""
	}

	// Handle string content directly
	if str, ok := content.(string); ok {
		return str
	}

	// Convert to JSON for complex types
	data, err := json.Marshal(content)
	if err != nil {
		return fmt.Sprintf("failed to marshal content: %v", err)
	}

	return string(data)
}

// LoggerZapWrapper provides zap-style logging with either legacy or chain logger
type LoggerZapWrapper struct {
	chainLogger *ChainLoggerImpl
	logger      *Logger // for backward compatibility with original Logger
}

// Warn implements zap-style Warn
func (z *LoggerZapWrapper) Warn(action string, flag string, content any) {
	// Handle legacy logger case
	if z.logger != nil {
		if !z.logger.isDebug && WarnLevel < InfoLevel {
			return
		}
		z.logger.Warn(action, flag, content)
		return
	}

	// Handle new chain logger case
	if z.chainLogger != nil && z.chainLogger.level > WarnLevel {
		return
	}

	contentStr := z.convertContent(content)

	z.chainLogger.Warn().
		Str("action", action).
		Str("flag", flag).
		Str("content", contentStr).
		Msg(action).
		Log()
}

// Debug implements zap-style Debug
func (z *LoggerZapWrapper) Debug(action string, flag string, content any) {
	// Handle legacy logger case
	if z.logger != nil {
		if !z.logger.isDebug {
			return
		}
		z.logger.Debug(action, flag, content)
		return
	}

	// Handle new chain logger case
	if z.chainLogger != nil && z.chainLogger.level > DebugLevel {
		return
	}

	contentStr := z.convertContent(content)

	z.chainLogger.Debug().
		Str("action", action).
		Str("flag", flag).
		Str("content", contentStr).
		Msg(action).
		Log()
}

// Error implements zap-style Error
func (z *LoggerZapWrapper) Error(action string, flag string, content any) {
	// Handle legacy logger case
	if z.logger != nil {
		z.logger.Error(action, flag, content)
		return
	}

	// Handle new chain logger case
	if z.chainLogger != nil && z.chainLogger.level > ErrorLevel {
		return
	}

	contentStr := z.convertContent(content)

	z.chainLogger.Error().
		Str("action", action).
		Str("flag", flag).
		Str("content", contentStr).
		Msg(action).
		Log()
}

// Info implements zap-style Info
func (z *LoggerZapWrapper) Info(action string, flag string, content any) {
	// Handle legacy logger case
	if z.logger != nil {
		z.logger.Info(action, flag, content)
		return
	}

	// Handle new chain logger case
	if z.chainLogger != nil && z.chainLogger.level > InfoLevel {
		return
	}

	contentStr := z.convertContent(content)

	z.chainLogger.Info().
		Str("action", action).
		Str("flag", flag).
		Str("content", contentStr).
		Msg(action).
		Log()
}

// convertContent converts the content parameter to a JSON string
func (z *LoggerZapWrapper) convertContent(content any) string {
	if content == nil {
		return ""
	}

	// Handle string content directly
	if str, ok := content.(string); ok {
		return str
	}

	// Convert to JSON for complex types
	data, err := json.Marshal(content)
	if err != nil {
		return fmt.Sprintf("failed to marshal content: %v", err)
	}

	return string(data)
}

// ZapCompatibilityWrapper provides a minimal zap-compatible interface
type ZapCompatibilityWrapper struct {
	chainLogger *ChainLoggerImpl
}

// Info provides basic zap.Info compatibility
func (z *ZapCompatibilityWrapper) Info(msg string, fields ...interface{}) {
	chain := z.chainLogger.Info().Msg(msg)

	// Handle pairs of key-value fields
	for i := 0; i+1 < len(fields); i += 2 {
		if key, ok := fields[i].(string); ok {
			chain = chain.Any(key, fields[i+1])
		}
	}

	chain.Log()
}

// Warn provides basic zap.Warn compatibility
func (z *ZapCompatibilityWrapper) Warn(msg string, fields ...interface{}) {
	chain := z.chainLogger.Warn().Msg(msg)

	// Handle pairs of key-value fields
	for i := 0; i+1 < len(fields); i += 2 {
		if key, ok := fields[i].(string); ok {
			chain = chain.Any(key, fields[i+1])
		}
	}

	chain.Log()
}

// Error provides basic zap.Error compatibility
func (z *ZapCompatibilityWrapper) Error(msg string, fields ...interface{}) {
	chain := z.chainLogger.Error().Msg(msg)

	// Handle pairs of key-value fields
	for i := 0; i+1 < len(fields); i += 2 {
		if key, ok := fields[i].(string); ok {
			chain = chain.Any(key, fields[i+1])
		}
	}

	chain.Log()
}

// Debug provides basic zap.Debug compatibility
func (z *ZapCompatibilityWrapper) Debug(msg string, fields ...interface{}) {
	chain := z.chainLogger.Debug().Msg(msg)

	// Handle pairs of key-value fields
	for i := 0; i+1 < len(fields); i += 2 {
		if key, ok := fields[i].(string); ok {
			chain = chain.Any(key, fields[i+1])
		}
	}

	chain.Log()
}
