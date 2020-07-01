package cmd

import (
	"context"
	"os"
	"os/signal"

	"golang.org/x/sys/unix"
)

func registerSignals(ctx context.Context, cancel context.CancelFunc) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, unix.SIGTERM)
	go func() {
		select {
		case <-c:
			signal.Stop(c)
			cancel()
		case <-ctx.Done():
			signal.Stop(c)
		}
	}()
}
