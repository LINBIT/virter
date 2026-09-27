package virter

import (
	"context"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
)

func TestLogReadyProgress(t *testing.T) {
	hook := logtest.NewGlobal()
	t.Cleanup(func() { log.StandardLogger().ReplaceHooks(make(log.LevelHooks)) })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		logReadyProgress(ctx, "vm1", time.Now().Add(-30*time.Second), time.Millisecond)
		close(done)
	}()

	assert.Eventually(t, func() bool { return len(hook.AllEntries()) >= 2 }, time.Second, time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("logReadyProgress did not return after cancel")
	}

	entry := hook.AllEntries()[0]
	assert.Equal(t, log.InfoLevel, entry.Level)
	assert.Equal(t, "Still waiting for VM 'vm1' to get ready (30s elapsed)", entry.Message)
}
