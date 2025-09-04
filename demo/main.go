package main

import (
	"fmt"
	libai "local.git/libs/libai.git"
)

func main() {
	fmt.Println("=== libai 链式日志库 完整演示 ===")

	// 演示 zap 集成
	libai.DemoZapIntegration()
	
	// 演示文件输出增强功能
	fmt.Println()
	libai.DemoFileOutputEnhancements()
	
	// 简单的对比演示
	fmt.Println("\n=== 新老 API 对比 ===")
	
	// 创建链式日志器
	config := libai.DefaultConfig()
	config.Origin = "demo-comparison"
	config.Level = libai.DebugLevel
	config.Console.Enabled = true
	config.Console.Format = "text"
	config.Console.Colorized = true
	
	chainLogger, _ := libai.NewChainLogger(config)
	
	fmt.Println("\n旧版 API 风格:")
	legacyLogger := chainLogger.Legacy()
	legacyLogger.Info("user_action", "login", map[string]interface{}{
		"user_id": 123,
		"method": "password",
	})
	
	fmt.Println("\n新版链式 API 风格:")
	chainLogger.Info().
		Msg("用户操作").
		Str("action", "login").
		Int("user_id", 123).
		Str("method", "password").
		Log()
	
	chainLogger.Close()
	
	fmt.Println("\n=== 演示完成 ===")
}