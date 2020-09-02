package cmd

import (
	"context"
	"syscall"

	"github.com/sethvargo/go-signalcontext"
)

func onInterruptWrap(ctx context.Context) (context.Context, context.CancelFunc) {
	return signalcontext.Wrap(ctx, syscall.SIGINT, syscall.SIGTERM)
}
