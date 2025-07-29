package libai

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type LogModel struct {
	Level      string `bson:"level"`
	Origin     string `bson:"origin"`      // 来源程序
	Action     string `bson:"action"`      // 动作
	Flag       string `bson:"flag"`        // 标识
	CreateTime int    `bson:"create_time"` // 时间
	Content    string `bson:"content"`     // 详细内容
}

type Logger struct {
	origin     string
	mongo      *mongo.Client
	database   string
	collection string
	isDebug    bool
}

func (l *Logger) log(level, action, flag string, content any) {
	c, err := json.Marshal(content)
	if err != nil {
		fmt.Printf("log content not json: %v\n", err)
		return
	}

	if l.isDebug {
		fmt.Printf("[%s] %s ||  %s\n", level, action, string(c))
	}

	coll := l.mongo.Database(l.database).Collection(l.collection)
	record := LogModel{
		Level:      level,
		Action:     action,
		Origin:     l.origin,
		Flag:       flag,
		Content:    string(c),
		CreateTime: int(time.Now().Unix()), // Assuming time is set in context
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

type LoggerBuilder struct {
	mongo struct {
		host       string
		port       int
		username   string
		password   string
		database   string
		collection string
	}
	origin  string
	isDebug bool
}

func NewLoggerBuilder() *LoggerBuilder {
	return &LoggerBuilder{}
}
func (b LoggerBuilder) mongoToDSN() string {
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

func (b *LoggerBuilder) Build() (*Logger, error) {
	if b.origin == "" {
		return nil, fmt.Errorf("origin must be set")
	}

	clientOptions := options.Client().
		ApplyURI(b.mongoToDSN())

	client, err := mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	if err = client.Ping(context.TODO(), nil); err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	logger := &Logger{
		origin:     b.origin,
		mongo:      client,
		database:   b.mongo.database,
		collection: b.mongo.collection,
		isDebug:    b.isDebug,
	}

	return logger, nil
}
