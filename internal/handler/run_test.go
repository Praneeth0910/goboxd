package handler

import (
	"testing"
	"time"

	"github.com/thesouldev/goboxd/internal/config"
	"github.com/thesouldev/goboxd/internal/stats"
)

func TestNewRunHandlerSemaphore(t *testing.T) {
	cfg := &config.Config{MaxConcurrent: 1}
	st := &stats.Stats{}
	sem := make(chan struct{}, 1)

	h := NewRunHandler(cfg, st, sem)
	if h.sem != sem {
		t.Errorf("expected semaphore to be assigned correctly")
	}
}

func TestSemaphoreBlockingAndQueuing(t *testing.T) {
	// A semaphore with capacity 1
	sem := make(chan struct{}, 1)

	// 1. Acquire the single slot immediately
	sem <- struct{}{}

	// 2. Try to acquire again in a goroutine. Since the channel is full, this must block (queue).
	blocked := true
	done := make(chan bool)
	go func() {
		sem <- struct{}{}
		blocked = false
		done <- true
	}()

	// Verify that the goroutine is indeed blocked and has not completed yet
	select {
	case <-done:
		t.Fatalf("expected second acquire to block, but it succeeded immediately")
	case <-time.After(50 * time.Millisecond):
		// This is the expected path: the goroutine is blocked waiting on the channel
	}

	// 3. Release the first slot
	<-sem

	// Verify that the goroutine now unblocks, acquires the slot, and completes successfully
	select {
	case <-done:
		if blocked {
			t.Errorf("expected blocked to be false after acquisition completed")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("expected second acquire to complete after release, but it timed out")
	}

	// Clean up by releasing the second acquired slot
	<-sem
}
