package internal

import (
	"testing"
	"time"

	"go.uber.org/zap/zapcore"
)

func TestNewPathsSyncService(t *testing.T) {
	debounceDelay := 100 * time.Millisecond
	service, err := NewPathsSyncService(debounceDelay)

	if err != nil {
		t.Fatalf("NewPathsSyncService() error = %v", err)
	}

	if service == nil {
		t.Fatal("NewPathsSyncService() returned nil")
	}

	if service.fsWatcher == nil {
		t.Error("fsWatcher should be initialized")
	}

	if service.patchEngine == nil {
		t.Error("patchEngine should be initialized")
	}

	if service.debouncer == nil {
		t.Error("debouncer should be initialized")
	}

	if service.logger == nil {
		t.Error("logger should be initialized")
	}

	if service.isStarted {
		t.Error("service should not be started initially")
	}
}

func TestPathsSyncServiceSetPathConfig(t *testing.T) {
	service, err := NewPathsSyncService(50 * time.Millisecond)
	if err != nil {
		t.Fatalf("NewPathsSyncService() error = %v", err)
	}

	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	pathConfig, err := ParsePathConfig(tmpDir)
	if err != nil {
		t.Fatalf("ParsePathConfig() error = %v", err)
	}

	err = service.SetPathConfig(pathConfig)
	if err != nil {
		t.Errorf("SetPathConfig() error = %v", err)
	}

	// Verify pathConfig was added
	if len(service.pathConfigs) != 1 {
		t.Errorf("Expected 1 pathConfig, got %d", len(service.pathConfigs))
	}

	if service.pathConfigs[0] != pathConfig {
		t.Error("pathConfig was not stored correctly")
	}
}

func TestPathsSyncServiceSetNotifyer(t *testing.T) {
	service, err := NewPathsSyncService(50 * time.Millisecond)
	if err != nil {
		t.Fatalf("NewPathsSyncService() error = %v", err)
	}

	// Create a mock notifyer
	mockNotifyer := &MockNotifyer{}

	service.SetNotifyer(mockNotifyer)

	// We can't directly check the notifyer field since it's private,
	// but we can check that the method doesn't panic
}

func TestPathsSyncServiceStartStop(t *testing.T) {
	service, err := NewPathsSyncService(50 * time.Millisecond)
	if err != nil {
		t.Fatalf("NewPathsSyncService() error = %v", err)
	}

	// Test Start
	err = service.Start()
	if err != nil {
		t.Errorf("Start() error = %v", err)
	}

	if !service.IsStarted() {
		t.Error("service should be started after Start()")
	}

	// Test starting again (should not error)
	err = service.Start()
	if err != nil {
		t.Errorf("Start() called twice should not error, got %v", err)
	}

	// Test Stop
	service.Stop()

	if service.IsStarted() {
		t.Error("service should not be started after Stop()")
	}

	// Test stopping again (should not panic)
	service.Stop()
}

func TestPathsSyncServiceIsStarted(t *testing.T) {
	service, err := NewPathsSyncService(50 * time.Millisecond)
	if err != nil {
		t.Fatalf("NewPathsSyncService() error = %v", err)
	}

	// Initially not started
	if service.IsStarted() {
		t.Error("service should not be started initially")
	}

	// Start the service
	err = service.Start()
	if err != nil {
		t.Errorf("Start() error = %v", err)
	}

	if !service.IsStarted() {
		t.Error("service should be started after Start()")
	}

	// Stop the service
	service.Stop()

	if service.IsStarted() {
		t.Error("service should not be started after Stop()")
	}
}

func TestPathsSyncServiceCheckHealth(t *testing.T) {
	service, err := NewPathsSyncService(50 * time.Millisecond)
	if err != nil {
		t.Fatalf("NewPathsSyncService() error = %v", err)
	}

	// Test health check when not started
	err = service.CheckHealth()
	if err == nil {
		t.Error("CheckHealth() should return error when service is not started")
	}

	// Start the service
	err = service.Start()
	if err != nil {
		t.Errorf("Start() error = %v", err)
	}
	defer service.Stop()

	// Test health check when started
	// Give it a moment to start the goroutine
	time.Sleep(10 * time.Millisecond)

	err = service.CheckHealth()
	if err != nil {
		t.Errorf("CheckHealth() should not return error when service is started, got: %v", err)
	}
}

// MockNotifyer is a test implementation of the Notifyer interface
type MockNotifyer struct {
	NotifyCallCount int
	NotifyResult    bool
}

func (m *MockNotifyer) Notify() bool {
	m.NotifyCallCount++
	return m.NotifyResult
}

func (m *MockNotifyer) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("type", "mock")
	enc.AddInt("callCount", m.NotifyCallCount)
	enc.AddBool("result", m.NotifyResult)
	return nil
}

func TestPathsSyncServiceWithInvalidPath(t *testing.T) {
	service, err := NewPathsSyncService(50 * time.Millisecond)
	if err != nil {
		t.Fatalf("NewPathsSyncService() error = %v", err)
	}

	// Try to set a non-existent path
	pathConfig, err := ParsePathConfig("/non/existent/path")
	if err != nil {
		t.Fatalf("ParsePathConfig() error = %v", err)
	}

	err = service.SetPathConfig(pathConfig)
	if err == nil {
		t.Error("SetPathConfig() should error for non-existent path")
	}
}

func TestPathsSyncServiceConcurrentAccess(t *testing.T) {
	service, err := NewPathsSyncService(50 * time.Millisecond)
	if err != nil {
		t.Fatalf("NewPathsSyncService() error = %v", err)
	}

	// Test concurrent start/stop calls (should not panic or race)
	done := make(chan bool, 2)

	go func() {
		defer func() { done <- true }()
		for i := 0; i < 10; i++ {
			service.Start()
			time.Sleep(1 * time.Millisecond)
		}
	}()

	go func() {
		defer func() { done <- true }()
		for i := 0; i < 10; i++ {
			service.Stop()
			time.Sleep(1 * time.Millisecond)
		}
	}()

	// Wait for both goroutines to complete
	<-done
	<-done
}
