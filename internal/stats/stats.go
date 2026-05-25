package stats

import (
	"sync"
	"sync/atomic"
	"time"
)

type Stats struct {
	InFlightJobs       int64
	JobsTotal          int64
	JobsFailedInternal int64

	mu                  sync.Mutex
	LastInternalErrorAt time.Time
}

func (s *Stats) IncrInflight() {
	atomic.AddInt64(&s.InFlightJobs, 1)
}

func (s *Stats) DecrInflight() {
	atomic.AddInt64(&s.InFlightJobs, -1)
}

func (s *Stats) IncrTotal() {
	atomic.AddInt64(&s.JobsTotal, 1)
}

func (s *Stats) IncrFailed() {
	atomic.AddInt64(&s.JobsFailedInternal, 1)
}

func (s *Stats) SetLastError(t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastInternalErrorAt = t
}
