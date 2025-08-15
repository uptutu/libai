package libai

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

type LogModel struct {
	Level      string `bson:"level"`
	Origin     string `bson:"origin"`      // 来源程序
	Action     string `bson:"action"`      // 动作
	Flag       string `bson:"flag"`        // 标识
	CreateTime int    `bson:"create_time"` // 时间
	Content    string `bson:"content"`     // 详细内容
	StackTrace string `bson:"stack_trace"` // 堆栈信息
}

type Logger struct {
	origin      string
	mongo       *mongo.Client
	database    string
	collection  string
	isDebug     bool
	enableStack bool        // 是否启用非info级别日志的堆栈信息
	zapLogger   *zap.Logger // Optional: if you want to use zap for logging
	wrap        *LoggerZapWrapper
}

// getStackTrace 获取堆栈信息
func (l *Logger) getStackTrace() string {
	if !l.enableStack {
		return ""
	}

	buf := make([]byte, 1024*4)
	n := runtime.Stack(buf, false)
	stack := string(buf[:n])

	// 过滤掉当前函数和log函数的堆栈信息
	lines := strings.Split(stack, "\n")
	var filteredLines []string
	skip := 0
	for i, line := range lines {
		if strings.Contains(line, "getStackTrace") || strings.Contains(line, ".log(") {
			skip = i + 2 // 跳过函数名和文件行号
			continue
		}
		if i > skip {
			filteredLines = append(filteredLines, line)
		}
	}

	return strings.Join(filteredLines, "\n")
}

func (l *Logger) log(level, action, flag string, content any) {
	c, err := json.Marshal(content)
	if err != nil {
		fmt.Printf("log content not json: %v, stack: %s\n", err, l.getStackTrace())
		return
	}

	// 获取堆栈信息（仅对非info级别的日志）
	var stackTrace string
	if level != "info" {
		stackTrace = l.getStackTrace()
	}

	if l.isDebug {
		fmt.Printf("[%s] %s ||  %s\n", level, action, string(c))
		if stackTrace != "" {
			fmt.Printf("Stack trace:\n%s\n", stackTrace)
		}
	}

	if l.mongo == nil {
		return
	}

	coll := l.mongo.Database(l.database).Collection(l.collection)
	record := LogModel{
		Level:      level,
		Action:     action,
		Origin:     l.origin,
		Flag:       flag,
		Content:    string(c),
		StackTrace: stackTrace,
		CreateTime: int(time.Now().Unix()),
	}

	if _, err = coll.InsertOne(context.TODO(), record); err != nil {
		fmt.Println("Failed to insert log record:", err)
		fmt.Printf("[%s] %s %s\n", level, action, content)
		return
	}
}

func (l *Logger) Warn(action string, flag string, content any) {
	l.log("warn", action, flag, content)
}

func (l *Logger) Debug(action string, flag string, content any) {
	if !l.isDebug {
		return
	}
	l.log("debug", action, flag, content)
}

func (l *Logger) Error(action string, flag string, content any) {
	l.log("error", action, flag, content)
}

func (l *Logger) Info(action string, flag string, content any) {
	l.log("info", action, flag, content)
}

func (l *Logger) Z() *LoggerZapWrapper {
	if l.wrap == nil {
		l.wrap = &LoggerZapWrapper{Logger: l}
	}
	return l.wrap
}

func (l *Logger) Zap() *zap.Logger {
	if l.zapLogger == nil {
		if l.isDebug {
			l.zapLogger, _ = zap.NewDevelopment()
		} else {
			l.zapLogger, _ = zap.NewProduction()
		}
	}
	return l.zapLogger
}

type LoggerBuilder struct {
	mongo struct {
		host       string
		port       int
		username   string
		password   string
		database   string
		collection string
	}
	origin      string
	isDebug     bool
	enableStack bool // 是否启用非info级别日志的堆栈信息
}

func NewLoggerBuilder() *LoggerBuilder {
	return &LoggerBuilder{}
}
func (b *LoggerBuilder) mongoToDSN() string {
	if b.mongo.username == "" && b.mongo.password == "" {
		return fmt.Sprintf("mongodb://%s:%d", b.mongo.host, b.mongo.port)
	}
	return fmt.Sprintf("mongodb://%s:%s@%s:%d", b.mongo.username, b.mongo.password, b.mongo.host, b.mongo.port)
}

func (b *LoggerBuilder) SetMongo(host string, port int, username, password, database, collection string) *LoggerBuilder {
	b.mongo.host = host
	b.mongo.port = port
	b.mongo.username = username
	b.mongo.password = password
	b.mongo.database = database
	b.mongo.collection = collection
	return b
}

func (b *LoggerBuilder) SetOrigin(origin string) *LoggerBuilder {
	b.origin = origin
	return b
}

func (b *LoggerBuilder) SetDebugMode() *LoggerBuilder {
	b.isDebug = true
	return b
}

// SetStackTrace 设置是否启用非info级别日志的堆栈信息
func (b *LoggerBuilder) SetStackTrace(enable bool) *LoggerBuilder {
	b.enableStack = enable
	return b
}

func (b *LoggerBuilder) Build() (*Logger, error) {
	if b.origin == "" {
		return nil, fmt.Errorf("origin must be set")
	}

	var client *mongo.Client
	var err error

	if b.mongo.host != "" {
		clientOptions := options.Client().
			ApplyURI(b.mongoToDSN())

		client, err = mongo.Connect(context.TODO(), clientOptions)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
		}

		if err = client.Ping(context.TODO(), nil); err != nil {
			return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
		}
	}

	zapL, _ := zap.NewDevelopment()
	if !b.isDebug {
		zapL, _ = zap.NewProduction()
	}

	logger := &Logger{
		origin:      b.origin,
		mongo:       client,
		database:    b.mongo.database,
		collection:  b.mongo.collection,
		isDebug:     b.isDebug,
		enableStack: b.enableStack,
		zapLogger:   zapL,
	}

	return logger, nil
}

type LoggerZapWrapper struct {
	*Logger
}

func (l *LoggerZapWrapper) Warn(action string, flag string, content any) {
	fields := []zap.Field{zap.String("flag", flag), zap.Any("content", content)}
	if l.enableStack {
		fields = append(fields, zap.String("stack_trace", l.getStackTrace()))
	}
	l.zapLogger.Warn(action, fields...)
}

func (l *LoggerZapWrapper) Debug(action string, flag string, content any) {
	if !l.isDebug {
		return
	}
	fields := []zap.Field{zap.String("flag", flag), zap.Any("content", content)}
	if l.enableStack {
		fields = append(fields, zap.String("stack_trace", l.getStackTrace()))
	}
	l.zapLogger.Debug(action, fields...)
}

func (l *LoggerZapWrapper) Error(action string, flag string, content any) {
	fields := []zap.Field{zap.String("flag", flag), zap.Any("content", content)}
	if l.enableStack {
		fields = append(fields, zap.String("stack_trace", l.getStackTrace()))
	}
	l.zapLogger.Error(action, fields...)
}

func (l *LoggerZapWrapper) Info(action string, flag string, content any) {
	l.zapLogger.Info(action, zap.String("flag", flag), zap.Any("content", content))
}
