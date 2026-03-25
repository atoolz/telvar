package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type Scanner interface {
	Run(ctx context.Context) error
}

type Scheduler struct {
	scanner  Scanner
	interval time.Duration
	mu       sync.Mutex
	running  bool
}

func New(scanner Scanner, interval time.Duration) *Scheduler {
	return &Scheduler{
		scanner:  scanner,
		interval: interval,
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	s.runOnce(ctx)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("scheduler stopped")
			return
		case <-ticker.C:
			s.runOnce(ctx)
		}
	}
}

func (s *Scheduler) RunOnce(ctx context.Context) error {
	return s.scanner.Run(ctx)
}

func (s *Scheduler) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

func (s *Scheduler) runOnce(ctx context.Context) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		slog.Warn("discovery already running, skipping")
		return
	}
	s.running = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	start := time.Now()
	slog.Info("scheduled discovery starting")

	if err := s.scanner.Run(ctx); err != nil {
		slog.Error("scheduled discovery failed", "error", err, "duration", time.Since(start).Round(time.Millisecond))
		return
	}

	slog.Info("scheduled discovery completed", "duration", time.Since(start).Round(time.Millisecond))
}
