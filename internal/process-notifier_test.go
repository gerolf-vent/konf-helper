package internal

import (
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

func TestNewProcessNotifier(t *testing.T) {
	processName := "test-process"
	signal := syscall.SIGUSR1

	pn := NewProcessNotifier(processName, signal)

	if pn == nil {
		t.Fatal("NewProcessNotifier() returned nil")
	}
	if pn.processName != processName {
		t.Errorf("processName = %v, want %v", pn.processName, processName)
	}
	if pn.signal != signal {
		t.Errorf("signal = %v, want %v", pn.signal, signal)
	}
}

func TestFindPIDNotFound(t *testing.T) {
	// Use a process name that definitely won't exist
	pn := NewProcessNotifier("non-existent-process-name-12345", syscall.SIGUSR1)

	pid, err := pn.findPID()
	if err == nil {
		t.Error("findPID() should return error for non-existent process")
	}
	if pid != 0 {
		t.Errorf("findPID() pid = %d, want 0 for non-existent process", pid)
	}

	expectedError := "process \"non-existent-process-name-12345\" not found"
	if err.Error() != expectedError {
		t.Errorf("findPID() error = %v, want %v", err.Error(), expectedError)
	}
}

func TestFindPIDCurrentProcess(t *testing.T) {
	// Get the current process name from /proc/self/comm
	commData, err := os.ReadFile("/proc/self/comm")
	if err != nil {
		t.Skipf("Cannot read /proc/self/comm: %v", err)
	}

	processName := strings.TrimSpace(string(commData))
	pn := NewProcessNotifier(processName, syscall.SIGUSR1)

	pid, err := pn.findPID()
	if err != nil {
		t.Errorf("findPID() error = %v for current process", err)
	}

	currentPID := os.Getpid()
	if pid != currentPID {
		t.Errorf("findPID() pid = %d, want %d (current process)", pid, currentPID)
	}
}

func TestNotifyNonExistentProcess(t *testing.T) {
	pn := NewProcessNotifier("non-existent-process", syscall.SIGUSR1)

	ok := pn.Notify()
	if ok {
		t.Error("Notify() should return error for non-existent process")
	}
}

func TestNotifyCurrentProcess(t *testing.T) {
	// Get the current process name
	commData, err := os.ReadFile("/proc/self/comm")
	if err != nil {
		t.Skipf("Cannot read /proc/self/comm: %v", err)
	}

	processName := strings.TrimSpace(string(commData))

	// Use a harmless signal that we can safely send to ourselves
	pn := NewProcessNotifier(processName, syscall.Signal(0)) // Signal 0 is a null signal

	ok := pn.Notify()
	if !ok {
		t.Errorf("Notify() has error for current process with null signal")
	}
}

func TestFindPIDWithInvalidProcDir(t *testing.T) {
	// This test is tricky because we can't easily mock the filesystem
	// We'll test by creating a ProcessNotifier and testing edge cases
	pn := NewProcessNotifier("init", syscall.SIGUSR1)

	// Test that we can at least call findPID without panicking
	_, err := pn.findPID()
	// We don't assert on the result because init might or might not exist
	// depending on the test environment, but the function should not panic
	_ = err // Ignore the error for this test
}

func TestFindPIDErrorHandling(t *testing.T) {
	// Create a temporary test to verify error handling paths
	tests := []struct {
		name        string
		processName string
		expectError bool
	}{
		{
			name:        "empty process name",
			processName: "",
			expectError: true,
		},
		{
			name:        "process with special characters",
			processName: "test-proc-with-dashes",
			expectError: true, // Likely won't exist
		},
		{
			name:        "very long process name",
			processName: strings.Repeat("a", 100),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pn := NewProcessNotifier(tt.processName, syscall.SIGUSR1)
			_, err := pn.findPID()

			if tt.expectError && err == nil {
				t.Error("findPID() should return error")
			}
		})
	}
}

// Container-specific tests

func TestFindPIDInContainer(t *testing.T) {
	// Check for container environment
	inContainer := false
	if _, err := os.Stat("/.dockerenv"); err == nil {
		inContainer = true
	} else if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		content := string(data)
		if strings.Contains(content, "docker") ||
			strings.Contains(content, "containerd") ||
			strings.Contains(content, "podman") {
			inContainer = true
		}
	}

	if !inContainer {
		t.Skip("Skipping container-specific test: not running in container")
	}

	t.Run("find_init_process", func(t *testing.T) {
		// In containers, PID 1 is often the main application
		// Let's see what process is running as PID 1
		commData, err := os.ReadFile("/proc/1/comm")
		if err != nil {
			t.Skipf("Cannot read /proc/1/comm: %v", err)
		}

		initProcessName := strings.TrimSpace(string(commData))
		t.Logf("Container init process (PID 1): %s", initProcessName)

		pn := NewProcessNotifier(initProcessName, syscall.SIGUSR1)
		pid, err := pn.findPID()

		if err != nil {
			t.Errorf("Should find init process: %v", err)
		} else if pid != 1 {
			t.Errorf("Expected PID 1 for init process, got %d", pid)
		}
	})

	t.Run("find_current_process_in_container", func(t *testing.T) {
		// Test finding the current test process
		commData, err := os.ReadFile("/proc/self/comm")
		if err != nil {
			t.Skipf("Cannot read /proc/self/comm: %v", err)
		}

		currentProcessName := strings.TrimSpace(string(commData))
		currentPID := os.Getpid()

		pn := NewProcessNotifier(currentProcessName, syscall.SIGUSR1)
		foundPID, err := pn.findPID()

		if err != nil {
			t.Errorf("Should find current process: %v", err)
		} else if foundPID != currentPID {
			t.Logf("Found PID %d, current PID %d (might be multiple processes with same name)", foundPID, currentPID)
		}
	})
}

func TestNotifyInContainer(t *testing.T) {
	// Check for container environment
	inContainer := false
	if _, err := os.Stat("/.dockerenv"); err == nil {
		inContainer = true
	} else if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		content := string(data)
		if strings.Contains(content, "docker") ||
			strings.Contains(content, "containerd") ||
			strings.Contains(content, "podman") {
			inContainer = true
		}
	}

	if !inContainer {
		t.Skip("Skipping container-specific test: not running in container")
	}

	// Test notifying the current process (safe test)
	commData, err := os.ReadFile("/proc/self/comm")
	if err != nil {
		t.Skipf("Cannot read /proc/self/comm: %v", err)
	}

	currentProcessName := strings.TrimSpace(string(commData))
	pn := NewProcessNotifier(currentProcessName, syscall.SIGUSR1)

	// This should work without error (sending signal to self)
	if ok := pn.Notify(); !ok {
		t.Errorf("Notify() to self should not fail")
	}
}

func TestContainerProcessDiscovery(t *testing.T) {
	// Check for container environment
	inContainer := false
	if _, err := os.Stat("/.dockerenv"); err == nil {
		inContainer = true
	} else if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		content := string(data)
		if strings.Contains(content, "docker") ||
			strings.Contains(content, "containerd") ||
			strings.Contains(content, "podman") {
			inContainer = true
		}
	}

	if !inContainer {
		t.Skip("Skipping container-specific test: not running in container")
	}

	t.Run("list_container_processes", func(t *testing.T) {
		// Read all processes in the container
		entries, err := os.ReadDir("/proc")
		if err != nil {
			t.Fatalf("Cannot read /proc: %v", err)
		}

		var processes []string
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			// Check if it's a numeric directory (PID)
			if _, err := strconv.Atoi(entry.Name()); err != nil {
				continue
			}

			// Read the process name
			commPath := "/proc/" + entry.Name() + "/comm"
			if commData, err := os.ReadFile(commPath); err == nil {
				processName := strings.TrimSpace(string(commData))
				processes = append(processes, processName)
			}
		}

		t.Logf("Found %d processes in container", len(processes))

		// Log first few processes for debugging
		maxLog := 10
		if len(processes) > maxLog {
			t.Logf("First %d processes: %v", maxLog, processes[:maxLog])
		} else {
			t.Logf("All processes: %v", processes)
		}

		// Basic validation - should have at least the current test process
		if len(processes) == 0 {
			t.Error("Should find at least one process in container")
		}
	})

	t.Run("container_runtime_info", func(t *testing.T) {
		t.Logf("Runtime OS: %s", runtime.GOOS)
		t.Logf("Current PID: %d", os.Getpid())
		t.Logf("Parent PID: %d", os.Getppid())

		// Check if we're running as PID 1 (container init)
		if os.Getpid() == 1 {
			t.Log("Running as PID 1 - this is the container init process")
		}

		// Check cgroup info to understand container environment
		if cgroupData, err := os.ReadFile("/proc/self/cgroup"); err == nil {
			lines := strings.Split(string(cgroupData), "\n")
			t.Logf("Container cgroup info (first few lines):")
			maxLines := 3
			for i, line := range lines {
				if i >= maxLines || line == "" {
					break
				}
				t.Logf("  %s", line)
			}
		}
	})
}

func TestProcessNotifierMarshalLogObject(t *testing.T) {
	tests := []struct {
		name        string
		processName string
		signal      syscall.Signal
		want        map[string]interface{}
	}{
		{
			name:        "SIGUSR1",
			processName: "test-process",
			signal:      syscall.SIGUSR1,
			want: map[string]interface{}{
				"process-name": "test-process",
				"signal":       "user defined signal 1",
			},
		},
		{
			name:        "SIGTERM",
			processName: "another-process",
			signal:      syscall.SIGTERM,
			want: map[string]interface{}{
				"process-name": "another-process",
				"signal":       "terminated",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pn := NewProcessNotifier(tt.processName, tt.signal)
			enc := NewMockObjectEncoder()

			err := pn.MarshalLogObject(enc)
			if err != nil {
				t.Fatalf("MarshalLogObject() error = %v", err)
			}

			for key, expectedValue := range tt.want {
				if actualValue, exists := enc.Fields[key]; !exists {
					t.Errorf("Expected field %s not found", key)
				} else if actualValue != expectedValue {
					t.Errorf("Field %s: got %v, want %v", key, actualValue, expectedValue)
				}
			}

			// Check that no unexpected fields are present
			for key := range enc.Fields {
				if _, expected := tt.want[key]; !expected {
					t.Errorf("Unexpected field %s with value %v", key, enc.Fields[key])
				}
			}
		})
	}
}

func TestProcessNotifierGetters(t *testing.T) {
	processName := "test-process"
	signal := syscall.SIGUSR2

	pn := NewProcessNotifier(processName, signal)

	if got := pn.ProcessName(); got != processName {
		t.Errorf("ProcessName() = %v, want %v", got, processName)
	}

	if got := pn.Signal(); got != signal {
		t.Errorf("Signal() = %v, want %v", got, signal)
	}
}
