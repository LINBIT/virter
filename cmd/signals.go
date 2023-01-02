package cmd

import (
	"context"
	"os/signal"
	"syscall"
)

func onInterruptWrap(ctx context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
}
