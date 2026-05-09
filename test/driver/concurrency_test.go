package driver_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/j4flmao/go-migrate-safe/driver/memory"
)

func TestDriver_Concurrency_Locking(t *testing.T) {
	drv := memory.New()
	ctx := context.Background()

	// 1. Test basic lock/unlock
	if err := drv.AcquireLock(ctx); err != nil {
		t.Fatalf("first lock failed: %v", err)
	}

	// Second lock should fail or block (memory driver usually fails if already locked)
	err := drv.AcquireLock(ctx)
	if err == nil {
		t.Error("second lock should have failed")
	}

	if err := drv.ReleaseLock(ctx); err != nil {
		t.Fatalf("unlock failed: %v", err)
	}

	// 2. Test concurrent access
	var wg sync.WaitGroup
	lockCount := 0
	var mu sync.Mutex

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := drv.AcquireLock(ctx); err == nil {
				mu.Lock()
				lockCount++
				mu.Unlock()
				time.Sleep(10 * time.Millisecond)
				drv.ReleaseLock(ctx)
			}
		}()
	}
	wg.Wait()

	// Memory driver is simple, but we should ensure only one goroutine at a time got the lock
	// In this simple test, we just want to ensure no panics and some successful locks
	if lockCount == 0 {
		t.Error("expected at least one successful lock")
	}
}
