package libai

import (
	"encoding/json"
	"fmt"
	"time"
)

// MessageSerializer implementations for different formats
type JSONMessageSerializer struct{}
type AvroMessageSerializer struct{}
type ProtobufMessageSerializer struct{}
type MessagePackSerializer struct{}

// NewMessageSerializer creates a new message serializer based on format
func NewMessageSerializer(format MessageFormat) (MessageSerializer, error) {
	switch format {
	case JSONFormat:
		return &JSONMessageSerializer{}, nil
	case AvroFormat:
		return &AvroMessageSerializer{}, nil
	case ProtobufFormat:
		return &ProtobufMessageSerializer{}, nil
	case MessagePackFormat:
		return &MessagePackSerializer{}, nil
	default:
		return &JSONMessageSerializer{}, nil // Default to JSON
	}
}

// JSONMessageSerializer implementation
func (s *JSONMessageSerializer) Serialize(entry *LogEntry) ([]byte, error) {
	// Create a message envelope with metadata and payload
	envelope := map[string]interface{}{
		"timestamp": entry.Timestamp.Format(time.RFC3339Nano),
		"level":     entry.Level.String(),
		"message":   entry.Message,
		"origin":    entry.Origin,
		"logger":    "libai",
		"version":   "1.0.0",
	}
	
	// Add optional fields
	if entry.Action != "" {
		envelope["action"] = entry.Action
	}
	
	if entry.Flag != "" {
		envelope["flag"] = entry.Flag
	}
	
	if entry.Caller != "" {
		envelope["caller"] = entry.Caller
	}
	
	if entry.StackTrace != "" {
		envelope["stack_trace"] = entry.StackTrace
	}
	
	if entry.Error != nil {
		envelope["error"] = entry.Error.Error()
	}
	
	// Add custom fields
	if len(entry.Fields) > 0 {
		envelope["fields"] = entry.Fields
	}
	
	// Add metadata
	metadata := map[string]interface{}{
		"serializer":    "json",
		"schema_version": "1.0",
		"created_at":     time.Now().Unix(),
	}
	envelope["metadata"] = metadata
	
	return json.Marshal(envelope)
}

func (s *JSONMessageSerializer) GetFormat() MessageFormat {
	return JSONFormat
}

// AvroMessageSerializer implementation (placeholder - would need Avro schema)
func (s *AvroMessageSerializer) Serialize(entry *LogEntry) ([]byte, error) {
	// For now, fallback to JSON serialization
	// In a real implementation, this would use Apache Avro schema
	jsonSerializer := &JSONMessageSerializer{}
	data, err := jsonSerializer.Serialize(entry)
	if err != nil {
		return nil, err
	}
	
	// Add Avro envelope (simplified)
	envelope := map[string]interface{}{
		"schema_id": 1,
		"data":     string(data),
		"format":   "avro",
	}
	
	return json.Marshal(envelope)
}

func (s *AvroMessageSerializer) GetFormat() MessageFormat {
	return AvroFormat
}

// ProtobufMessageSerializer implementation (placeholder - would need proto definitions)
func (s *ProtobufMessageSerializer) Serialize(entry *LogEntry) ([]byte, error) {
	// For now, fallback to JSON serialization
	// In a real implementation, this would use Protocol Buffers
	jsonSerializer := &JSONMessageSerializer{}
	data, err := jsonSerializer.Serialize(entry)
	if err != nil {
		return nil, err
	}
	
	// Add Protobuf envelope (simplified)
	envelope := map[string]interface{}{
		"proto_version": "3.0",
		"data":         string(data),
		"format":       "protobuf",
	}
	
	return json.Marshal(envelope)
}

func (s *ProtobufMessageSerializer) GetFormat() MessageFormat {
	return ProtobufFormat
}

// MessagePackSerializer implementation (placeholder - would need MessagePack library)
func (s *MessagePackSerializer) Serialize(entry *LogEntry) ([]byte, error) {
	// For now, fallback to JSON serialization
	// In a real implementation, this would use MessagePack
	jsonSerializer := &JSONMessageSerializer{}
	data, err := jsonSerializer.Serialize(entry)
	if err != nil {
		return nil, err
	}
	
	// Add MessagePack envelope (simplified)
	envelope := map[string]interface{}{
		"msgpack_version": "1.0",
		"data":           string(data),
		"format":         "msgpack",
	}
	
	return json.Marshal(envelope)
}

func (s *MessagePackSerializer) GetFormat() MessageFormat {
	return MessagePackFormat
}

// MessageRouter implementations
type DefaultMessageRouter struct {
	config RoutingConfig
}

type LevelBasedMessageRouter struct {
	config RoutingConfig
}

type FieldBasedMessageRouter struct {
	config RoutingConfig
}

// RoutingConfig contains message routing configuration
type RoutingConfig struct {
	Strategy    string                 `json:"strategy" yaml:"strategy"`         // "default", "level", "field"
	TopicPrefix string                 `json:"topic_prefix" yaml:"topic_prefix"` // Optional prefix for all topics
	LevelTopics map[string]string      `json:"level_topics" yaml:"level_topics"` // level -> topic mapping
	FieldRules  []FieldRoutingRule     `json:"field_rules" yaml:"field_rules"`   // Field-based routing rules
	Partitions  int                    `json:"partitions" yaml:"partitions"`     // Number of partitions
	Headers     map[string]interface{} `json:"headers" yaml:"headers"`           // Default headers
}

// FieldRoutingRule defines field-based routing rules
type FieldRoutingRule struct {
	Field     string      `json:"field" yaml:"field"`         // Field name to check
	Value     interface{} `json:"value" yaml:"value"`         // Value to match
	Topic     string      `json:"topic" yaml:"topic"`         // Target topic
	Condition string      `json:"condition" yaml:"condition"` // "equals", "contains", "regex"
}

// NewMessageRouter creates a new message router
func NewMessageRouter(config RoutingConfig) (MessageRouter, error) {
	switch config.Strategy {
	case "level":
		return &LevelBasedMessageRouter{config: config}, nil
	case "field":
		return &FieldBasedMessageRouter{config: config}, nil
	default:
		return &DefaultMessageRouter{config: config}, nil
	}
}

// DefaultMessageRouter implementation
func (r *DefaultMessageRouter) Route(entry *LogEntry, topic string) (*MQMessage, error) {
	// Apply topic prefix if configured
	if r.config.TopicPrefix != "" {
		topic = r.config.TopicPrefix + "." + topic
	}
	
	// Create message with routing key based on origin and level
	key := fmt.Sprintf("%s.%s", entry.Origin, entry.Level.String())
	
	message := &MQMessage{
		Topic:     topic,
		Key:       key,
		Headers:   make(map[string]string),
		Timestamp: entry.Timestamp,
		Priority:  r.getLevelPriority(entry.Level),
		Metadata: map[string]interface{}{
			"routing_strategy": "default",
			"router_version":   "1.0",
		},
	}
	
	// Add default headers
	for k, v := range r.config.Headers {
		if strVal, ok := v.(string); ok {
			message.Headers[k] = strVal
		}
	}
	
	// Add standard headers
	message.Headers["level"] = entry.Level.String()
	message.Headers["origin"] = entry.Origin
	message.Headers["timestamp"] = entry.Timestamp.Format(time.RFC3339)
	
	if entry.Action != "" {
		message.Headers["action"] = entry.Action
	}
	
	if entry.Flag != "" {
		message.Headers["flag"] = entry.Flag
	}
	
	return message, nil
}

func (r *DefaultMessageRouter) GetPartition(message *MQMessage, partitionCount int) int32 {
	if partitionCount <= 1 {
		return 0
	}
	
	// Simple hash-based partitioning using message key
	hash := int32(0)
	for _, b := range []byte(message.Key) {
		hash = hash*31 + int32(b)
	}
	
	return hash % int32(partitionCount)
}

// LevelBasedMessageRouter implementation
func (r *LevelBasedMessageRouter) Route(entry *LogEntry, topic string) (*MQMessage, error) {
	// Check if there's a specific topic for this level
	levelTopic, exists := r.config.LevelTopics[entry.Level.String()]
	if exists {
		topic = levelTopic
	}
	
	// Apply topic prefix if configured
	if r.config.TopicPrefix != "" {
		topic = r.config.TopicPrefix + "." + topic
	}
	
	// Create message
	message := &MQMessage{
		Topic:     topic,
		Key:       fmt.Sprintf("%s.%s", entry.Origin, entry.Level.String()),
		Headers:   make(map[string]string),
		Timestamp: entry.Timestamp,
		Priority:  r.getLevelPriority(entry.Level),
		Metadata: map[string]interface{}{
			"routing_strategy": "level",
			"router_version":   "1.0",
		},
	}
	
	// Add headers
	message.Headers["level"] = entry.Level.String()
	message.Headers["origin"] = entry.Origin
	message.Headers["routing"] = "level_based"
	
	return message, nil
}

func (r *LevelBasedMessageRouter) GetPartition(message *MQMessage, partitionCount int) int32 {
	if partitionCount <= 1 {
		return 0
	}
	
	// Partition by level to ensure ordering within levels
	levelHash := map[string]int32{
		"debug": 0,
		"info":  1,
		"warn":  2,
		"error": 3,
		"fatal": 4,
	}
	
	if hash, exists := levelHash[message.Headers["level"]]; exists {
		return hash % int32(partitionCount)
	}
	
	return 0
}

// FieldBasedMessageRouter implementation
func (r *FieldBasedMessageRouter) Route(entry *LogEntry, topic string) (*MQMessage, error) {
	// Apply field-based routing rules
	for _, rule := range r.config.FieldRules {
		if r.matchRule(entry, rule) {
			topic = rule.Topic
			break
		}
	}
	
	// Apply topic prefix if configured
	if r.config.TopicPrefix != "" {
		topic = r.config.TopicPrefix + "." + topic
	}
	
	// Create message with field-based routing key
	key := r.generateFieldBasedKey(entry)
	
	message := &MQMessage{
		Topic:     topic,
		Key:       key,
		Headers:   make(map[string]string),
		Timestamp: entry.Timestamp,
		Priority:  r.getLevelPriority(entry.Level),
		Metadata: map[string]interface{}{
			"routing_strategy": "field",
			"router_version":   "1.0",
		},
	}
	
	// Add headers based on fields
	message.Headers["level"] = entry.Level.String()
	message.Headers["origin"] = entry.Origin
	message.Headers["routing"] = "field_based"
	
	// Add relevant field values as headers
	for fieldName, fieldValue := range entry.Fields {
		if strVal, ok := fieldValue.(string); ok && len(strVal) < 256 {
			message.Headers["field_"+fieldName] = strVal
		}
	}
	
	return message, nil
}

func (r *FieldBasedMessageRouter) GetPartition(message *MQMessage, partitionCount int) int32 {
	if partitionCount <= 1 {
		return 0
	}
	
	// Hash based on message key
	hash := int32(0)
	for _, b := range []byte(message.Key) {
		hash = hash*31 + int32(b)
	}
	
	return hash % int32(partitionCount)
}

// Helper methods
func (r *DefaultMessageRouter) getLevelPriority(level LogLevel) int {
	priorities := map[LogLevel]int{
		DebugLevel: 1,
		InfoLevel:  2,
		WarnLevel:  3,
		ErrorLevel: 4,
		FatalLevel: 5,
	}
	
	if priority, exists := priorities[level]; exists {
		return priority
	}
	
	return 2 // Default to info priority
}

func (r *LevelBasedMessageRouter) getLevelPriority(level LogLevel) int {
	return (&DefaultMessageRouter{}).getLevelPriority(level)
}

func (r *FieldBasedMessageRouter) getLevelPriority(level LogLevel) int {
	return (&DefaultMessageRouter{}).getLevelPriority(level)
}

func (r *FieldBasedMessageRouter) matchRule(entry *LogEntry, rule FieldRoutingRule) bool {
	fieldValue, exists := entry.Fields[rule.Field]
	if !exists {
		return false
	}
	
	switch rule.Condition {
	case "equals":
		return fieldValue == rule.Value
	case "contains":
		if strVal, ok := fieldValue.(string); ok {
			if strRule, ok := rule.Value.(string); ok {
				return fmt.Sprintf("%v", strVal) == strRule
			}
		}
	case "regex":
		// Would implement regex matching in a real implementation
		return false
	default:
		return fieldValue == rule.Value
	}
	
	return false
}

func (r *FieldBasedMessageRouter) generateFieldBasedKey(entry *LogEntry) string {
	// Generate key based on relevant fields
	key := fmt.Sprintf("%s.%s", entry.Origin, entry.Level.String())
	
	// Add important field values to the key
	importantFields := []string{"user_id", "session_id", "request_id", "service", "component"}
	
	for _, fieldName := range importantFields {
		if value, exists := entry.Fields[fieldName]; exists {
			if strVal, ok := value.(string); ok {
				key += "." + strVal
			}
		}
	}
	
	return key
}