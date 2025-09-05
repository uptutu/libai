package libai

import (
	"fmt"
	"os"
	"time"
)

func DemoZapIntegration() {
	fmt.Println("=== libai 链式日志库 zap 集成演示 ===")

	// 1. 控制台彩色文本输出
	fmt.Println("1. 控制台彩色文本输出:")
	textConfig := DefaultConfig()
	textConfig.Origin = "demo-app"
	textConfig.Level = DebugLevel
	textConfig.Console.Enabled = true
	textConfig.Console.Format = "text"
	textConfig.Console.Colorized = true
	textConfig.EnableStack = true

	textLogger, _ := NewChainLogger(textConfig)

	textLogger.Debug().Msg("系统调试信息").Str("component", "auth").Int("user_count", 1250).Log()
	textLogger.Info().Msg("用户登录成功").Str("user", "张三").Str("ip", "192.168.1.100").Time("login_time", time.Now()).Log()
	textLogger.Warn().Msg("磁盘空间不足").Float64("usage", 85.6).Str("disk", "/var/log").Bool("auto_cleanup", true).Log()
	textLogger.Error().Msg("数据库连接失败").Str("host", "db.example.com").Int("retry_count", 3).Stack().Log()

	textLogger.Close()

	fmt.Println("\n2. 控制台 JSON 格式输出:")
	// 2. 控制台 JSON 输出
	jsonConfig := DefaultConfig()
	jsonConfig.Origin = "api-service"
	jsonConfig.Console.Enabled = true
	jsonConfig.Console.Format = "json"
	jsonConfig.Console.Colorized = false

	jsonLogger, _ := NewChainLogger(jsonConfig)

	jsonLogger.Info().
		Msg("API请求处理完成").
		Str("method", "POST").
		Str("endpoint", "/api/users").
		Int("status_code", 201).
		Dur("response_time", 45*time.Millisecond).
		Any("request_body", map[string]interface{}{
			"name":  "李四",
			"email": "lisi@example.com",
		}).
		Log()

	jsonLogger.Close()

	fmt.Println("\n3. 文件输出演示:")
	// 3. 文件输出
	fileConfig := DefaultConfig()
	fileConfig.Origin = "file-service"
	fileConfig.Console.Enabled = false
	fileConfig.File.Enabled = true
	fileConfig.File.Filename = "/tmp/libai-demo.log"
	fileConfig.File.Format = "json"

	fileLogger, _ := NewChainLogger(fileConfig)

	fileLogger.Info().
		Msg("数据处理任务完成").
		Str("task_id", "task_12345").
		Int("processed_records", 50000).
		Float64("processing_time_seconds", 123.45).
		Bool("success", true).
		Log()

	fileLogger.Close()

	// 检查文件是否创建
	if _, err := os.Stat("/tmp/libai-demo.log"); err == nil {
		fmt.Println("✓ 日志已成功写入文件 /tmp/libai-demo.log")
	} else {
		fmt.Println("✗ 文件写入失败")
	}

	fmt.Println("\n4. 向后兼容性演示:")
	// 4. 向后兼容性
	legacyConfig := DefaultConfig()
	legacyConfig.Origin = "legacy-app"
	legacyConfig.Console.Enabled = true
	legacyConfig.Console.Format = "text"
	legacyConfig.Console.Colorized = true

	chainLogger, _ := NewChainLogger(legacyConfig)
	legacyLogger := chainLogger.Legacy()

	// 使用旧的 API 风格
	legacyLogger.Info("user_registration", "success", map[string]interface{}{
		"user_id":     12345,
		"username":    "王五",
		"register_ip": "192.168.1.200",
	})

	legacyLogger.Error("payment_error", "card_declined", map[string]interface{}{
		"transaction_id": "tx_98765",
		"amount":         99.99,
		"card_last4":     "1234",
	})

	chainLogger.Close()

	fmt.Println("\n5. 上下文字段演示:")
	// 5. 上下文字段
	contextConfig := DefaultConfig()
	contextConfig.Origin = "context-app"
	contextConfig.Console.Enabled = true
	contextConfig.Console.Format = "text"
	contextConfig.Console.Colorized = true

	contextLogger, _ := NewChainLogger(contextConfig)

	// 创建带上下文的日志器
	sessionLogger := contextLogger.WithFields(map[string]interface{}{
		"session_id": "sess_abc123",
		"user_id":    789,
		"request_id": "req_xyz789",
	})

	sessionLogger.Info().Msg("开始处理用户请求").Str("action", "profile_update").Log()
	sessionLogger.Warn().Msg("参数验证警告").Str("field", "phone").Str("value", "invalid").Log()
	sessionLogger.Info().Msg("用户资料更新成功").Bool("email_changed", true).Bool("phone_changed", false).Log()

	contextLogger.Close()

	fmt.Println("\n=== zap 集成演示完成 ===")
}
