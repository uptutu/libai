package libai

import (
	"context"
	"errors"
	"time"
)

// MockOutputPlugin is a mock implementation for testing
type MockOutputPlugin struct {
	name       string
	delay      time.Duration
	shouldFail bool
	writeCount int64
}

// Write implements the OutputPlugin interface
func (m *MockOutputPlugin) Write(ctx context.Context, entry *LogEntry) error {
	m.writeCount++
	
	// Simulate processing delay if specified
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	
	// Simulate failure if configured
	if m.shouldFail {
		return errors.New("mock output plugin failure")
	}
	
	return nil
}

// Close implements the OutputPlugin interface
func (m *MockOutputPlugin) Close() error {
	return nil
}

// Name implements the OutputPlugin interface
func (m *MockOutputPlugin) Name() string {
	return m.name
}

// GetStats implements the OutputPlugin interface
func (m *MockOutputPlugin) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"name":        m.name,
		"write_count": m.writeCount,
		"should_fail": m.shouldFail,
		"delay":       m.delay.String(),
	}
}

// GetWriteCount returns the number of writes performed (for testing)
func (m *MockOutputPlugin) GetWriteCount() int64 {
	return m.writeCount
}

// SetShouldFail configures whether the plugin should fail
func (m *MockOutputPlugin) SetShouldFail(shouldFail bool) {
	m.shouldFail = shouldFail
}

// SetDelay configures the processing delay
func (m *MockOutputPlugin) SetDelay(delay time.Duration) {
	m.delay = delay
}