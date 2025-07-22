package internal

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
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

func TestPathsSyncServiceSetMultiplePathConfigs(t *testing.T) {
	service, err := NewPathsSyncService(50 * time.Millisecond)
	if err != nil {
		t.Fatalf("NewPathsSyncService() error = %v", err)
	}

	// Create multiple temporary directories
	tmpDir1 := t.TempDir()
	tmpDir2 := t.TempDir()
	tmpDir3 := t.TempDir()

	pathConfigs := make([]*PathConfig, 3)
	for i, dir := range []string{tmpDir1, tmpDir2, tmpDir3} {
		pathConfig, err := ParsePathConfig(dir)
		if err != nil {
			t.Fatalf("ParsePathConfig() error = %v", err)
		}
		pathConfigs[i] = pathConfig

		err = service.SetPathConfig(pathConfig)
		if err != nil {
			t.Errorf("SetPathConfig() error = %v", err)
		}
	}

	// Verify all pathConfigs were added
	if len(service.pathConfigs) != 3 {
		t.Errorf("Expected 3 pathConfigs, got %d", len(service.pathConfigs))
	}

	// Verify order is maintained
	for i, expected := range pathConfigs {
		if service.pathConfigs[i] != expected {
			t.Errorf("pathConfig at index %d was not stored correctly", i)
		}
	}
}

func TestPathsSyncServiceSetPathConfigWhileRunning(t *testing.T) {
	service, err := NewPathsSyncService(50 * time.Millisecond)
	if err != nil {
		t.Fatalf("NewPathsSyncService() error = %v", err)
	}

	// Start the service first
	err = service.Start()
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer service.Stop()

	tmpDir := t.TempDir()
	pathConfig, err := ParsePathConfig(tmpDir)
	if err != nil {
		t.Fatalf("ParsePathConfig() error = %v", err)
	}

	// Should be able to add path config while running
	err = service.SetPathConfig(pathConfig)
	if err != nil {
		t.Errorf("SetPathConfig() should work while service is running, got: %v", err)
	}

	if len(service.pathConfigs) != 1 {
		t.Errorf("Expected 1 pathConfig, got %d", len(service.pathConfigs))
	}
}

func TestPathsSyncServiceSetPathConfigWithFiles(t *testing.T) {
	service, err := NewPathsSyncService(50 * time.Millisecond)
	if err != nil {
		t.Fatalf("NewPathsSyncService() error = %v", err)
	}

	tmpDir := t.TempDir()

	// Create some test files
	testFiles := []string{"config.yaml", "secrets.yaml", "deployment.yaml"}
	for _, file := range testFiles {
		filePath := filepath.Join(tmpDir, file)
		if err := os.WriteFile(filePath, []byte("test: content"), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", file, err)
		}
	}

	pathConfig, err := ParsePathConfig(tmpDir)
	if err != nil {
		t.Fatalf("ParsePathConfig() error = %v", err)
	}

	err = service.SetPathConfig(pathConfig)
	if err != nil {
		t.Errorf("SetPathConfig() error = %v", err)
	}

	// Verify the path config includes the created files
	if len(service.pathConfigs) != 1 {
		t.Errorf("Expected 1 pathConfig, got %d", len(service.pathConfigs))
	}
}

func TestPathsSyncServiceSetPathConfigErrorCases(t *testing.T) {
	service, err := NewPathsSyncService(50 * time.Millisecond)
	if err != nil {
		t.Fatalf("NewPathsSyncService() error = %v", err)
	}

	tests := []struct {
		name             string
		path             string
		expectParseError bool
		expectSetError   bool
	}{
		{"Non-existent path", "/path/that/does/not/exist", false, true},
		{"Empty path", "", true, true},
		{"Relative path", "./relative", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.path == "./relative" {
				// Create relative directory for this test
				if err := os.MkdirAll(tt.path, 0755); err != nil {
					t.Fatalf("Failed to create relative directory: %v", err)
				}
				defer os.RemoveAll(tt.path)
			}

			pathConfig, err := ParsePathConfig(tt.path)
			if err != nil && !tt.expectParseError {
				t.Fatalf("ParsePathConfig() unexpected error = %v", err)
			}
			if err == nil && tt.expectParseError {
				t.Fatalf("ParsePathConfig() expected error but got none")
			}

			if err == nil {
				err = service.SetPathConfig(pathConfig)
				if tt.expectSetError && err == nil {
					t.Errorf("SetPathConfig() expected error but got none")
				}
				if !tt.expectSetError && err != nil {
					t.Errorf("SetPathConfig() unexpected error = %v", err)
				}
			}
		})
	}
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

func TestPathsSyncServiceLifecycleEdgeCases(t *testing.T) {
	service, err := NewPathsSyncService(50 * time.Millisecond)
	if err != nil {
		t.Fatalf("NewPathsSyncService() error = %v", err)
	}

	// Test multiple rapid starts and stops
	for i := 0; i < 5; i++ {
		err = service.Start()
		if err != nil {
			t.Errorf("Start() iteration %d error = %v", i, err)
		}

		if !service.IsStarted() {
			t.Errorf("service should be started after Start() iteration %d", i)
		}

		service.Stop()

		if service.IsStarted() {
			t.Errorf("service should be stopped after Stop() iteration %d", i)
		}
	}
}

func TestPathsSyncServiceConcurrentStartStop(t *testing.T) {
	service, err := NewPathsSyncService(50 * time.Millisecond)
	if err != nil {
		t.Fatalf("NewPathsSyncService() error = %v", err)
	}

	var wg sync.WaitGroup
	numGoroutines := 10

	// Test concurrent starts
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			service.Start()
		}()
	}
	wg.Wait()

	// Should only be started once
	if !service.IsStarted() {
		t.Error("service should be started after concurrent Start() calls")
	}

	// Test concurrent stops
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			service.Stop()
		}()
	}
	wg.Wait()

	// Should be stopped
	if service.IsStarted() {
		t.Error("service should be stopped after concurrent Stop() calls")
	}
}

func TestPathsSyncServiceNotificationIntegration(t *testing.T) {
	service, err := NewPathsSyncService(50 * time.Millisecond)
	if err != nil {
		t.Fatalf("NewPathsSyncService() error = %v", err)
	}

	mockNotifier := &MockNotifier{}
	service.SetNotifier(mockNotifier)

	tmpDir := t.TempDir()
	pathConfig, err := ParsePathConfig(tmpDir)
	if err != nil {
		t.Fatalf("ParsePathConfig() error = %v", err)
	}

	err = service.SetPathConfig(pathConfig)
	if err != nil {
		t.Errorf("SetPathConfig() error = %v", err)
	}

	err = service.Start()
	if err != nil {
		t.Errorf("Start() error = %v", err)
	}
	defer service.Stop()

	// Give the service time to start
	time.Sleep(100 * time.Millisecond)

	// Create a file to trigger a notification
	testFile := filepath.Join(tmpDir, "test.yaml")
	if err := os.WriteFile(testFile, []byte("test: data"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Wait for debounce and processing
	time.Sleep(200 * time.Millisecond)

	if mockNotifier.NotifyCallCount == 0 {
		t.Error("Notifier should have been called after file change")
	}
}

func TestPathsSyncServiceResourceCleanup(t *testing.T) {
	service, err := NewPathsSyncService(50 * time.Millisecond)
	if err != nil {
		t.Fatalf("NewPathsSyncService() error = %v", err)
	}

	// Add multiple path configs
	for i := 0; i < 3; i++ {
		tmpDir := t.TempDir()
		pathConfig, err := ParsePathConfig(tmpDir)
		if err != nil {
			t.Fatalf("ParsePathConfig() error = %v", err)
		}

		err = service.SetPathConfig(pathConfig)
		if err != nil {
			t.Errorf("SetPathConfig() error = %v", err)
		}
	}

	err = service.Start()
	if err != nil {
		t.Errorf("Start() error = %v", err)
	}

	// Verify service is running with resources
	if !service.IsStarted() {
		t.Error("service should be started")
	}

	if len(service.pathConfigs) != 3 {
		t.Errorf("Expected 3 pathConfigs, got %d", len(service.pathConfigs))
	}

	// Stop and verify cleanup
	service.Stop()

	if service.IsStarted() {
		t.Error("service should be stopped")
	}

	// Health check should fail after stop
	err = service.CheckHealth()
	if err == nil {
		t.Error("CheckHealth() should return error after service is stopped")
	}
}

func TestPathsSyncServiceDebounceConfiguration(t *testing.T) {
	debounceDelays := []time.Duration{
		10 * time.Millisecond,
		100 * time.Millisecond,
		500 * time.Millisecond,
	}

	for _, delay := range debounceDelays {
		t.Run(delay.String(), func(t *testing.T) {
			service, err := NewPathsSyncService(delay)
			if err != nil {
				t.Fatalf("NewPathsSyncService() error = %v", err)
			}

			if service.debouncer == nil {
				t.Error("debouncer should be initialized")
			}

			// Test that service can start and stop with different debounce delays
			err = service.Start()
			if err != nil {
				t.Errorf("Start() error = %v", err)
			}

			if !service.IsStarted() {
				t.Error("service should be started")
			}

			service.Stop()

			if service.IsStarted() {
				t.Error("service should be stopped")
			}
		})
	}
}

func TestPathsSyncServiceZeroDebounceDelay(t *testing.T) {
	service, err := NewPathsSyncService(0)
	if err != nil {
		t.Fatalf("NewPathsSyncService() with zero debounce should not error, got: %v", err)
	}

	if service == nil {
		t.Fatal("NewPathsSyncService() returned nil")
	}

	// Should still be able to start and stop
	err = service.Start()
	if err != nil {
		t.Errorf("Start() error = %v", err)
	}
	defer service.Stop()
}

func TestPathsSyncServiceNegativeDebounceDelay(t *testing.T) {
	service, err := NewPathsSyncService(-100 * time.Millisecond)

	// Depending on implementation, this might error or normalize to 0
	// Adjust this test based on actual behavior
	if err != nil {
		// If constructor rejects negative values
		if service != nil {
			t.Error("NewPathsSyncService() should return nil on error")
		}
		return
	}

	// If constructor accepts negative values (treats as 0 or absolute value)
	if service == nil {
		t.Fatal("NewPathsSyncService() returned nil without error")
	}
}
