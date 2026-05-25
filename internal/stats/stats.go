package stats

import (
	"sync/atomic"
)

// Stats holds atomic job counters
type Stats struct {
	totalJobs   int64
	successJobs int64
	failedJobs  int64
}

// NewStats creates a new Stats instance
func NewStats() *Stats {
	return &Stats{}
}

// IncrementJobs increments the total job counter
func (s *Stats) IncrementJobs() {
	atomic.AddInt64(&s.totalJobs, 1)
}

// IncrementSuccess increments the success counter
func (s *Stats) IncrementSuccess() {
	atomic.AddInt64(&s.successJobs, 1)
}

// IncrementFailed increments the failed counter
func (s *Stats) IncrementFailed() {
	atomic.AddInt64(&s.failedJobs, 1)
}

// TotalJobs returns the total number of jobs
func (s *Stats) TotalJobs() int64 {
	return atomic.LoadInt64(&s.totalJobs)
}

// SuccessJobs returns the number of successful jobs
func (s *Stats) SuccessJobs() int64 {
	return atomic.LoadInt64(&s.successJobs)
}

// FailedJobs returns the number of failed jobs
func (s *Stats) FailedJobs() int64 {
	return atomic.LoadInt64(&s.failedJobs)
}

// Summary returns a string summary of stats
func (s *Stats) Summary() string {
	return ""
}
