package actualtime

import (
	"time"
)

// ActualTime implements time operations using actual time operations
type ActualTime struct {
}

// Ping repeats an action at regular intervals
func (t ActualTime) Ping(count int, period time.Duration, f func() error) error {
	ticker := time.NewTicker(period)
	defer ticker.Stop()

	var lastErr error
	for i := 0; i < count; i++ {
		err := f()
		if err == nil {
			return nil
		}
		if i < count-1 {
			<-ticker.C
		}
		lastErr = err
	}

	return lastErr
}
