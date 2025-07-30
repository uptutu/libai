package libai

import (
	"fmt"
	"log"
)

func main() {
	// 创建带有堆栈信息的日志器
	logger, err := NewLoggerBuilder().
		SetOrigin("example-app").
		SetDebugMode().
		SetStackTrace(true). // 启用堆栈信息
		Build()

	if err != nil {
		log.Fatal("Failed to create logger:", err)
	}

	// 测试不同级别的日志
	fmt.Println("=== 测试普通日志器 ===")

	// Info 级别不会打印堆栈信息
	logger.Info("user_login", "success", map[string]interface{}{
		"user_id": 12345,
		"ip":      "192.168.1.1",
	})

	// Error 级别会打印堆栈信息
	logger.Error("database_error", "connection_failed", map[string]interface{}{
		"error": "connection timeout",
		"host":  "localhost:27017",
	})

	// Warn 级别会打印堆栈信息
	logger.Warn("performance_warning", "slow_query", map[string]interface{}{
		"query_time": "2.5s",
		"query":      "SELECT * FROM users",
	})

	// Debug 级别会打印堆栈信息
	logger.Debug("debug_info", "variable_state", map[string]interface{}{
		"variable": "user_count",
		"value":    100,
	})

	fmt.Println("\n=== 测试 Zap 包装器 ===")

	// 使用 Zap 包装器
	zapLogger := logger.Z()

	// Info 级别不会打印堆栈信息
	zapLogger.Info("zap_info", "test", "This is info level")

	// Error 级别会打印堆栈信息
	zapLogger.Error("zap_error", "test", "This is error level")

	// Warn 级别会打印堆栈信息
	zapLogger.Warn("zap_warn", "test", "This is warn level")
}

// 演示如何创建不启用堆栈信息的日志器
func createLoggerWithoutStack() {
	logger, err := NewLoggerBuilder().
		SetOrigin("no-stack-app").
		SetDebugMode().
		SetStackTrace(false). // 不启用堆栈信息
		Build()

	if err != nil {
		log.Fatal("Failed to create logger:", err)
	}

	// 即使是 Error 级别也不会打印堆栈信息
	logger.Error("test_error", "no_stack", "This error won't have stack trace")
}
