package scheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type mockScanner struct {
	runs atomic.Int32
	err  error
}

func (m *mockScanner) Run(ctx context.Context) error {
	m.runs.Add(1)
	return m.err
}

func TestRunOnce(t *testing.T) {
	scanner := &mockScanner{}
	s := New(scanner, time.Hour)

	if err := s.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	if scanner.runs.Load() != 1 {
		t.Errorf("expected 1 run, got %d", scanner.runs.Load())
	}
}

func TestStartRunsImmediatelyThenOnInterval(t *testing.T) {
	scanner := &mockScanner{}
	s := New(scanner, 10*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		s.Start(ctx)
		close(done)
	}()

	<-done

	runs := scanner.runs.Load()
	if runs < 2 {
		t.Errorf("expected at least 2 runs (initial + 1 tick), got %d", runs)
	}
}

func TestStartRespectsContextCancellation(t *testing.T) {
	scanner := &mockScanner{}
	s := New(scanner, time.Hour)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		s.Start(ctx)
		close(done)
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Start did not return after context cancellation")
	}

	if scanner.runs.Load() != 1 {
		t.Errorf("expected 1 run (initial only), got %d", scanner.runs.Load())
	}
}

func TestConcurrentRunsPrevented(t *testing.T) {
	scanner := &mockScanner{}
	s := New(scanner, time.Hour)

	s.mu.Lock()
	s.running = true
	s.mu.Unlock()

	s.runOnce(context.Background())

	if scanner.runs.Load() != 0 {
		t.Error("should not have run while already running")
	}
}

func TestIsRunning(t *testing.T) {
	scanner := &mockScanner{}
	s := New(scanner, time.Hour)

	if s.IsRunning() {
		t.Error("should not be running initially")
	}
}
