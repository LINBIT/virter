package actualtime

import (
	"context"
	"time"
)

// ActualTime implements time operations using actual time operations
type ActualTime struct {
}

// Ping repeats an action at regular intervals
func (t ActualTime) Ping(ctx context.Context, count int, period time.Duration, f func() error) error {
	ticker := time.NewTicker(period)
	defer ticker.Stop()

	var lastErr error
	for i := 0; i < count; i++ {
		err := f()
		if err == nil {
			return nil
		}
		if i < count-1 {
			select {
			case <-ticker.C:
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		lastErr = err
	}

	return lastErr
}

// After waits for the duration to elapse and then sends the current time on
// the returned channel
func (t ActualTime) After(d time.Duration) <-chan time.Time {
	return time.After(d)
}
