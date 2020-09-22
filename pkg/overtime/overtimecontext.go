package overtime

import (
	"context"
	"time"
)

// Creates a new context that is valid for the duration of the parent context + overtime.
//
// This may be of used for deferred cleanup steps that require a context.
//
// Example usage:
//  ctx, cancel := context.WithTimeout(context.Background(), 10 * time.Minute)
//
//  cleanupCtx, cleanupCancel := overtime.WithOvertimeContext(ctx, 1 * time.Minute)
//  defer cleanupCancel()
//
//  stuff := CreateSomethingTemporary(ctx)
//  defer DestroyTemporary(cleanupCtx, stuff)
func WithOvertimeContext(parent context.Context, overtime time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		select {
		case <- ctx.Done():
			// we are done here, context is cancelled
			return
		case <- parent.Done():
			// "parent" context is cancelled, we can start our own timeout now
		}

		timer, timerCancel := context.WithTimeout(ctx, overtime)
		defer timerCancel()
		// No need for extra select here. The timer context will be cancelled in case the parent context is cancelled.
		<- timer.Done()
		cancel()
	}()

	return ctx, cancel
}
