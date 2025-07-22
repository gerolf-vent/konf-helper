package internal

import (
	"sync"
	"testing"
	"time"
)

func TestNewDebouncer(t *testing.T) {
	delay := 100 * time.Millisecond
	debouncer := NewDebouncer(delay)

	if debouncer == nil {
		t.Fatal("NewDebouncer() returned nil")
	}
	if debouncer.delay != delay {
		t.Errorf("NewDebouncer() delay = %v, want %v", debouncer.delay, delay)
	}
	if debouncer.timer != nil {
		t.Error("timer should be nil initially")
	}
}

func TestDebouncerCall(t *testing.T) {
	delay := 50 * time.Millisecond
	debouncer := NewDebouncer(delay)

	callCount := 0
	var mu sync.Mutex

	fn := func() {
		mu.Lock()
		callCount++
		mu.Unlock()
	}

	// Call multiple times quickly
	debouncer.Call(fn)
	debouncer.Call(fn)
	debouncer.Call(fn)

	// Wait for the delay to pass
	time.Sleep(delay + 10*time.Millisecond)

	mu.Lock()
	finalCount := callCount
	mu.Unlock()

	// Function should only be called once due to debouncing
	if finalCount != 1 {
		t.Errorf("Expected function to be called once, but was called %d times", finalCount)
	}
}

func TestDebouncerMultipleCalls(t *testing.T) {
	delay := 30 * time.Millisecond
	debouncer := NewDebouncer(delay)

	callCount := 0
	var mu sync.Mutex

	fn := func() {
		mu.Lock()
		callCount++
		mu.Unlock()
	}

	// First batch of calls
	debouncer.Call(fn)
	debouncer.Call(fn)

	// Wait for first call to execute
	time.Sleep(delay + 10*time.Millisecond)

	mu.Lock()
	firstCount := callCount
	mu.Unlock()

	// Second batch of calls
	debouncer.Call(fn)
	debouncer.Call(fn)

	// Wait for second call to execute
	time.Sleep(delay + 10*time.Millisecond)

	mu.Lock()
	finalCount := callCount
	mu.Unlock()

	// Should have been called twice total (once per batch)
	if firstCount != 1 {
		t.Errorf("Expected function to be called once after first batch, but was called %d times", firstCount)
	}
	if finalCount != 2 {
		t.Errorf("Expected function to be called twice total, but was called %d times", finalCount)
	}
}

func TestDebouncerConcurrency(t *testing.T) {
	delay := 100 * time.Millisecond
	debouncer := NewDebouncer(delay)

	callCount := 0
	var mu sync.Mutex

	fn := func() {
		mu.Lock()
		callCount++
		mu.Unlock()
	}

	// Call from multiple goroutines simultaneously
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			debouncer.Call(fn)
		}()
	}

	wg.Wait()

	// Wait for the debounced call to execute
	time.Sleep(delay + 10*time.Millisecond)

	mu.Lock()
	finalCount := callCount
	mu.Unlock()

	// Despite 10 concurrent calls, function should only be called once
	if finalCount != 1 {
		t.Errorf("Expected function to be called once despite concurrent calls, but was called %d times", finalCount)
	}
}

func TestDebouncerReset(t *testing.T) {
	delay := 50 * time.Millisecond
	debouncer := NewDebouncer(delay)

	callCount := 0
	var mu sync.Mutex

	fn := func() {
		mu.Lock()
		callCount++
		mu.Unlock()
	}

	// Make initial call
	debouncer.Call(fn)

	// Wait for half the delay
	time.Sleep(delay / 2)

	// Make another call - this should reset the timer
	debouncer.Call(fn)

	// Wait for half the original delay (timer should still be running)
	time.Sleep(delay / 2)

	mu.Lock()
	midCount := callCount
	mu.Unlock()

	// Function should not have been called yet
	if midCount != 0 {
		t.Errorf("Function should not have been called yet, but was called %d times", midCount)
	}

	// Wait for the remaining time
	time.Sleep(delay/2 + 10*time.Millisecond)

	mu.Lock()
	finalCount := callCount
	mu.Unlock()

	// Now it should have been called once
	if finalCount != 1 {
		t.Errorf("Expected function to be called once, but was called %d times", finalCount)
	}
}

func TestDebouncerZeroDelay(t *testing.T) {
	// Test with zero delay - should still work
	debouncer := NewDebouncer(0)

	callCount := 0
	var mu sync.Mutex

	fn := func() {
		mu.Lock()
		callCount++
		mu.Unlock()
	}

	debouncer.Call(fn)

	// Even with zero delay, need to wait a bit for the timer to fire
	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	finalCount := callCount
	mu.Unlock()

	if finalCount != 1 {
		t.Errorf("Expected function to be called once with zero delay, but was called %d times", finalCount)
	}
}

func TestDebouncerTimerCleanup(t *testing.T) {
	delay := 30 * time.Millisecond
	debouncer := NewDebouncer(delay)

	called := false
	fn := func() {
		called = true
	}

	// Make a call
	debouncer.Call(fn)

	// Verify timer is set
	debouncer.mu.Lock()
	firstTimer := debouncer.timer
	debouncer.mu.Unlock()

	if firstTimer == nil {
		t.Error("Timer should be set after first call")
	}

	// Make another call - should replace the timer
	debouncer.Call(fn)

	// Verify timer is still set (may be different instance)
	debouncer.mu.Lock()
	secondTimer := debouncer.timer
	debouncer.mu.Unlock()

	if secondTimer == nil {
		t.Error("Timer should still be set after second call")
	}

	// Wait for execution
	time.Sleep(delay + 10*time.Millisecond)

	if !called {
		t.Error("Function should have been called")
	}
}

func TestDebouncerWithNilFunction(t *testing.T) {
	delay := 10 * time.Millisecond
	debouncer := NewDebouncer(delay)

	// This should not panic and should not set a timer
	debouncer.Call(nil)

	// Verify timer is not set
	debouncer.mu.Lock()
	timer := debouncer.timer
	debouncer.mu.Unlock()

	if timer != nil {
		t.Error("Timer should not be set for nil function")
	}

	// Wait for the delay - nothing should happen
	time.Sleep(delay + 5*time.Millisecond)

	// Test passes if no panic occurred
}

func TestDebouncerStopsPreviousTimer(t *testing.T) {
	delay := 100 * time.Millisecond
	debouncer := NewDebouncer(delay)

	callCount := 0
	var mu sync.Mutex

	fn1 := func() {
		mu.Lock()
		callCount += 1
		mu.Unlock()
	}

	fn2 := func() {
		mu.Lock()
		callCount += 10
		mu.Unlock()
	}

	// Start first function
	debouncer.Call(fn1)

	// Wait a bit but not the full delay
	time.Sleep(delay / 3)

	// Start second function - should cancel first
	debouncer.Call(fn2)

	// Wait for second function to execute
	time.Sleep(delay + 10*time.Millisecond)

	mu.Lock()
	finalCount := callCount
	mu.Unlock()

	// Should only have executed fn2 (adds 10), fn1 (adds 1) should have been cancelled
	if finalCount != 10 {
		t.Errorf("Expected callCount to be 10 (only fn2), but got %d", finalCount)
	}
}
