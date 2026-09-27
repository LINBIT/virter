package cmd

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestReadyConfig(t *testing.T) {
	testcases := []struct {
		name         string
		pingCount    int
		pingPeriod   time.Duration
		readyTimeout time.Duration
		expect       time.Duration
	}{
		{name: "defaults give 5m", pingCount: 300, pingPeriod: time.Second, expect: 5 * time.Minute},
		{name: "tuned count keeps the attempt limit", pingCount: 500, pingPeriod: time.Second, expect: 0},
		{name: "tuned period keeps the attempt limit", pingCount: 300, pingPeriod: 2 * time.Second, expect: 0},
		{name: "ready_timeout wins over tuned count", pingCount: 500, pingPeriod: time.Second, readyTimeout: 45 * time.Second, expect: 45 * time.Second},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			actual := readyConfig(tc.pingCount, tc.pingPeriod, tc.readyTimeout)
			assert.Equal(t, tc.expect, actual.Timeout)
			assert.Equal(t, tc.pingCount, actual.Retries)
			assert.Equal(t, tc.pingPeriod, actual.CheckTimeout)
		})
	}
}
