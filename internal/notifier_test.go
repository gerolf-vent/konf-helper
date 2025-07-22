package internal

import (
	"sync"

	"go.uber.org/zap/zapcore"
)

// MockNotifier is a test implementation of the Notifier interface
type MockNotifier struct {
	NotifyCallCount int
	NotifyResult    bool
	mu              sync.Mutex
}

func (m *MockNotifier) Notify() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.NotifyCallCount++
	return m.NotifyResult
}

func (m *MockNotifier) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.NotifyCallCount = 0
}

func (m *MockNotifier) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("type", "mock")
	return nil
}
